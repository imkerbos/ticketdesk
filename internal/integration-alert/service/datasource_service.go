// Package service 提供告警数据源业务逻辑层
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/kerbos/ticketdesk/internal/integration-alert/dto"
	"github.com/kerbos/ticketdesk/internal/integration-alert/repository"
	"github.com/kerbos/ticketdesk/internal/model"
	"github.com/kerbos/ticketdesk/pkg/config"
	"github.com/kerbos/ticketdesk/pkg/logger"
	"github.com/kerbos/ticketdesk/pkg/safehttp"
)

// DatasourceService 数据源服务接口
type DatasourceService interface {
	Create(ctx context.Context, req *dto.CreateDatasourceRequest, userID uint64) (*dto.DatasourceResponse, error)
	GetByID(ctx context.Context, id uint64) (*dto.DatasourceResponse, error)
	GetByName(ctx context.Context, name string) (*dto.DatasourceResponse, error)
	Update(ctx context.Context, id uint64, req *dto.UpdateDatasourceRequest) (*dto.DatasourceResponse, error)
	Delete(ctx context.Context, id uint64) error
	List(ctx context.Context, req *dto.DatasourceListRequest) (*dto.DatasourceListResponse, error)
	TestConnection(ctx context.Context, req *dto.TestDatasourceRequest) (*dto.TestDatasourceResponse, error)
	TestConnectionByID(ctx context.Context, id uint64) (*dto.TestDatasourceResponse, error)
	StartAllPollers(ctx context.Context) error
	StopAllPollers()
}

// datasourceService 数据源服务实现
type datasourceService struct {
	repo          repository.AlertDatasourceRepository
	alertRuleRepo repository.AlertRuleRepository
	alertService  AlertService
	alertRepo     repository.AlertRepository
	legacyCfg     *config.NightingaleConfig
	pollers       map[uint64]*N9ePoller
	mu            sync.Mutex
}

// NewDatasourceService 创建数据源服务
func NewDatasourceService(
	repo repository.AlertDatasourceRepository,
	alertRuleRepo repository.AlertRuleRepository,
	alertService AlertService,
	alertRepo repository.AlertRepository,
	legacyCfg *config.NightingaleConfig,
) DatasourceService {
	return &datasourceService{
		repo:          repo,
		alertRuleRepo: alertRuleRepo,
		alertService:  alertService,
		alertRepo:     alertRepo,
		legacyCfg:     legacyCfg,
		pollers:       make(map[uint64]*N9ePoller),
	}
}

// Create 创建数据源
func (s *datasourceService) Create(ctx context.Context, req *dto.CreateDatasourceRequest, userID uint64) (*dto.DatasourceResponse, error) {
	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	pollInterval := req.PollInterval
	if pollInterval == 0 {
		pollInterval = 30
	}

	ds := &model.AlertDatasource{
		Name:         req.Name,
		Type:         req.Type,
		Description:  req.Description,
		Config:       string(configJSON),
		PushMode:     req.PushMode,
		PollInterval: pollInterval,
		Status:       1,
		CreatedBy:    userID,
	}

	if err := s.repo.Create(ctx, ds); err != nil {
		return nil, fmt.Errorf("failed to create datasource: %w", err)
	}

	// 如果启用且为轮询模式，启动 Poller
	if ds.Status == 1 && !ds.PushMode && ds.Type == model.DatasourceTypeNightingale {
		s.startPoller(ds)
	}

	return s.toDatasourceResponse(ds), nil
}

// GetByID 获取数据源详情
func (s *datasourceService) GetByID(ctx context.Context, id uint64) (*dto.DatasourceResponse, error) {
	ds, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.toDatasourceResponse(ds), nil
}

// GetByName 根据名称获取数据源
func (s *datasourceService) GetByName(ctx context.Context, name string) (*dto.DatasourceResponse, error) {
	ds, err := s.repo.GetByName(ctx, name)
	if err != nil {
		return nil, err
	}
	return s.toDatasourceResponse(ds), nil
}

