// Package service 提供系统配置业务逻辑层
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/kerbos/ticketdesk/internal/model"
	"github.com/kerbos/ticketdesk/internal/system-config/dto"
	"github.com/kerbos/ticketdesk/internal/system-config/repository"
	"github.com/kerbos/ticketdesk/pkg/i18n"
	"github.com/kerbos/ticketdesk/pkg/logger"
	"github.com/kerbos/ticketdesk/pkg/redis"
)

// 配置分类常量
const (
	CategoryEmail     = "email"
	CategoryWebhook   = "webhook"
	CategorySecurity  = "security"
	CategoryGeneral   = "general"
	CategoryLark      = "lark"
	CategoryTelegram  = "telegram"
	CategoryRateLimit = "ratelimit"
	CategoryBrand     = "brand"
)

// 配置键常量
const (
	// 邮件配置键
	KeyEmailSMTPHost     = "email.smtp_host"
	KeyEmailSMTPPort     = "email.smtp_port"
	KeyEmailSMTPUsername = "email.smtp_username"
	KeyEmailSMTPPassword = "email.smtp_password"
	KeyEmailFromAddress  = "email.from_address"
	KeyEmailFromName     = "email.from_name"
	KeyEmailUseTLS       = "email.use_tls"
	KeyEmailEnabled      = "email.enabled"

	// 安全配置键
	KeySecurityMFAEnabled            = "security.mfa_enabled"
	KeySecurityMFARequired           = "security.mfa_required"
	KeySecurityPasswordMinLength     = "security.password_min_length"
	KeySecurityPasswordRequireUpper  = "security.password_require_upper"
	KeySecurityPasswordRequireNumber = "security.password_require_number"
	KeySecuritySessionTimeout        = "security.session_timeout"
	KeySecurityWebhookSecret         = "security.webhook_secret" // 告警 Webhook HMAC 密钥；留空则不校验签名

	// 通用配置键
	KeyGeneralSiteURL = "general.site_url" // 站点域名
	// KeyGeneralLanguage 平台默认语言（zh-CN / en-US）。
	// 决定群消息、日报，以及未单独设置语言的用户收到的站内通知与邮件。
	KeyGeneralLanguage = "general.language"

	// CategorySetup 初始化向导配置分类
	CategorySetup = "setup"

	// KeySetupCompleted 是否已完成首次初始化（"1" 为已完成）
	//
	// 不按"users 表是否为空"判定：管理员被删掉的实例会被误判成新装，
	// 那样任何人都能重新跑一遍向导、把自己变成管理员。
	KeySetupCompleted = "setup.completed"

	// KeySetupTokenHash 自动生成的 setup token 的哈希
	//
	// 只在没有通过环境变量提供 token 时才有值。存哈希不存明文：
	// 明文只在生成它的那个副本的启动日志里出现一次。
	KeySetupTokenHash = "setup.token_hash"

	// 飞书配置键
	KeyLarkEnabled    = "lark.enabled"
	KeyLarkWebhookURL = "lark.webhook_url"
	KeyLarkSecret     = "lark.secret"

	// Telegram 配置键
	KeyTelegramEnabled  = "telegram.enabled"
	KeyTelegramBotToken = "telegram.bot_token"
	KeyTelegramChatID   = "telegram.chat_id"

	// 限流配置键
	KeyRateLimitWebhook = "ratelimit.webhook_limit" // 默认 100
	KeyRateLimitAuth    = "ratelimit.auth_limit"    // 默认 20
	KeyRateLimitAPI     = "ratelimit.api_limit"     // 默认 300

	// SSO 配置分类
	CategorySSO = "sso"

	// SSO 配置键
	KeySSOEnabled        = "sso.enabled"
	KeySSOProviderName   = "sso.provider_name"
	KeySSOClientID       = "sso.client_id"
	KeySSOClientSecret   = "sso.client_secret"
	KeySSOIssuerURL      = "sso.issuer_url"
	KeySSORedirectURI    = "sso.redirect_uri"
	KeySSOScopes         = "sso.scopes"
	KeySSOAutoCreateUser = "sso.auto_create_user"
	KeySSODefaultRole    = "sso.default_role"
	KeySSOClaimMappings  = "sso.claim_mappings" // JSON: [{"local_field":"username","claim_name":"preferred_username"}, ...]

	// 品牌配置键
	KeyBrandSystemName        = "brand.system_name"
	KeyBrandSystemDescription = "brand.system_description"
	KeyBrandCopyrightText     = "brand.copyright_text"
	KeyBrandLogoURL           = "brand.logo_url"
	KeyBrandFaviconURL        = "brand.favicon_url"
	KeyBrandLoginTitle        = "brand.login_title"
	KeyBrandLoginDescription  = "brand.login_description"
)

// 业务错误定义
var (
	ErrConfigNotFound  = errors.New("system.config_not_found")
	ErrWebhookNotFound = errors.New("system.webhook_not_found")
)

