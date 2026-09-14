// Package service 提供项目通知渠道业务逻辑层
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/kerbos/ticketdesk/internal/core-project/dto"
	"github.com/kerbos/ticketdesk/internal/core-project/repository"
	"github.com/kerbos/ticketdesk/internal/model"
	"github.com/kerbos/ticketdesk/internal/notification/lark"
	"github.com/kerbos/ticketdesk/internal/notification/telegram"
	configService "github.com/kerbos/ticketdesk/internal/system-config/service"
	"github.com/kerbos/ticketdesk/pkg/logger"
	"github.com/kerbos/ticketdesk/pkg/safego"
)

// 业务错误定义
var (
	ErrChannelNotFound = errors.New("project.channel_not_found")
	ErrInvalidConfig   = errors.New("project.channel_invalid")
)

// NotificationChannelService 项目通知渠道服务接口
type NotificationChannelService interface {
	// CRUD
	CreateChannel(ctx context.Context, projectID uint64, req *dto.CreateNotificationChannelRequest, userID uint64) (*dto.NotificationChannelResponse, error)
	GetChannel(ctx context.Context, id uint64) (*dto.NotificationChannelResponse, error)
	UpdateChannel(ctx context.Context, id uint64, req *dto.UpdateNotificationChannelRequest) (*dto.NotificationChannelResponse, error)
	DeleteChannel(ctx context.Context, id uint64) error
	ListChannels(ctx context.Context, projectID uint64) ([]*dto.NotificationChannelResponse, error)

	// 测试发送
	TestChannel(ctx context.Context, id uint64) error

	// 事件通知（发送到项目的所有已启用渠道）
	NotifyProject(ctx context.Context, projectID uint64, event string, data any) error
}

// notificationChannelService 项目通知渠道服务实现
type notificationChannelService struct {
	channelRepo repository.NotificationChannelRepository
	projectRepo repository.ProjectRepository
	configSvc   configService.ConfigService
}

// NewNotificationChannelService 创建项目通知渠道服务实例
func NewNotificationChannelService(
	channelRepo repository.NotificationChannelRepository,
	projectRepo repository.ProjectRepository,
	configSvc configService.ConfigService,
) NotificationChannelService {
	return &notificationChannelService{
		channelRepo: channelRepo,
		projectRepo: projectRepo,
		configSvc:   configSvc,
	}
}

// CreateChannel 创建通知渠道
func (s *notificationChannelService) CreateChannel(ctx context.Context, projectID uint64, req *dto.CreateNotificationChannelRequest, userID uint64) (*dto.NotificationChannelResponse, error) {
	// 验证配置
	configJSON, err := s.validateAndMarshalConfig(req.ChannelType, req.Config)
	if err != nil {
		return nil, err
	}

	eventTypesJSON, err := marshalEventTypes(req.EventTypes)
	if err != nil {
		return nil, err
	}

	channel := &model.ProjectNotificationChannel{
		ProjectID:   projectID,
		ChannelType: req.ChannelType,
		Name:        req.Name,
		Config:      configJSON,
		EventTypes:  eventTypesJSON,
		MentionAll:  req.MentionAll,
		Enabled:     req.Enabled,
		CreatedBy:   userID,
	}

	if err := s.channelRepo.Create(ctx, channel); err != nil {
		return nil, fmt.Errorf("创建通知渠道失败: %w", err)
	}

	logger.Info("notification channel created",
		zap.Uint64("project_id", projectID),
		zap.String("channel_type", req.ChannelType),
		zap.String("name", req.Name),
	)

	return s.toResponse(channel), nil
}

// GetChannel 获取通知渠道
func (s *notificationChannelService) GetChannel(ctx context.Context, id uint64) (*dto.NotificationChannelResponse, error) {
	channel, err := s.channelRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrChannelNotFound
		}
		return nil, fmt.Errorf("获取通知渠道失败: %w", err)
	}
	return s.toResponse(channel), nil
}

// UpdateChannel 更新通知渠道
func (s *notificationChannelService) UpdateChannel(ctx context.Context, id uint64, req *dto.UpdateNotificationChannelRequest) (*dto.NotificationChannelResponse, error) {
	channel, err := s.channelRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrChannelNotFound
		}
		return nil, fmt.Errorf("获取通知渠道失败: %w", err)
	}

	if req.Name != nil {
		channel.Name = *req.Name
	}
	if req.Enabled != nil {
		channel.Enabled = *req.Enabled
	}
	if req.Config != nil {
		configJSON, err := s.validateAndMarshalConfig(channel.ChannelType, req.Config)
		if err != nil {
			return nil, err
		}
		channel.Config = configJSON
	}
	if req.EventTypes != nil {
		eventTypesJSON, err := marshalEventTypes(*req.EventTypes)
		if err != nil {
			return nil, err
		}
		channel.EventTypes = eventTypesJSON
	}
	if req.MentionAll != nil {
		channel.MentionAll = *req.MentionAll
	}

	if err := s.channelRepo.Update(ctx, channel); err != nil {
		return nil, fmt.Errorf("更新通知渠道失败: %w", err)
	}

	logger.Info("notification channel updated", zap.Uint64("channel_id", id))

	return s.toResponse(channel), nil
}

