// Package webhook 提供 Webhook 发送服务
package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/kerbos/ticketdesk/internal/model"
	"github.com/kerbos/ticketdesk/internal/system-config/repository"
	"github.com/kerbos/ticketdesk/pkg/logger"
	"github.com/kerbos/ticketdesk/pkg/safego"
	"github.com/kerbos/ticketdesk/pkg/safehttp"
)

// webhookSendTimeout 单次 Webhook 投递的超时（含连接与读写）
const webhookSendTimeout = 30 * time.Second

// WebhookPayload Webhook 请求负载
type WebhookPayload struct {
	Event     string      `json:"event"`
	Timestamp int64       `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// WebhookService Webhook 服务接口
type WebhookService interface {
	// SendEvent 发送事件到所有订阅的 Webhook
	SendEvent(ctx context.Context, event string, data interface{}) error
	// SendToWebhook 发送到指定 Webhook
	SendToWebhook(ctx context.Context, webhook *model.Webhook, event string, data interface{}) error
}

// webhookService Webhook 服务实现
type webhookService struct {
	webhookRepo    repository.WebhookRepository
	webhookLogRepo repository.WebhookLogRepository
	httpClient     *http.Client
}

// NewWebhookService 创建 Webhook 服务实例
func NewWebhookService(
	webhookRepo repository.WebhookRepository,
	webhookLogRepo repository.WebhookLogRepository,
) WebhookService {
	return &webhookService{
		webhookRepo:    webhookRepo,
		webhookLogRepo: webhookLogRepo,
		// 目标 URL 由管理员配置、由服务端发起请求，需带 SSRF 防护
		httpClient: safehttp.NewClient(30 * time.Second),
	}
}

// SendEvent 发送事件到所有订阅的 Webhook
func (s *webhookService) SendEvent(ctx context.Context, event string, data interface{}) error {
	// 获取订阅此事件的所有启用的 Webhook
	webhooks, err := s.webhookRepo.GetByEvent(ctx, event)
	if err != nil {
		return fmt.Errorf("获取 Webhook 列表失败: %w", err)
	}

	if len(webhooks) == 0 {
		logger.Debug("no webhooks subscribed to event", zap.String("event", event))
		return nil
	}

	// 并发发送到所有 Webhook
	//
	// 这里刻意不沿用调用方传入的 ctx：SendEvent 通常在 HTTP handler 里被调用，
	// 请求返回后该 ctx 立即被 cancel，这些已经派生出去的协程会在发起 HTTP 请求时
	// 直接拿到 context canceled —— 表现为 webhook 静默发不出去。
	// 后台投递用独立 ctx 并自带超时。
	for _, webhook := range webhooks {
		go func(wh *model.Webhook) {
			defer safego.Recover("webhook.send")

			sendCtx, cancel := context.WithTimeout(context.Background(), webhookSendTimeout)
			defer cancel()

			if err := s.SendToWebhook(sendCtx, wh, event, data); err != nil {
				logger.Error("failed to send webhook",
					zap.Uint64("webhook_id", wh.ID),
					zap.String("event", event),
					zap.Error(err),
				)
			}
		}(webhook)
	}

	return nil
}

// SendToWebhook 发送到指定 Webhook
func (s *webhookService) SendToWebhook(ctx context.Context, webhook *model.Webhook, event string, data interface{}) error {
	// 构建请求负载
	payload := WebhookPayload{
		Event:     event,
		Timestamp: time.Now().Unix(),
		Data:      data,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化负载失败: %w", err)
	}

	// 创建日志记录
	webhookLog := &model.WebhookLog{
		WebhookID: webhook.ID,
		Event:     event,
		Payload:   string(payloadBytes),
		Status:    0, // 待发送
	}

	if err := s.webhookLogRepo.Create(ctx, webhookLog); err != nil {
		logger.Warn("failed to create webhook log", zap.Error(err))
	}

	// 发送请求
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhook.URL, bytes.NewReader(payloadBytes))
	if err != nil {
		s.updateLogStatus(ctx, webhookLog, 2, 0, "", err.Error())
		return fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "TicketDesk-Webhook/1.0")
	req.Header.Set("X-Webhook-Event", event)
	req.Header.Set("X-Webhook-Timestamp", fmt.Sprintf("%d", payload.Timestamp))

	// 添加 HMAC 签名
	if webhook.Secret != "" {
		signature := s.generateSignature(payloadBytes, webhook.Secret)
		req.Header.Set("X-Webhook-Signature", signature)
	}

	// 添加自定义请求头
	if webhook.Headers != "" {
		var headers map[string]string
		if err := json.Unmarshal([]byte(webhook.Headers), &headers); err == nil {
			for k, v := range headers {
				req.Header.Set(k, v)
			}
		}
	}

	// 发送请求
	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.updateLogStatus(ctx, webhookLog, 2, 0, "", err.Error())
		return fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		s.updateLogStatus(ctx, webhookLog, 2, resp.StatusCode, "", fmt.Sprintf("读取响应失败: %v", err))
		return fmt.Errorf("读取 webhook 响应失败: %w", err)
	}

	// 判断响应状态
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		s.updateLogStatus(ctx, webhookLog, 1, resp.StatusCode, string(respBody), "")
		logger.Info("webhook sent successfully",
			zap.Uint64("webhook_id", webhook.ID),
			zap.String("event", event),
			zap.Int("status_code", resp.StatusCode),
		)
		return nil
	}

	// 非 2xx 响应
	errMsg := fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody))
	s.updateLogStatus(ctx, webhookLog, 2, resp.StatusCode, string(respBody), errMsg)
	return fmt.Errorf("webhook 返回错误: %s", errMsg)
}

// generateSignature 生成 HMAC-SHA256 签名
func (s *webhookService) generateSignature(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// updateLogStatus 更新日志状态
func (s *webhookService) updateLogStatus(ctx context.Context, log *model.WebhookLog, status int8, responseCode int, responseBody, errorMsg string) {
	log.Status = status
	log.ResponseCode = responseCode
	log.ResponseBody = responseBody
	log.ErrorMessage = errorMsg
	if err := s.webhookLogRepo.Update(ctx, log); err != nil {
		logger.Warn("failed to update webhook log", zap.Error(err))
	}
}