// ConfigService 系统配置服务接口
type ConfigService interface {
	// 配置管理
	GetConfig(ctx context.Context, key string) (*dto.ConfigResponse, error)
	GetConfigValue(ctx context.Context, key string) (string, error)
	GetConfigsByCategory(ctx context.Context, category string) ([]*dto.ConfigResponse, error)
	GetAllConfigs(ctx context.Context) ([]*dto.ConfigResponse, error)
	UpdateConfig(ctx context.Context, key, value string, userID uint64) error
	BatchUpdateConfigs(ctx context.Context, configs map[string]string, userID uint64) error

	// 邮件配置
	GetEmailConfig(ctx context.Context) (*dto.EmailConfig, error)
	UpdateEmailConfig(ctx context.Context, req *dto.UpdateEmailConfigRequest, userID uint64) error

	// 安全配置
	GetSecurityConfig(ctx context.Context) (*dto.SecurityConfig, error)
	UpdateSecurityConfig(ctx context.Context, req *dto.UpdateSecurityConfigRequest, userID uint64) error

	// 飞书配置
	GetLarkConfig(ctx context.Context) (*dto.LarkConfig, error)
	UpdateLarkConfig(ctx context.Context, req *dto.UpdateLarkConfigRequest, userID uint64) error

	// Telegram 配置
	GetTelegramConfig(ctx context.Context) (*dto.TelegramConfig, error)
	UpdateTelegramConfig(ctx context.Context, req *dto.UpdateTelegramConfigRequest, userID uint64) error

	// 限流配置
	GetRateLimitConfig(ctx context.Context) (*dto.RateLimitConfig, error)
	UpdateRateLimitConfig(ctx context.Context, req *dto.UpdateRateLimitConfigRequest, userID uint64) error

	// SSO 配置
	GetSSOConfig(ctx context.Context) (*dto.SSOConfig, error)
	UpdateSSOConfig(ctx context.Context, req *dto.UpdateSSOConfigRequest, userID uint64) error

	// 品牌配置
	GetBrandConfig(ctx context.Context) (*dto.BrandConfig, error)
	UpdateBrandConfig(ctx context.Context, req *dto.UpdateBrandConfigRequest, userID uint64) error

	// Webhook 管理
	CreateWebhook(ctx context.Context, req *dto.CreateWebhookRequest, userID uint64) (*dto.WebhookResponse, error)
	GetWebhook(ctx context.Context, id uint64) (*dto.WebhookResponse, error)
	UpdateWebhook(ctx context.Context, id uint64, req *dto.UpdateWebhookRequest) (*dto.WebhookResponse, error)
	DeleteWebhook(ctx context.Context, id uint64) error
	ListWebhooks(ctx context.Context, req *dto.ListWebhooksRequest) ([]*dto.WebhookResponse, int64, error)

	// Webhook 日志
	ListWebhookLogs(ctx context.Context, req *dto.ListWebhookLogsRequest) ([]*dto.WebhookLogResponse, int64, error)

	// 缓存操作
	InvalidateCache(ctx context.Context, key string) error
	InvalidateAllCache(ctx context.Context) error
}

// configService 系统配置服务实现
type configService struct {
	configRepo     repository.ConfigRepository
	webhookRepo    repository.WebhookRepository
	webhookLogRepo repository.WebhookLogRepository
	cache          map[string]*cacheEntry
	cacheMu        sync.RWMutex
	cacheTTL       time.Duration
}

// cacheEntry 缓存条目
type cacheEntry struct {
	value     string
	expiresAt time.Time
}

// NewConfigService 创建系统配置服务实例
func NewConfigService(
	configRepo repository.ConfigRepository,
	webhookRepo repository.WebhookRepository,
	webhookLogRepo repository.WebhookLogRepository,
) ConfigService {
	return &configService{
		configRepo:     configRepo,
		webhookRepo:    webhookRepo,
		webhookLogRepo: webhookLogRepo,
		cache:          make(map[string]*cacheEntry),
		cacheTTL:       5 * time.Minute,
	}
}

// ============ 配置管理 ============

// GetConfig 获取配置
func (s *configService) GetConfig(ctx context.Context, key string) (*dto.ConfigResponse, error) {
	config, err := s.configRepo.GetByKey(ctx, key)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrConfigNotFound
		}
		return nil, fmt.Errorf("获取配置失败: %w", err)
	}
	return s.toConfigResponse(config), nil
}

// GetConfigValue 获取配置值（带缓存）
func (s *configService) GetConfigValue(ctx context.Context, key string) (string, error) {
	// 先从内存缓存获取
	s.cacheMu.RLock()
	if entry, ok := s.cache[key]; ok && time.Now().Before(entry.expiresAt) {
		s.cacheMu.RUnlock()
		return entry.value, nil
	}
	s.cacheMu.RUnlock()

	// 尝试从 Redis 获取
	if redis.Client != nil {
		val, err := redis.Client.Get(ctx, "config:"+key).Result()
		if err == nil {
			s.setLocalCache(key, val)
			return val, nil
		}
	}

	// 从数据库获取
	config, err := s.configRepo.GetByKey(ctx, key)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrConfigNotFound
		}
		return "", fmt.Errorf("获取配置失败: %w", err)
	}

	// 更新缓存
	s.setLocalCache(key, config.ConfigValue)
	if redis.Client != nil {
		redis.Client.Set(ctx, "config:"+key, config.ConfigValue, s.cacheTTL)
	}

	return config.ConfigValue, nil
}

// GetConfigsByCategory 根据分类获取配置
func (s *configService) GetConfigsByCategory(ctx context.Context, category string) ([]*dto.ConfigResponse, error) {
	configs, err := s.configRepo.GetByCategory(ctx, category)
	if err != nil {
		return nil, fmt.Errorf("获取配置列表失败: %w", err)
	}

	responses := make([]*dto.ConfigResponse, len(configs))
	for i, config := range configs {
		responses[i] = s.toConfigResponse(config)
	}
	return responses, nil
}

// GetAllConfigs 获取所有配置
func (s *configService) GetAllConfigs(ctx context.Context) ([]*dto.ConfigResponse, error) {
	configs, err := s.configRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取配置列表失败: %w", err)
	}

	responses := make([]*dto.ConfigResponse, len(configs))
	for i, config := range configs {
		responses[i] = s.toConfigResponse(config)
	}
	return responses, nil
}

