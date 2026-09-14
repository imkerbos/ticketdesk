// Package main 应用入口
//
// @title           TicketDesk API
// @version         1.0
// @description     项目化工单与告警联动系统 API. 一切问题都是工单, 一切告警都必须被跟进.
// @description     **统一响应格式**: `{code, message, data}`. 错误响应 `{code, message, details?}` (code 为字符串如 "BAD_REQUEST" / "UNAUTHORIZED").
// @description     **速率限制**: 全站默认 300 req/min/IP, /auth/* 端点 20 req/min/IP, /webhook/* 100 req/min/IP. 超出返回 **429 Too Many Requests**, 此状态码在所有端点皆可能出现, 单独 endpoint 文档不重复声明.
// @termsOfService  https://github.com/imkerbos/TicketDesk
//
// @contact.name   TicketDesk
// @contact.url    https://github.com/imkerbos/TicketDesk
//
// @license.name  MIT
//
// 不声明 @host：声明了就会被写进 swagger.json，文档页上的 Base URL 永远是
// 生成时那台机器的地址（原来是 localhost:10010），换个域名部署就对不上，
// 照着调的人第一个请求就失败。留空时 swagger UI 用当前页面的 origin。
// @BasePath  /api/v1
//
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT Bearer 令牌 (`Bearer <jwt>`) 或 Personal Access Token (`Bearer td_pat_xxx`)
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/kerbos/ticketdesk/internal/api/router"
	"github.com/kerbos/ticketdesk/internal/model"
	"github.com/kerbos/ticketdesk/pkg/cache"
	"github.com/kerbos/ticketdesk/pkg/config"
	"github.com/kerbos/ticketdesk/pkg/database"
	"github.com/kerbos/ticketdesk/pkg/jwt"
	"github.com/kerbos/ticketdesk/pkg/logger"
	"github.com/kerbos/ticketdesk/pkg/redis"
)

var configPath string

func init() {
	flag.StringVar(&configPath, "config", "configs/config-dev.yaml", "配置文件路径")
}

func main() {
	flag.Parse()

	// 加载配置
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	if err := logger.Init(&cfg.Log); err != nil {
		fmt.Printf("Failed to init logger: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if err := logger.Sync(); err != nil {
			fmt.Printf("Failed to sync logger: %v\n", err)
		}
	}()

	logger.Info("starting ticketdesk",
		zap.String("env", cfg.App.Env),
		zap.Int("port", cfg.App.Port),
	)

	// 初始化数据库
	if err := database.Init(&cfg.Database, cfg.App.Debug); err != nil {
		logger.Fatal("failed to init database", zap.Error(err))
	}
	defer func() {
		if err := database.Close(); err != nil {
			logger.Error("failed to close database", zap.Error(err))
		}
	}()

	// 初始化 Redis（迁移需要用它加分布式锁，因此要先于迁移初始化）
	if err := redis.Init(&cfg.Redis); err != nil {
		logger.Fatal("failed to init redis", zap.Error(err))
	}
	defer func() {
		if err := redis.Close(); err != nil {
			logger.Error("failed to close redis", zap.Error(err))
		}
	}()

	// 自动迁移 + 种子数据。
	//
	// 多副本部署时所有副本会同时启动并各跑一遍 AutoMigrate，
	// 意味着并发对同一批表执行 DDL（含 CREATE/DROP INDEX），
	// 大表上还会互相阻塞。这里用分布式锁串行化：
	// 抢到锁的副本执行，其余副本等待锁释放后再继续启动。
	if err := runMigrations(cfg); err != nil {
		logger.Fatal("failed to migrate database", zap.Error(err))
	}

	// 初始化 JWT 管理器
	jwtManager := jwt.NewManager(&cfg.JWT)

	// 设置路由
	appRouter := router.NewRouter(cfg, jwtManager, database.GetDB())
	r := appRouter.Setup()

	// 创建 HTTP 服务器
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.App.Port),
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// 启动服务器
	go func() {
		logger.Info("server started", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("failed to start server", zap.Error(err))
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")

	// 停止所有数据源轮询器
	appRouter.StopPollers()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server forced to shutdown", zap.Error(err))
	}

	logger.Info("server exited")
}

// migrationLockKey 迁移互斥锁的键
const migrationLockKey = "ticketdesk:migration:lock"

// runMigrations 在分布式锁保护下执行数据库迁移与种子数据
//
// 锁获取失败（Redis 不可用）时降级为直接执行：单副本部署下行为不变，
// 多副本下退回到原来的并发风险，但不会因为 Redis 抖动而拒绝启动。
func runMigrations(cfg *config.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	migrate := func() error {
		if err := model.AutoMigrate(database.GetDB()); err != nil {
			return fmt.Errorf("auto migrate: %w", err)
		}
		if err := model.SeedData(database.GetDB()); err != nil {
			return fmt.Errorf("seed data: %w", err)
		}
		return nil
	}

	// 最多等 5 分钟让先启动的副本完成迁移
	lockValue := cache.TryLockWithRetry(ctx, migrationLockKey, 10*time.Minute, 60, 5*time.Second)
	switch lockValue {
	case "":
		logger.Warn("migration lock not acquired within timeout, proceeding without lock")
		return migrate()
	case "degraded":
		logger.Warn("migration lock degraded (redis unavailable), proceeding without lock")
		return migrate()
	default:
		defer cache.UnlockWithValue(ctx, migrationLockKey, lockValue)
		logger.Info("migration lock acquired", zap.String("env", cfg.App.Env))
		return migrate()
	}
}