// DeleteChannel 删除通知渠道
func (s *notificationChannelService) DeleteChannel(ctx context.Context, id uint64) error {
	_, err := s.channelRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrChannelNotFound
		}
		return fmt.Errorf("获取通知渠道失败: %w", err)
	}

	if err := s.channelRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("删除通知渠道失败: %w", err)
	}

	logger.Info("notification channel deleted", zap.Uint64("channel_id", id))
	return nil
}

// ListChannels 获取项目的所有通知渠道
func (s *notificationChannelService) ListChannels(ctx context.Context, projectID uint64) ([]*dto.NotificationChannelResponse, error) {
	channels, err := s.channelRepo.ListByProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("查询通知渠道失败: %w", err)
	}

	responses := make([]*dto.NotificationChannelResponse, len(channels))
	for i, ch := range channels {
		responses[i] = s.toResponse(ch)
	}
	return responses, nil
}

// TestChannel 测试通知渠道
func (s *notificationChannelService) TestChannel(ctx context.Context, id uint64) error {
	channel, err := s.channelRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrChannelNotFound
		}
		return fmt.Errorf("获取通知渠道失败: %w", err)
	}

	switch channel.ChannelType {
	case "lark":
		return s.testLarkChannel(ctx, channel)
	case "telegram":
		return s.testTelegramChannel(ctx, channel)
	default:
		return fmt.Errorf("不支持的渠道类型: %s", channel.ChannelType)
	}
}

// NotifyProject 向项目的所有已启用渠道发送通知
func (s *notificationChannelService) NotifyProject(ctx context.Context, projectID uint64, event string, data any) error {
	channels, err := s.channelRepo.ListEnabledByProject(ctx, projectID)
	if err != nil {
		return fmt.Errorf("查询项目通知渠道失败: %w", err)
	}

	if len(channels) == 0 {
		return nil
	}

	for _, ch := range channels {
		// 按渠道订阅的事件类型过滤
		if !channelSubscribesEvent(ch, event) {
			continue
		}
		go func(channel *model.ProjectNotificationChannel) {
			defer safego.Recover("project.notifyChannel")
			var sendErr error
			switch channel.ChannelType {
			case "lark":
				sendErr = s.sendLarkNotification(context.Background(), channel, event, data)
			case "telegram":
				sendErr = s.sendTelegramNotification(context.Background(), channel, event, data)
			}
			if sendErr != nil {
				logger.Warn("failed to send project notification",
					zap.Uint64("project_id", projectID),
					zap.Uint64("channel_id", channel.ID),
					zap.String("channel_type", channel.ChannelType),
					zap.Error(sendErr),
				)
			}
		}(ch)
	}

	return nil
}

// marshalEventTypes 将事件类型数组序列化为 JSON 字符串
func marshalEventTypes(events []string) (string, error) {
	if len(events) == 0 {
		return "", fmt.Errorf("订阅的事件类型不能为空")
	}
	bytes, err := json.Marshal(events)
	if err != nil {
		return "", fmt.Errorf("序列化事件类型失败: %w", err)
	}
	return string(bytes), nil
}

// parseEventTypes 反序列化渠道订阅的事件类型；空值返回 nil
func parseEventTypes(raw string) []string {
	if raw == "" {
		return nil
	}
	var events []string
	if err := json.Unmarshal([]byte(raw), &events); err != nil {
		return nil
	}
	return events
}

// channelSubscribesEvent 判断渠道是否订阅指定事件
// 旧数据 EventTypes 为空 → 视为全订阅，向后兼容（迁移会补默认值，但运行时仍保险）
func channelSubscribesEvent(ch *model.ProjectNotificationChannel, event string) bool {
	events := parseEventTypes(ch.EventTypes)
	if len(events) == 0 {
		return true
	}
	return slices.Contains(events, event)
}

// ============ 内部方法 ============