// UpdateConfig 更新配置
func (s *configService) UpdateConfig(ctx context.Context, key, value string, userID uint64) error {
	config := &model.SystemConfig{
		ConfigKey:   key,
		ConfigValue: value,
		UpdatedBy:   &userID,
	}
	// 从 key 前缀推断 category（如 "brand.logo_url" → "brand"），确保新建记录有正确的分类
	if parts := strings.SplitN(key, ".", 2); len(parts) == 2 {
		config.Category = parts[0]
	}

	if err := s.configRepo.Upsert(ctx, config); err != nil {
		return fmt.Errorf("更新配置失败: %w", err)
	}

	// 清除缓存
	_ = s.InvalidateCache(ctx, key)

	// 平台语言立刻在本副本生效，管理员保存完发条测试消息就能看到效果；
	// 其余副本由 StartLanguageSync 在一个间隔内收敛
	if key == KeyGeneralLanguage {
		i18n.SetBackgroundLang(value)
	}

	logger.Info("config updated",
		zap.String("key", key),
		zap.Uint64("updated_by", userID),
	)

	return nil
}

// BatchUpdateConfigs 批量更新配置
func (s *configService) BatchUpdateConfigs(ctx context.Context, configs map[string]string, userID uint64) error {
	modelConfigs := make([]*model.SystemConfig, 0, len(configs))
	for key, value := range configs {
		modelConfigs = append(modelConfigs, &model.SystemConfig{
			ConfigKey:   key,
			ConfigValue: value,
			UpdatedBy:   &userID,
		})
	}

	if err := s.configRepo.BatchUpsert(ctx, modelConfigs); err != nil {
		return fmt.Errorf("批量更新配置失败: %w", err)
	}

	// 清除所有缓存
	_ = s.InvalidateAllCache(ctx)

	if lang, ok := configs[KeyGeneralLanguage]; ok {
		i18n.SetBackgroundLang(lang)
	}

	logger.Info("configs batch updated",
		zap.Int("count", len(configs)),
		zap.Uint64("updated_by", userID),
	)

	return nil
}

// ============ 邮件配置 ============

// GetEmailConfig 获取邮件配置
func (s *configService) GetEmailConfig(ctx context.Context) (*dto.EmailConfig, error) {
	configs, err := s.configRepo.GetByCategory(ctx, CategoryEmail)
	if err != nil {
		return nil, fmt.Errorf("获取邮件配置失败: %w", err)
	}

	config := &dto.EmailConfig{}
	for _, c := range configs {
		switch c.ConfigKey {
		case KeyEmailSMTPHost:
			config.SMTPHost = c.ConfigValue
		case KeyEmailSMTPPort:
			if port, err := strconv.Atoi(c.ConfigValue); err == nil {
				config.SMTPPort = port
			}
		case KeyEmailSMTPUsername:
			config.SMTPUsername = c.ConfigValue
		case KeyEmailSMTPPassword:
			// 密码不返回
		case KeyEmailFromAddress:
			config.FromAddress = c.ConfigValue
		case KeyEmailFromName:
			config.FromName = c.ConfigValue
		case KeyEmailUseTLS:
			config.UseTLS = c.ConfigValue == "true"
		case KeyEmailEnabled:
			config.Enabled = c.ConfigValue == "true"
		}
	}

	return config, nil
}

// UpdateEmailConfig 更新邮件配置
func (s *configService) UpdateEmailConfig(ctx context.Context, req *dto.UpdateEmailConfigRequest, userID uint64) error {
	configs := []*model.SystemConfig{
		{ConfigKey: KeyEmailSMTPHost, ConfigValue: req.SMTPHost, ConfigType: "string", Category: CategoryEmail, Description: "SMTP 服务器地址"},
		{ConfigKey: KeyEmailSMTPPort, ConfigValue: strconv.Itoa(req.SMTPPort), ConfigType: "number", Category: CategoryEmail, Description: "SMTP 服务器端口"},
		{ConfigKey: KeyEmailSMTPUsername, ConfigValue: req.SMTPUsername, ConfigType: "string", Category: CategoryEmail, Description: "SMTP 用户名"},
		{ConfigKey: KeyEmailFromAddress, ConfigValue: req.FromAddress, ConfigType: "string", Category: CategoryEmail, Description: "发件人地址"},
		{ConfigKey: KeyEmailFromName, ConfigValue: req.FromName, ConfigType: "string", Category: CategoryEmail, Description: "发件人名称"},
		{ConfigKey: KeyEmailUseTLS, ConfigValue: strconv.FormatBool(req.UseTLS), ConfigType: "boolean", Category: CategoryEmail, Description: "是否使用 TLS"},
		{ConfigKey: KeyEmailEnabled, ConfigValue: strconv.FormatBool(req.Enabled), ConfigType: "boolean", Category: CategoryEmail, Description: "是否启用邮件通知"},
	}

	// 如果密码不为空，则更新密码
	if req.SMTPPassword != "" {
		configs = append(configs, &model.SystemConfig{
			ConfigKey:   KeyEmailSMTPPassword,
			ConfigValue: req.SMTPPassword,
			ConfigType:  "string",
			Category:    CategoryEmail,
			Description: "SMTP 密码",
			IsSecret:    true,
		})
	}

	for _, c := range configs {
		c.UpdatedBy = &userID
	}

	if err := s.configRepo.BatchUpsert(ctx, configs); err != nil {
		return fmt.Errorf("更新邮件配置失败: %w", err)
	}

	// 清除缓存
	_ = s.InvalidateAllCache(ctx)

	logger.Info("email config updated", zap.Uint64("updated_by", userID))

	return nil
}