// Update 更新数据源
func (s *datasourceService) Update(ctx context.Context, id uint64, req *dto.UpdateDatasourceRequest) (*dto.DatasourceResponse, error) {
	ds, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	configChanged := false

	if req.Name != nil {
		ds.Name = *req.Name
	}
	if req.Description != nil {
		ds.Description = *req.Description
	}
	if req.Config != nil {
		configJSON, err := json.Marshal(req.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal config: %w", err)
		}
		ds.Config = string(configJSON)
		configChanged = true
	}
	if req.PushMode != nil {
		ds.PushMode = *req.PushMode
	}
	if req.PollInterval != nil {
		ds.PollInterval = *req.PollInterval
		configChanged = true
	}
	if req.Status != nil {
		ds.Status = *req.Status
	}

	if err := s.repo.Update(ctx, ds); err != nil {
		return nil, fmt.Errorf("failed to update datasource: %w", err)
	}

	// Poller 生命周期管理
	s.managePollerLifecycle(ds, configChanged)

	return s.toDatasourceResponse(ds), nil
}

// Delete 删除数据源
func (s *datasourceService) Delete(ctx context.Context, id uint64) error {
	// 检查是否有关联的告警规则
	count, err := s.alertRuleRepo.CountByDatasourceID(ctx, id)
	if err != nil {
		return fmt.Errorf("检查关联规则失败: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("该数据源下有 %d 条告警规则，请先删除关联规则", count)
	}

	// 停止 Poller
	s.stopPoller(id)

	return s.repo.Delete(ctx, id)
}

// List 获取数据源列表
func (s *datasourceService) List(ctx context.Context, req *dto.DatasourceListRequest) (*dto.DatasourceListResponse, error) {
	datasources, total, err := s.repo.List(ctx, req)
	if err != nil {
		return nil, err
	}

	items := make([]dto.DatasourceResponse, 0, len(datasources))
	for _, ds := range datasources {
		items = append(items, *s.toDatasourceResponse(ds))
	}

	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	return &dto.DatasourceListResponse{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// TestConnection 测试数据源连接
func (s *datasourceService) TestConnection(ctx context.Context, req *dto.TestDatasourceRequest) (*dto.TestDatasourceResponse, error) {
	start := time.Now()

	switch req.Type {
	case model.DatasourceTypeNightingale:
		baseURL, _ := req.Config["base_url"].(string)
		token, _ := req.Config["token"].(string)
		if baseURL == "" {
			return &dto.TestDatasourceResponse{
				Success: false,
				Message: "base_url 不能为空",
			}, nil
		}
		client := NewN9eClientFromDatasource(baseURL, token)
		_, _, err := client.FetchActiveAlerts(ctx, 1, 1)
		latency := time.Since(start).Milliseconds()
		if err != nil {
			return &dto.TestDatasourceResponse{
				Success:   false,
				Message:   fmt.Sprintf("连接失败: %v", err),
				LatencyMs: latency,
			}, nil
		}
		return &dto.TestDatasourceResponse{
			Success:   true,
			Message:   "连接成功",
			LatencyMs: latency,
		}, nil

	case model.DatasourceTypePrometheus:
		baseURL, _ := req.Config["base_url"].(string)
		if baseURL == "" {
			return &dto.TestDatasourceResponse{
				Success: true,
				Message: "推送模式已就绪（无需配置 base_url）",
			}, nil
		}
		healthURL := baseURL + "/-/healthy"
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, http.NoBody)
		if err != nil {
			return &dto.TestDatasourceResponse{
				Success: false,
				Message: fmt.Sprintf("请求创建失败: %v", err),
			}, nil
		}
		httpClient := safehttp.NewClient(10 * time.Second)
		resp, err := httpClient.Do(httpReq)
		latency := time.Since(start).Milliseconds()
		if err != nil {
			return &dto.TestDatasourceResponse{
				Success:   false,
				Message:   fmt.Sprintf("连接失败: %v", err),
				LatencyMs: latency,
			}, nil
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return &dto.TestDatasourceResponse{
				Success:   false,
				Message:   fmt.Sprintf("健康检查返回状态码: %d", resp.StatusCode),
				LatencyMs: latency,
			}, nil
		}
		return &dto.TestDatasourceResponse{
			Success:   true,
			Message:   "连接成功",
			LatencyMs: latency,
		}, nil

	default:
		return &dto.TestDatasourceResponse{
			Success: false,
			Message: fmt.Sprintf("不支持的数据源类型: %s", req.Type),
		}, nil
	}
}

// TestConnectionByID 测试已保存数据源的连接
func (s *datasourceService) TestConnectionByID(ctx context.Context, id uint64) (*dto.TestDatasourceResponse, error) {
	ds, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	var configMap map[string]interface{}
	if err := json.Unmarshal([]byte(ds.Config), &configMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	result, err := s.TestConnection(ctx, &dto.TestDatasourceRequest{
		Type:   ds.Type,
		Config: configMap,
	})
	if err != nil {
		return nil, err
	}

	// 更新检测结果
	if updateErr := s.repo.UpdateCheckResult(ctx, id, result.Success, result.Message); updateErr != nil {
		logger.Error("failed to update check result", zap.Uint64("id", id), zap.Error(updateErr))
	}

	return result, nil
}

// StartAllPollers 启动所有启用的轮询数据源的 Poller
func (s *datasourceService) StartAllPollers(ctx context.Context) error {
	// 旧配置迁移：如果旧配置启用且数据库中无 nightingale 数据源，自动创建
	if s.legacyCfg != nil && s.legacyCfg.Enable {
		s.migrateLegacyConfig(ctx)
	}

	datasources, err := s.repo.ListEnabled(ctx)
	if err != nil {
		return fmt.Errorf("failed to list enabled datasources: %w", err)
	}

	for _, ds := range datasources {
		if !ds.PushMode && ds.Type == model.DatasourceTypeNightingale {
			s.startPoller(ds)
		}
	}

	logger.Info("all datasource pollers started", zap.Int("count", len(s.pollers)))
	return nil
}

// StopAllPollers 停止所有 Poller
func (s *datasourceService) StopAllPollers() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, poller := range s.pollers {
		poller.Stop()
		delete(s.pollers, id)
	}

	logger.Info("all datasource pollers stopped")
}

// startPoller 启动单个数据源的 Poller
func (s *datasourceService) startPoller(ds *model.AlertDatasource) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 已在运行则不重复启动
	if _, exists := s.pollers[ds.ID]; exists {
		return
	}

	var configMap map[string]interface{}
	if err := json.Unmarshal([]byte(ds.Config), &configMap); err != nil {
		logger.Error("failed to unmarshal datasource config for poller",
			zap.Uint64("id", ds.ID),
			zap.Error(err),
		)
		return
	}

	baseURL, _ := configMap["base_url"].(string)
	token, _ := configMap["token"].(string)

	if baseURL == "" {
		logger.Warn("datasource has no base_url, skipping poller",
			zap.Uint64("id", ds.ID),
			zap.String("name", ds.Name),
		)
		return
	}

	interval := time.Duration(ds.PollInterval) * time.Second
	if interval < 5*time.Second {
		interval = 30 * time.Second
	}

	client := NewN9eClientFromDatasource(baseURL, token)
	poller := NewN9ePollerWithSource(client, s.alertService, s.alertRepo, interval, ds.Name, ds.ID)
	poller.Start()

	s.pollers[ds.ID] = poller
	logger.Info("datasource poller started",
		zap.Uint64("id", ds.ID),
		zap.String("name", ds.Name),
	)
}

// stopPoller 停止单个数据源的 Poller
func (s *datasourceService) stopPoller(id uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if poller, exists := s.pollers[id]; exists {
		poller.Stop()
		delete(s.pollers, id)
	}
}

// managePollerLifecycle 管理 Poller 生命周期
func (s *datasourceService) managePollerLifecycle(ds *model.AlertDatasource, configChanged bool) {
	if ds.Type != model.DatasourceTypeNightingale {
		return
	}

	needsPoller := ds.Status == 1 && !ds.PushMode

	s.mu.Lock()
	_, running := s.pollers[ds.ID]
	s.mu.Unlock()

	// 禁用或切换为推送模式 → 停止
	if !needsPoller && running {
		s.stopPoller(ds.ID)
		return
	}

	// 启用且为轮询模式
	if needsPoller {
		if running && configChanged {
			// 配置变更 → 重启
			s.stopPoller(ds.ID)
			s.startPoller(ds)
		} else if !running {
			// 新启动
			s.startPoller(ds)
		}
	}
}

// migrateLegacyConfig 将旧配置迁移为数据源记录
func (s *datasourceService) migrateLegacyConfig(ctx context.Context) {
	// 检查是否已有 nightingale 类型的数据源
	datasources, err := s.repo.ListEnabled(ctx)
	if err != nil {
		logger.Error("failed to check existing datasources for migration", zap.Error(err))
		return
	}

	for _, ds := range datasources {
		if ds.Type == model.DatasourceTypeNightingale {
			// 已有夜莺数据源，不需要迁移
			return
		}
	}

	configMap := map[string]interface{}{
		"base_url": s.legacyCfg.BaseURL,
		"token":    s.legacyCfg.Token,
	}
	configJSON, _ := json.Marshal(configMap)

	pollInterval := s.legacyCfg.PollInterval
	if pollInterval <= 0 {
		pollInterval = 30
	}

	ds := &model.AlertDatasource{
		Name:         "nightingale",
		Type:         model.DatasourceTypeNightingale,
		Description:  "从旧配置自动迁移的夜莺数据源",
		Config:       string(configJSON),
		PushMode:     false,
		PollInterval: pollInterval,
		Status:       1,
		CreatedBy:    1, // 系统用户
	}

	if err := s.repo.Create(ctx, ds); err != nil {
		logger.Error("failed to migrate legacy nightingale config", zap.Error(err))
		return
	}

	logger.Info("legacy nightingale config migrated to datasource",
		zap.String("name", ds.Name),
		zap.String("base_url", s.legacyCfg.BaseURL),
	)
}

// toDatasourceResponse 转换为数据源响应
func (s *datasourceService) toDatasourceResponse(ds *model.AlertDatasource) *dto.DatasourceResponse {
	var configMap map[string]interface{}
	_ = json.Unmarshal([]byte(ds.Config), &configMap)

	// 数据源名允许中文和空格，直接拼进路径会得到一个复制到 Prometheus / 夜莺
	// 配置里就用不了的 URL（形如 .../datasource/夜莺 N9E/webhook）。
	// 这里做路径转义，路由侧拿到的仍是解码后的原名，行为不变。
	webhookURL := fmt.Sprintf("/api/v1/alerts/datasource/%s/webhook", url.PathEscape(ds.Name))

	return &dto.DatasourceResponse{
		ID:           ds.ID,
		Name:         ds.Name,
		Type:         ds.Type,
		Description:  ds.Description,
		Config:       configMap,
		PushMode:     ds.PushMode,
		PollInterval: ds.PollInterval,
		Status:       ds.Status,
		WebhookURL:   webhookURL,
		LastCheckAt:  ds.LastCheckAt,
		LastCheckOk:  ds.LastCheckOk,
		LastCheckMsg: ds.LastCheckMsg,
		CreatedBy:    ds.CreatedBy,
		CreatedAt:    ds.CreatedAt,
		UpdatedAt:    ds.UpdatedAt,
	}
}