// validateAndMarshalConfig 验证并序列化渠道配置
func (s *notificationChannelService) validateAndMarshalConfig(channelType string, config any) (string, error) {
	configBytes, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("序列化配置失败: %w", err)
	}

	// 验证配置结构
	switch channelType {
	case "lark":
		var larkConfig dto.LarkChannelConfig
		if err := json.Unmarshal(configBytes, &larkConfig); err != nil {
			logger.Warn("invalid lark channel config", zap.Error(err))
			return "", ErrInvalidConfig
		}
		if larkConfig.WebhookURL == "" {
			logger.Warn("lark channel missing webhook url")
			return "", ErrInvalidConfig
		}
	case "telegram":
		var tgConfig dto.TelegramChannelConfig
		if err := json.Unmarshal(configBytes, &tgConfig); err != nil {
			logger.Warn("invalid telegram channel config", zap.Error(err))
			return "", ErrInvalidConfig
		}
		if tgConfig.ChatID == "" {
			logger.Warn("telegram channel missing chat id")
			return "", ErrInvalidConfig
		}
	default:
		return "", fmt.Errorf("不支持的渠道类型: %s", channelType)
	}

	return string(configBytes), nil
}

// getSiteURL 获取站点 URL
func (s *notificationChannelService) getSiteURL(ctx context.Context) string {
	if s.configSvc != nil {
		url, err := s.configSvc.GetConfigValue(ctx, configService.KeyGeneralSiteURL)
		if err == nil && url != "" {
			return url
		}
	}
	return ""
}

// testLarkChannel 测试飞书渠道
func (s *notificationChannelService) testLarkChannel(ctx context.Context, channel *model.ProjectNotificationChannel) error {
	var config dto.LarkChannelConfig
	if err := json.Unmarshal([]byte(channel.Config), &config); err != nil {
		return fmt.Errorf("解析飞书配置失败: %w", err)
	}

	svc := lark.NewDirectLarkSender(config.WebhookURL, config.Secret, s.getSiteURL(ctx))
	return svc.SendTestMessage(ctx)
}

// testTelegramChannel 测试 Telegram 渠道
func (s *notificationChannelService) testTelegramChannel(ctx context.Context, channel *model.ProjectNotificationChannel) error {
	var config dto.TelegramChannelConfig
	if err := json.Unmarshal([]byte(channel.Config), &config); err != nil {
		return fmt.Errorf("解析 Telegram 配置失败: %w", err)
	}

	svc := telegram.NewDirectTelegramSender(config.BotToken, config.ChatID, s.getSiteURL(ctx))
	return svc.SendTestMessage(ctx)
}

// sendLarkNotification 通过飞书渠道发送通知
func (s *notificationChannelService) sendLarkNotification(ctx context.Context, channel *model.ProjectNotificationChannel, event string, data any) error {
	var config dto.LarkChannelConfig
	if err := json.Unmarshal([]byte(channel.Config), &config); err != nil {
		return fmt.Errorf("解析飞书配置失败: %w", err)
	}

	// 注入渠道级 mention_all 标志：仅对 issue.daily_digest 事件生效
	// 其他事件（创建/流转/指派/告警合并）只 @ 指派人，避免群消息过度打扰
	if channel.MentionAll && event == "issue.daily_digest" {
		if m, ok := data.(map[string]any); ok {
			m["mention_all"] = true
			data = m
		}
	}

	svc := lark.NewDirectLarkSender(config.WebhookURL, config.Secret, s.getSiteURL(ctx))
	return svc.SendNotification(ctx, event, data)
}

// sendTelegramNotification 通过 Telegram 渠道发送通知
func (s *notificationChannelService) sendTelegramNotification(ctx context.Context, channel *model.ProjectNotificationChannel, event string, data any) error {
	var config dto.TelegramChannelConfig
	if err := json.Unmarshal([]byte(channel.Config), &config); err != nil {
		return fmt.Errorf("解析 Telegram 配置失败: %w", err)
	}

	svc := telegram.NewDirectTelegramSender(config.BotToken, config.ChatID, s.getSiteURL(ctx))
	return svc.SendNotification(ctx, event, data)
}

// toResponse 将模型转换为响应 DTO
func (s *notificationChannelService) toResponse(ch *model.ProjectNotificationChannel) *dto.NotificationChannelResponse {
	// 解析 config JSON，隐藏敏感字段
	var configObj any
	var rawConfig map[string]any
	if err := json.Unmarshal([]byte(ch.Config), &rawConfig); err == nil {
		// 隐藏敏感字段
		delete(rawConfig, "secret")
		delete(rawConfig, "bot_token")
		configObj = rawConfig
	} else {
		configObj = ch.Config
	}

	events := parseEventTypes(ch.EventTypes)
	if events == nil {
		events = []string{}
	}

	return &dto.NotificationChannelResponse{
		ID:          ch.ID,
		ProjectID:   ch.ProjectID,
		ChannelType: ch.ChannelType,
		Name:        ch.Name,
		Config:      configObj,
		EventTypes:  events,
		MentionAll:  ch.MentionAll,
		Enabled:     ch.Enabled,
		CreatedBy:   ch.CreatedBy,
		CreatedAt:   ch.CreatedAt,
		UpdatedAt:   ch.UpdatedAt,
	}
}