// ============ 安全配置 ============

// GetSecurityConfig 获取安全配置
func (s *configService) GetSecurityConfig(ctx context.Context) (*dto.SecurityConfig, error) {
	configs, err := s.configRepo.GetByCategory(ctx, CategorySecurity)
	if err != nil {
		return nil, fmt.Errorf("获取安全配置失败: %w", err)
	}

	// 默认值
	config := &dto.SecurityConfig{
		MFAEnabled:            false,
		MFARequired:           false,
		PasswordMinLength:     6,
		PasswordRequireUpper:  false,
		PasswordRequireNumber: false,
		SessionTimeout:        120, // 2小时
	}

	for _, c := range configs {
		switch c.ConfigKey {
		case KeySecurityMFAEnabled:
			config.MFAEnabled = c.ConfigValue == "true"
		case KeySecurityMFARequired:
			config.MFARequired = c.ConfigValue == "true"
		case KeySecurityPasswordMinLength:
			if length, err := strconv.Atoi(c.ConfigValue); err == nil {
				config.PasswordMinLength = length
			}
		case KeySecurityPasswordRequireUpper:
			config.PasswordRequireUpper = c.ConfigValue == "true"
		case KeySecurityPasswordRequireNumber:
			config.PasswordRequireNumber = c.ConfigValue == "true"
		case KeySecuritySessionTimeout:
			if timeout, err := strconv.Atoi(c.ConfigValue); err == nil {
				config.SessionTimeout = timeout
			}
		}
	}

	return config, nil
}

// UpdateSecurityConfig 更新安全配置
func (s *configService) UpdateSecurityConfig(ctx context.Context, req *dto.UpdateSecurityConfigRequest, userID uint64) error {
	var configs []*model.SystemConfig

	if req.MFAEnabled != nil {
		configs = append(configs, &model.SystemConfig{
			ConfigKey:   KeySecurityMFAEnabled,
			ConfigValue: strconv.FormatBool(*req.MFAEnabled),
			ConfigType:  "boolean",
			Category:    CategorySecurity,
			Description: "是否启用 MFA",
			UpdatedBy:   &userID,
		})
	}

	if req.MFARequired != nil {
		configs = append(configs, &model.SystemConfig{
			ConfigKey:   KeySecurityMFARequired,
			ConfigValue: strconv.FormatBool(*req.MFARequired),
			ConfigType:  "boolean",
			Category:    CategorySecurity,
			Description: "是否强制要求 MFA",
			UpdatedBy:   &userID,
		})
	}

	if req.PasswordMinLength != nil {
		configs = append(configs, &model.SystemConfig{
			ConfigKey:   KeySecurityPasswordMinLength,
			ConfigValue: strconv.Itoa(*req.PasswordMinLength),
			ConfigType:  "number",
			Category:    CategorySecurity,
			Description: "密码最小长度",
			UpdatedBy:   &userID,
		})
	}

	if req.PasswordRequireUpper != nil {
		configs = append(configs, &model.SystemConfig{
			ConfigKey:   KeySecurityPasswordRequireUpper,
			ConfigValue: strconv.FormatBool(*req.PasswordRequireUpper),
			ConfigType:  "boolean",
			Category:    CategorySecurity,
			Description: "密码是否需要大写字母",
			UpdatedBy:   &userID,
		})
	}

	if req.PasswordRequireNumber != nil {
		configs = append(configs, &model.SystemConfig{
			ConfigKey:   KeySecurityPasswordRequireNumber,
			ConfigValue: strconv.FormatBool(*req.PasswordRequireNumber),
			ConfigType:  "boolean",
			Category:    CategorySecurity,
			Description: "密码是否需要数字",
			UpdatedBy:   &userID,
		})
	}

	if req.SessionTimeout != nil {
		configs = append(configs, &model.SystemConfig{
			ConfigKey:   KeySecuritySessionTimeout,
			ConfigValue: strconv.Itoa(*req.SessionTimeout),
			ConfigType:  "number",
			Category:    CategorySecurity,
			Description: "会话超时时间（分钟）",
			UpdatedBy:   &userID,
		})
	}

	if len(configs) > 0 {
		if err := s.configRepo.BatchUpsert(ctx, configs); err != nil {
			return fmt.Errorf("更新安全配置失败: %w", err)
		}
	}

	// 清除缓存
	_ = s.InvalidateAllCache(ctx)

	logger.Info("security config updated", zap.Uint64("updated_by", userID))

	return nil
}

// ============ 飞书配置 ============

// GetLarkConfig 获取飞书配置
func (s *configService) GetLarkConfig(ctx context.Context) (*dto.LarkConfig, error) {
	configs, err := s.configRepo.GetByCategory(ctx, CategoryLark)
	if err != nil {
		return nil, fmt.Errorf("获取飞书配置失败: %w", err)
	}

	config := &dto.LarkConfig{}
	for _, c := range configs {
		switch c.ConfigKey {
		case KeyLarkEnabled:
			config.Enabled = c.ConfigValue == "true"
		case KeyLarkWebhookURL:
			config.WebhookURL = c.ConfigValue
		case KeyLarkSecret:
			// 密钥不返回
		}
	}

	return config, nil
}

