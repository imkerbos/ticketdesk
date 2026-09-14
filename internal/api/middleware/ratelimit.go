// Package middleware 提供 HTTP 中间件
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/kerbos/ticketdesk/internal/api/response"
	"github.com/kerbos/ticketdesk/pkg/logger"
	pkgRedis "github.com/kerbos/ticketdesk/pkg/redis"
)

// ConfigValueGetter 配置值获取接口（避免依赖整个 ConfigService）
type ConfigValueGetter interface {
	GetConfigValue(ctx context.Context, key string) (string, error)
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	KeyPrefix string            // Redis key 前缀，如 "rl:webhook"
	Limit     int               // 窗口内最大请求数（硬编码默认值，降级用）
	Window    time.Duration     // 时间窗口
	ConfigKey string            // system_config key，如 "ratelimit.webhook_limit"
	ConfigSvc ConfigValueGetter // 动态配置获取服务
}

// rateLimitScript Lua 脚本：INCR + 首次设置 EXPIRE 原子化，避免 key 永不过期
var rateLimitScript = `
local key = KEYS[1]
local ttl = tonumber(ARGV[1])
local count = redis.call('INCR', key)
if count == 1 then
    redis.call('EXPIRE', key, ttl)
end
return count
`

// RateLimitMiddleware 基于 Redis Lua 脚本的固定窗口限流中间件
// Redis 不可用时 fail-open（放行）
func RateLimitMiddleware(cfg RateLimitConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		client := pkgRedis.GetClient()
		if client == nil {
			c.Next()
			return
		}

		ip := c.ClientIP()
		windowSec := int(cfg.Window.Seconds())
		if windowSec < 1 {
			windowSec = 60
		}

		// 动态获取限流值：优先从 ConfigService 读取，失败则用硬编码默认值
		limit := cfg.Limit
		if cfg.ConfigSvc != nil && cfg.ConfigKey != "" {
			if val, err := cfg.ConfigSvc.GetConfigValue(c.Request.Context(), cfg.ConfigKey); err == nil {
				if v, err := strconv.Atoi(val); err == nil && v > 0 {
					limit = v
				}
			}
		}

		// 按时间窗口生成 key，确保窗口到期后自动切换
		windowID := time.Now().Unix() / int64(windowSec)
		key := fmt.Sprintf("%s:%s:%d", cfg.KeyPrefix, ip, windowID)

		ctx := c.Request.Context()

		// Lua 脚本原子执行 INCR + EXPIRE
		count, err := client.Eval(ctx, rateLimitScript, []string{key}, windowSec+1).Int64()
		if err != nil {
			// Redis 出错，放行
			logger.Warn("ratelimit Lua eval failed, allowing request",
				zap.String("key", key),
				zap.Error(err),
			)
			c.Next()
			return
		}

		// 设置限流响应头
		remaining := limit - int(count)
		if remaining < 0 {
			remaining = 0
		}
		c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))

		if int(count) > limit {
			retryAfter := windowSec - int(time.Now().Unix()%int64(windowSec))
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			response.ErrorT(c, http.StatusTooManyRequests, "TOO_MANY_REQUESTS", "common.too_many_requests")
			c.Abort()
			return
		}

		c.Next()
	}
}
