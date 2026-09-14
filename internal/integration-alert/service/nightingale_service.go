// Package service 提供夜莺告警集成服务
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/kerbos/ticketdesk/internal/integration-alert/dto"
	"github.com/kerbos/ticketdesk/internal/integration-alert/repository"
	"github.com/kerbos/ticketdesk/pkg/config"
	"github.com/kerbos/ticketdesk/pkg/logger"
	"github.com/kerbos/ticketdesk/pkg/safego"
)

// ============ N9eClient — 夜莺 API 客户端 ============

// N9eClient 夜莺 API 客户端
type N9eClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewN9eClient 创建夜莺 API 客户端
func NewN9eClient(cfg *config.NightingaleConfig) *N9eClient {
	return &N9eClient{
		baseURL: cfg.BaseURL,
		token:   cfg.Token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewN9eClientFromDatasource 从数据源配置创建夜莺 API 客户端
func NewN9eClientFromDatasource(baseURL, token string) *N9eClient {
	return &N9eClient{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// FetchActiveAlerts 从夜莺拉取活跃告警列表
func (c *N9eClient) FetchActiveAlerts(ctx context.Context, page, limit int) ([]dto.N9eAlertEvent, int64, error) {
	url := fmt.Sprintf("%s/api/n9e/alert-cur-events/list?limit=%d&p=%d", c.baseURL, limit, page)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置认证头
	if c.token != "" {
		req.Header.Set("X-User-Token", c.token)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to fetch active alerts: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, 0, fmt.Errorf("nightingale API returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read response body: %w", err)
	}

	var result dto.N9eAlertListResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, 0, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if result.Err != "" {
		return nil, 0, fmt.Errorf("nightingale API error: %s", result.Err)
	}

	return result.Dat.List, result.Dat.Total, nil
}

// FetchAllActiveAlerts 拉取全部活跃告警（自动分页）
func (c *N9eClient) FetchAllActiveAlerts(ctx context.Context) ([]dto.N9eAlertEvent, error) {
	var allEvents []dto.N9eAlertEvent
	page := 1
	limit := 100

	for {
		events, total, err := c.FetchActiveAlerts(ctx, page, limit)
		if err != nil {
			return nil, err
		}

		allEvents = append(allEvents, events...)

		// 检查是否还有更多数据
		if int64(len(allEvents)) >= total || len(events) < limit {
			break
		}

		page++
	}

	return allEvents, nil
}

// ============ N9ePoller — 夜莺告警轮询器 ============

// N9ePoller 夜莺告警定时轮询器
type N9ePoller struct {
	client       *N9eClient
	alertService AlertService
	alertRepo    repository.AlertRepository
	interval     time.Duration
	sourceName   string
	datasourceID uint64
	stopCh       chan struct{}
	doneCh       chan struct{}
}

// NewN9ePoller 创建夜莺告警轮询器
func NewN9ePoller(
	client *N9eClient,
	alertService AlertService,
	alertRepo repository.AlertRepository,
	interval time.Duration,
) *N9ePoller {
	return &N9ePoller{
		client:       client,
		alertService: alertService,
		alertRepo:    alertRepo,
		interval:     interval,
		sourceName:   "nightingale",
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}
}

// NewN9ePollerWithSource 创建带自定义来源名称的夜莺告警轮询器
func NewN9ePollerWithSource(
	client *N9eClient,
	alertService AlertService,
	alertRepo repository.AlertRepository,
	interval time.Duration,
	sourceName string,
	datasourceID uint64,
) *N9ePoller {
	return &N9ePoller{
		client:       client,
		alertService: alertService,
		alertRepo:    alertRepo,
		interval:     interval,
		sourceName:   sourceName,
		datasourceID: datasourceID,
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}
}

// Start 启动轮询器
func (p *N9ePoller) Start() {
	logger.Info("nightingale poller started",
		zap.String("source", p.sourceName),
		zap.Duration("interval", p.interval),
	)

	safego.Go("nightingale.poller", p.run)
}

// Stop 停止轮询器
func (p *N9ePoller) Stop() {
	logger.Info("stopping nightingale poller...", zap.String("source", p.sourceName))
	close(p.stopCh)
	<-p.doneCh
	logger.Info("nightingale poller stopped", zap.String("source", p.sourceName))
}

// run 轮询主循环
func (p *N9ePoller) run() {
	defer close(p.doneCh)

	// 启动后立即执行一次
	safego.Run("nightingale.poll", p.poll)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			// 单次轮询 panic 不应终止轮询循环
			safego.Run("nightingale.poll", p.poll)
		}
	}
}

// poll 执行一次轮询
func (p *N9ePoller) poll() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	logger.Debug("nightingale poller: fetching active alerts")

	// 1. 从夜莺拉取全部活跃告警
	events, err := p.client.FetchAllActiveAlerts(ctx)
	if err != nil {
		logger.Error("nightingale poller: failed to fetch active alerts", zap.Error(err))
		return
	}

	logger.Info("nightingale poller: fetched active alerts",
		zap.Int("count", len(events)),
	)

	// 2. 收集活跃告警的 hash 集合
	activeHashes := make(map[string]struct{}, len(events))
	for _, event := range events {
		hash := event.Hash
		if hash == "" {
			hash = fmt.Sprintf("n9e-%d-%d", event.RuleID, event.ID)
		}
		activeHashes[hash] = struct{}{}
	}

	// 3. 处理每条活跃告警
	for _, event := range events {
		if err := p.alertService.HandleNightingaleWebhookWithSource(ctx, []dto.N9eAlertEvent{event}, p.sourceName, p.datasourceID); err != nil {
			logger.Error("nightingale poller: failed to process alert",
				zap.String("source", p.sourceName),
				zap.String("hash", event.Hash),
				zap.String("rule_name", event.RuleName),
				zap.Error(err),
			)
		}
	}

	// 4. 自动恢复检测：本地 firing 但不在活跃列表中的告警标记为 resolved
	p.detectRecoveredAlerts(ctx, activeHashes)
}

// detectRecoveredAlerts 检测已恢复的告警
func (p *N9ePoller) detectRecoveredAlerts(ctx context.Context, activeHashes map[string]struct{}) {
	// 查询本地 source 且 status=firing 的告警
	localFiringAlerts, err := p.alertRepo.ListBySourceAndStatus(ctx, p.sourceName, "firing")
	if err != nil {
		logger.Error("nightingale poller: failed to list local firing alerts", zap.Error(err))
		return
	}

	recoveredCount := 0
	for _, alert := range localFiringAlerts {
		if _, active := activeHashes[alert.Fingerprint]; !active {
			// 本地 firing 但夜莺已不再活跃 → 标记为 resolved
			now := time.Now()
			alert.Status = "resolved"
			alert.EndsAt = &now

			if err := p.alertRepo.Update(ctx, alert); err != nil {
				logger.Error("nightingale poller: failed to resolve alert",
					zap.Uint64("alert_id", alert.ID),
					zap.String("fingerprint", alert.Fingerprint),
					zap.Error(err),
				)
				continue
			}

			recoveredCount++
			logger.Info("nightingale poller: alert auto-recovered",
				zap.Uint64("alert_id", alert.ID),
				zap.String("alert_name", alert.AlertName),
				zap.String("fingerprint", alert.Fingerprint),
			)

			// 如果告警关联了工单，尝试自动关闭工单（所有告警都恢复时）
			if alert.IssueID != nil {
				if err := p.alertService.TryAutoResolveIssue(ctx, *alert.IssueID); err != nil {
					logger.Error("nightingale poller: failed to try auto resolve issue",
						zap.Uint64("issue_id", *alert.IssueID),
						zap.Error(err),
					)
				}
			}
		}
	}

	if recoveredCount > 0 {
		logger.Info("nightingale poller: auto-recovered alerts",
			zap.Int("count", recoveredCount),
		)
	}
}