// UpdateLarkConfig 更新飞书配置
func (s *configService) UpdateLarkConfig(ctx context.Context, req *dto.UpdateLarkConfigRequest, userID uint64) error {
	configs := []*model.SystemConfig{
		{ConfigKey: KeyLarkEnabled, ConfigValue: strconv.FormatBool(req.Enabled), ConfigType: "boolean", Category: CategoryLark, Description: "是否启用飞书通知"},
		{ConfigKey: KeyLarkWebhookURL, ConfigValue: req.WebhookURL, ConfigType: "string", Category: CategoryLark, Description: "飞书机器人 Webhook URL"},
	}

	// 如果密钥不为空，则更新密钥
	if req.Secret != "" {
		configs = append(configs, &model.SystemConfig{
			ConfigKey:   KeyLarkSecret,
			ConfigValue: req.Secret,
			ConfigType:  "string",
			Category:    CategoryLark,
			Description: "飞书机器人签名密钥",
			IsSecret:    true,
		})
	}

	for _, c := range configs {
		c.UpdatedBy = &userID
	}

	if err := s.configRepo.BatchUpsert(ctx, configs); err != nil {
		return fmt.Errorf("更新飞书配置失败: %w", err)
	}

	// 清除缓存
	_ = s.InvalidateAllCache(ctx)

	logger.Info("lark config updated", zap.Uint64("updated_by", userID))

	return nil
}

// ============ Telegram 配置 ============

// GetTelegramConfig 获取 Telegram 配置
func (s *configService) GetTelegramConfig(ctx context.Context) (*dto.TelegramConfig, error) {
	configs, err := s.configRepo.GetByCategory(ctx, CategoryTelegram)
	if err != nil {
		return nil, fmt.Errorf("获取 Telegram 配置失败: %w", err)
	}

	config := &dto.TelegramConfig{}
	for _, c := range configs {
		switch c.ConfigKey {
		case KeyTelegramEnabled:
			config.Enabled = c.ConfigValue == "true"
		case KeyTelegramBotToken:
			// Token 不返回
		case KeyTelegramChatID:
			config.ChatID = c.ConfigValue
		}
	}

	return config, nil
}

// UpdateTelegramConfig 更新 Telegram 配置
func (s *configService) UpdateTelegramConfig(ctx context.Context, req *dto.UpdateTelegramConfigRequest, userID uint64) error {
	configs := []*model.SystemConfig{
		{ConfigKey: KeyTelegramEnabled, ConfigValue: strconv.FormatBool(req.Enabled), ConfigType: "boolean", Category: CategoryTelegram, Description: "是否启用 Telegram 通知"},
		{ConfigKey: KeyTelegramChatID, ConfigValue: req.ChatID, ConfigType: "string", Category: CategoryTelegram, Description: "Telegram Chat ID"},
	}

	// 如果 Token 不为空，则更新 Token
	if req.BotToken != "" {
		configs = append(configs, &model.SystemConfig{
			ConfigKey:   KeyTelegramBotToken,
			ConfigValue: req.BotToken,
			ConfigType:  "string",
			Category:    CategoryTelegram,
			Description: "Telegram Bot Token",
			IsSecret:    true,
		})
	}

	for _, c := range configs {
		c.UpdatedBy = &userID
	}

	if err := s.configRepo.BatchUpsert(ctx, configs); err != nil {
		return fmt.Errorf("更新 Telegram 配置失败: %w", err)
	}

	// 清除缓存
	_ = s.InvalidateAllCache(ctx)

	logger.Info("telegram config updated", zap.Uint64("updated_by", userID))

	return nil
}

// ============ 限流配置 ============

// GetRateLimitConfig 获取限流配置
func (s *configService) GetRateLimitConfig(ctx context.Context) (*dto.RateLimitConfig, error) {
	configs, err := s.configRepo.GetByCategory(ctx, CategoryRateLimit)
	if err != nil {
		return nil, fmt.Errorf("获取限流配置失败: %w", err)
	}

	// 默认值
	config := &dto.RateLimitConfig{
		WebhookLimit: 100,
		AuthLimit:    20,
		APILimit:     300,
	}

	for _, c := range configs {
		switch c.ConfigKey {
		case KeyRateLimitWebhook:
			if v, err := strconv.Atoi(c.ConfigValue); err == nil {
				config.WebhookLimit = v
			}
		case KeyRateLimitAuth:
			if v, err := strconv.Atoi(c.ConfigValue); err == nil {
				config.AuthLimit = v
			}
		case KeyRateLimitAPI:
			if v, err := strconv.Atoi(c.ConfigValue); err == nil {
				config.APILimit = v
			}
		}
	}

	return config, nil
}

// UpdateRateLimitConfig 更新限流配置
func (s *configService) UpdateRateLimitConfig(ctx context.Context, req *dto.UpdateRateLimitConfigRequest, userID uint64) error {
	var configs []*model.SystemConfig

	if req.WebhookLimit != nil {
		configs = append(configs, &model.SystemConfig{
			ConfigKey:   KeyRateLimitWebhook,
			ConfigValue: strconv.Itoa(*req.WebhookLimit),
			ConfigType:  "number",
			Category:    CategoryRateLimit,
			Description: "Webhook 接口每 IP 每分钟最大请求数",
			UpdatedBy:   &userID,
		})
	}

	if req.AuthLimit != nil {
		configs = append(configs, &model.SystemConfig{
			ConfigKey:   KeyRateLimitAuth,
			ConfigValue: strconv.Itoa(*req.AuthLimit),
			ConfigType:  "number",
			Category:    CategoryRateLimit,
			Description: "认证接口每 IP 每分钟最大请求数",
			UpdatedBy:   &userID,
		})
	}

	if req.APILimit != nil {
		configs = append(configs, &model.SystemConfig{
			ConfigKey:   KeyRateLimitAPI,
			ConfigValue: strconv.Itoa(*req.APILimit),
			ConfigType:  "number",
			Category:    CategoryRateLimit,
			Description: "全局 API 每 IP 每分钟最大请求数",
			UpdatedBy:   &userID,
		})
	}

	if len(configs) > 0 {
		if err := s.configRepo.BatchUpsert(ctx, configs); err != nil {
			return fmt.Errorf("更新限流配置失败: %w", err)
		}
	}

	// 清除缓存
	_ = s.InvalidateAllCache(ctx)

	logger.Info("ratelimit config updated", zap.Uint64("updated_by", userID))

	return nil
}

// ============ SSO 配置 ============

// GetSSOConfig 获取 SSO 配置
func (s *configService) GetSSOConfig(ctx context.Context) (*dto.SSOConfig, error) {
	configs, err := s.configRepo.GetByCategory(ctx, CategorySSO)
	if err != nil {
		return nil, fmt.Errorf("获取 SSO 配置失败: %w", err)
	}

	// 默认值
	config := &dto.SSOConfig{
		Enabled:        false,
		ProviderName:   "企业统一认证",
		RedirectURI:    "http://localhost:5173/auth/sso/callback",
		Scopes:         "openid,profile,email",
		AutoCreateUser: true,
		DefaultRole:    "user",
		ClaimMappings: []dto.SSOClaimMapping{
			{LocalField: "username", ClaimName: "preferred_username"},
			{LocalField: "email", ClaimName: "email"},
			{LocalField: "display_name", ClaimName: "name"},
			{LocalField: "avatar", ClaimName: "picture"},
		},
	}

	for _, c := range configs {
		switch c.ConfigKey {
		case KeySSOEnabled:
			config.Enabled = c.ConfigValue == "true"
		case KeySSOProviderName:
			config.ProviderName = c.ConfigValue
		case KeySSOClientID:
			config.ClientID = c.ConfigValue
		case KeySSOClientSecret:
			// 密钥不返回
		case KeySSOIssuerURL:
			config.IssuerURL = c.ConfigValue
		case KeySSORedirectURI:
			config.RedirectURI = c.ConfigValue
		case KeySSOScopes:
			config.Scopes = c.ConfigValue
		case KeySSOAutoCreateUser:
			config.AutoCreateUser = c.ConfigValue == "true"
		case KeySSODefaultRole:
			config.DefaultRole = c.ConfigValue
		case KeySSOClaimMappings:
			var mappings []dto.SSOClaimMapping
			if err := json.Unmarshal([]byte(c.ConfigValue), &mappings); err == nil && len(mappings) > 0 {
				config.ClaimMappings = mappings
			}
		}
	}

	return config, nil
}

// UpdateSSOConfig 更新 SSO 配置
func (s *configService) UpdateSSOConfig(ctx context.Context, req *dto.UpdateSSOConfigRequest, userID uint64) error {
	// 序列化 claim mappings 为 JSON
	claimMappingsJSON, err := json.Marshal(req.ClaimMappings)
	if err != nil {
		return fmt.Errorf("序列化 claim mappings 失败: %w", err)
	}

	configs := []*model.SystemConfig{
		{ConfigKey: KeySSOEnabled, ConfigValue: strconv.FormatBool(req.Enabled), ConfigType: "boolean", Category: CategorySSO, Description: "是否启用 SSO"},
		{ConfigKey: KeySSOProviderName, ConfigValue: req.ProviderName, ConfigType: "string", Category: CategorySSO, Description: "SSO 提供方名称"},
		{ConfigKey: KeySSOClientID, ConfigValue: req.ClientID, ConfigType: "string", Category: CategorySSO, Description: "OIDC Client ID"},
		{ConfigKey: KeySSOIssuerURL, ConfigValue: req.IssuerURL, ConfigType: "string", Category: CategorySSO, Description: "OIDC Issuer URL"},
		{ConfigKey: KeySSORedirectURI, ConfigValue: req.RedirectURI, ConfigType: "string", Category: CategorySSO, Description: "OIDC 回调地址"},
		{ConfigKey: KeySSOScopes, ConfigValue: req.Scopes, ConfigType: "string", Category: CategorySSO, Description: "OIDC Scopes"},
		{ConfigKey: KeySSOAutoCreateUser, ConfigValue: strconv.FormatBool(req.AutoCreateUser), ConfigType: "boolean", Category: CategorySSO, Description: "是否自动创建用户"},
		{ConfigKey: KeySSODefaultRole, ConfigValue: req.DefaultRole, ConfigType: "string", Category: CategorySSO, Description: "自动创建用户的默认角色"},
		{ConfigKey: KeySSOClaimMappings, ConfigValue: string(claimMappingsJSON), ConfigType: "json", Category: CategorySSO, Description: "Claims 映射配置"},
	}

	// 如果 ClientSecret 不为空，则更新
	if req.ClientSecret != "" {
		configs = append(configs, &model.SystemConfig{
			ConfigKey:   KeySSOClientSecret,
			ConfigValue: req.ClientSecret,
			ConfigType:  "string",
			Category:    CategorySSO,
			Description: "OIDC Client Secret",
			IsSecret:    true,
		})
	}

	for _, c := range configs {
		c.UpdatedBy = &userID
	}

	if err := s.configRepo.BatchUpsert(ctx, configs); err != nil {
		return fmt.Errorf("更新 SSO 配置失败: %w", err)
	}

	// 清除缓存
	_ = s.InvalidateAllCache(ctx)

	logger.Info("SSO config updated", zap.Uint64("updated_by", userID))

	return nil
}

// ============ 品牌配置 ============

// GetBrandConfig 获取品牌配置
func (s *configService) GetBrandConfig(ctx context.Context) (*dto.BrandConfig, error) {
	configs, err := s.configRepo.GetByCategory(ctx, CategoryBrand)
	if err != nil {
		return nil, fmt.Errorf("获取品牌配置失败: %w", err)
	}

	// 默认值
	config := &dto.BrandConfig{
		SystemName:        "TicketDesk",
		SystemDescription: "项目化工单与告警联动系统",
		CopyrightText:     "© 2026 TicketDesk. All rights reserved.",
		LoginTitle:        "工单与告警联动系统",
		LoginDescription:  "一切问题都是工单，一切告警都必须被跟进。\n为运维与技术团队打造的项目化工单管理平台。",
	}

	for _, c := range configs {
		switch c.ConfigKey {
		case KeyBrandSystemName:
			if c.ConfigValue != "" {
				config.SystemName = c.ConfigValue
			}
		case KeyBrandSystemDescription:
			config.SystemDescription = c.ConfigValue
		case KeyBrandCopyrightText:
			if c.ConfigValue != "" {
				config.CopyrightText = c.ConfigValue
			}
		case KeyBrandLogoURL:
			config.LogoURL = c.ConfigValue
		case KeyBrandFaviconURL:
			config.FaviconURL = c.ConfigValue
		case KeyBrandLoginTitle:
			if c.ConfigValue != "" {
				config.LoginTitle = c.ConfigValue
			}
		case KeyBrandLoginDescription:
			if c.ConfigValue != "" {
				config.LoginDescription = c.ConfigValue
			}
		}
	}

	return config, nil
}

// UpdateBrandConfig 更新品牌配置
func (s *configService) UpdateBrandConfig(ctx context.Context, req *dto.UpdateBrandConfigRequest, userID uint64) error {
	configs := []*model.SystemConfig{
		{ConfigKey: KeyBrandSystemName, ConfigValue: req.SystemName, ConfigType: "string", Category: CategoryBrand, Description: "系统名称"},
		{ConfigKey: KeyBrandSystemDescription, ConfigValue: req.SystemDescription, ConfigType: "string", Category: CategoryBrand, Description: "系统描述"},
		{ConfigKey: KeyBrandCopyrightText, ConfigValue: req.CopyrightText, ConfigType: "string", Category: CategoryBrand, Description: "版权信息"},
		{ConfigKey: KeyBrandLoginTitle, ConfigValue: req.LoginTitle, ConfigType: "string", Category: CategoryBrand, Description: "登录页标题"},
		{ConfigKey: KeyBrandLoginDescription, ConfigValue: req.LoginDescription, ConfigType: "string", Category: CategoryBrand, Description: "登录页描述"},
	}

	for _, c := range configs {
		c.UpdatedBy = &userID
	}

	if err := s.configRepo.BatchUpsert(ctx, configs); err != nil {
		return fmt.Errorf("更新品牌配置失败: %w", err)
	}

	// 清除缓存
	_ = s.InvalidateAllCache(ctx)

	logger.Info("brand config updated", zap.Uint64("updated_by", userID))

	return nil
}

// ============ Webhook 管理 ============

// CreateWebhook 创建 Webhook
func (s *configService) CreateWebhook(ctx context.Context, req *dto.CreateWebhookRequest, userID uint64) (*dto.WebhookResponse, error) {
	eventsJSON, _ := json.Marshal(req.Events)
	headersJSON, _ := json.Marshal(req.Headers)

	webhook := &model.Webhook{
		Name:        req.Name,
		URL:         req.URL,
		Secret:      req.Secret,
		Events:      string(eventsJSON),
		Headers:     string(headersJSON),
		Description: req.Description,
		Status:      1,
		CreatedBy:   userID,
	}

	if err := s.webhookRepo.Create(ctx, webhook); err != nil {
		return nil, fmt.Errorf("创建 Webhook 失败: %w", err)
	}

	logger.Info("webhook created",
		zap.Uint64("webhook_id", webhook.ID),
		zap.String("name", webhook.Name),
		zap.Uint64("created_by", userID),
	)

	return s.toWebhookResponse(webhook), nil
}

// GetWebhook 获取 Webhook
func (s *configService) GetWebhook(ctx context.Context, id uint64) (*dto.WebhookResponse, error) {
	webhook, err := s.webhookRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWebhookNotFound
		}
		return nil, fmt.Errorf("获取 Webhook 失败: %w", err)
	}
	return s.toWebhookResponse(webhook), nil
}

// UpdateWebhook 更新 Webhook
func (s *configService) UpdateWebhook(ctx context.Context, id uint64, req *dto.UpdateWebhookRequest) (*dto.WebhookResponse, error) {
	webhook, err := s.webhookRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWebhookNotFound
		}
		return nil, fmt.Errorf("获取 Webhook 失败: %w", err)
	}

	if req.Name != nil {
		webhook.Name = *req.Name
	}
	if req.URL != nil {
		webhook.URL = *req.URL
	}
	if req.Secret != nil {
		webhook.Secret = *req.Secret
	}
	if req.Events != nil {
		eventsJSON, _ := json.Marshal(req.Events)
		webhook.Events = string(eventsJSON)
	}
	if req.Headers != nil {
		headersJSON, _ := json.Marshal(req.Headers)
		webhook.Headers = string(headersJSON)
	}
	if req.Status != nil {
		webhook.Status = *req.Status
	}
	if req.Description != nil {
		webhook.Description = *req.Description
	}

	if err := s.webhookRepo.Update(ctx, webhook); err != nil {
		return nil, fmt.Errorf("更新 Webhook 失败: %w", err)
	}

	logger.Info("webhook updated",
		zap.Uint64("webhook_id", id),
	)

	return s.toWebhookResponse(webhook), nil
}

// DeleteWebhook 删除 Webhook
func (s *configService) DeleteWebhook(ctx context.Context, id uint64) error {
	_, err := s.webhookRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrWebhookNotFound
		}
		return fmt.Errorf("获取 Webhook 失败: %w", err)
	}

	if err := s.webhookRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("删除 Webhook 失败: %w", err)
	}

	logger.Info("webhook deleted", zap.Uint64("webhook_id", id))

	return nil
}

// ListWebhooks 查询 Webhook 列表
func (s *configService) ListWebhooks(ctx context.Context, req *dto.ListWebhooksRequest) ([]*dto.WebhookResponse, int64, error) {
	page := req.GetDefaultPage()
	pageSize := req.GetDefaultPageSize()
	offset := (page - 1) * pageSize

	webhooks, total, err := s.webhookRepo.List(ctx, offset, pageSize, req.Status, req.Event)
	if err != nil {
		return nil, 0, fmt.Errorf("查询 Webhook 列表失败: %w", err)
	}

	responses := make([]*dto.WebhookResponse, len(webhooks))
	for i, webhook := range webhooks {
		responses[i] = s.toWebhookResponse(webhook)
	}

	return responses, total, nil
}

// ============ Webhook 日志 ============

// ListWebhookLogs 查询 Webhook 日志
func (s *configService) ListWebhookLogs(ctx context.Context, req *dto.ListWebhookLogsRequest) ([]*dto.WebhookLogResponse, int64, error) {
	page := req.GetDefaultPage()
	pageSize := req.GetDefaultPageSize()
	offset := (page - 1) * pageSize

	logs, total, err := s.webhookLogRepo.List(ctx, offset, pageSize, req.WebhookID, req.Event, req.Status)
	if err != nil {
		return nil, 0, fmt.Errorf("查询 Webhook 日志失败: %w", err)
	}

	responses := make([]*dto.WebhookLogResponse, len(logs))
	for i, log := range logs {
		responses[i] = s.toWebhookLogResponse(log)
	}

	return responses, total, nil
}

// ============ 缓存操作 ============

// InvalidateCache 清除指定配置的缓存
func (s *configService) InvalidateCache(ctx context.Context, key string) error {
	// 清除本地缓存
	s.cacheMu.Lock()
	delete(s.cache, key)
	s.cacheMu.Unlock()

	// 清除 Redis 缓存
	if redis.Client != nil {
		redis.Client.Del(ctx, "config:"+key)
	}

	return nil
}

// InvalidateAllCache 清除所有配置缓存
func (s *configService) InvalidateAllCache(ctx context.Context) error {
	// 清除本地缓存
	s.cacheMu.Lock()
	s.cache = make(map[string]*cacheEntry)
	s.cacheMu.Unlock()

	// 清除 Redis 缓存
	if redis.Client != nil {
		keys, _ := redis.Client.Keys(ctx, "config:*").Result()
		if len(keys) > 0 {
			redis.Client.Del(ctx, keys...)
		}
	}

	return nil
}

// ============ 辅助方法 ============

// setLocalCache 设置本地缓存
func (s *configService) setLocalCache(key, value string) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	s.cache[key] = &cacheEntry{
		value:     value,
		expiresAt: time.Now().Add(s.cacheTTL),
	}
}

// toConfigResponse 将配置模型转换为响应 DTO
func (s *configService) toConfigResponse(config *model.SystemConfig) *dto.ConfigResponse {
	resp := &dto.ConfigResponse{
		ID:          config.ID,
		ConfigKey:   config.ConfigKey,
		ConfigType:  config.ConfigType,
		Category:    config.Category,
		Description: config.Description,
		IsSecret:    config.IsSecret,
		UpdatedBy:   config.UpdatedBy,
		UpdatedAt:   config.UpdatedAt,
	}

	// 敏感配置不返回值
	if !config.IsSecret {
		resp.ConfigValue = config.ConfigValue
	}

	return resp
}

// toWebhookResponse 将 Webhook 模型转换为响应 DTO
func (s *configService) toWebhookResponse(webhook *model.Webhook) *dto.WebhookResponse {
	var events []string
	var headers map[string]string

	_ = json.Unmarshal([]byte(webhook.Events), &events)
	_ = json.Unmarshal([]byte(webhook.Headers), &headers)

	return &dto.WebhookResponse{
		ID:          webhook.ID,
		Name:        webhook.Name,
		URL:         webhook.URL,
		Events:      events,
		Headers:     headers,
		Status:      webhook.Status,
		Description: webhook.Description,
		CreatedBy:   webhook.CreatedBy,
		CreatedAt:   webhook.CreatedAt,
		UpdatedAt:   webhook.UpdatedAt,
	}
}

// toWebhookLogResponse 将 Webhook 日志模型转换为响应 DTO
func (s *configService) toWebhookLogResponse(log *model.WebhookLog) *dto.WebhookLogResponse {
	return &dto.WebhookLogResponse{
		ID:           log.ID,
		WebhookID:    log.WebhookID,
		Event:        log.Event,
		Payload:      log.Payload,
		ResponseCode: log.ResponseCode,
		ResponseBody: log.ResponseBody,
		Status:       log.Status,
		ErrorMessage: log.ErrorMessage,
		RetryCount:   log.RetryCount,
		CreatedAt:    log.CreatedAt,
	}
}
