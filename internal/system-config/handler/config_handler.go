// Package handler 提供系统配置 HTTP 处理层
package handler

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kerbos/ticketdesk/internal/api/response"
	"github.com/kerbos/ticketdesk/internal/notification/lark"
	"github.com/kerbos/ticketdesk/internal/notification/telegram"
	"github.com/kerbos/ticketdesk/internal/system-config/dto"
	"github.com/kerbos/ticketdesk/internal/system-config/service"
	"github.com/kerbos/ticketdesk/pkg/storage"
)

// ConfigHandler 系统配置处理器
type ConfigHandler struct {
	configService   service.ConfigService
	larkService     lark.LarkService
	telegramService telegram.TelegramService
	storage         storage.Storage
}

// NewConfigHandler 创建系统配置处理器
func NewConfigHandler(configService service.ConfigService) *ConfigHandler {
	return &ConfigHandler{configService: configService}
}

// SetLarkService 设置飞书服务（用于测试发送）
func (h *ConfigHandler) SetLarkService(larkService lark.LarkService) {
	h.larkService = larkService
}

// SetTelegramService 设置 Telegram 服务（用于测试发送）
func (h *ConfigHandler) SetTelegramService(telegramService telegram.TelegramService) {
	h.telegramService = telegramService
}

// SetLocalStorage 设置本地存储（用于品牌资源上传）
func (h *ConfigHandler) SetStorage(st storage.Storage) {
	h.storage = st
}

// publicConfigWhitelist 允许普通用户读取的配置 key 白名单
var publicConfigWhitelist = map[string]bool{
	"worklog.work_types": true,
}

// HandleGetPublicConfig 获取公共配置（普通用户可访问，受白名单限制）
// @Summary 获取公共配置
// @Description 普通认证用户可访问，受白名单限制
// @Tags Config
// @Produce json
// @Security BearerAuth
// @Param key path string true "配置键"
// @Success 200 {object} response.Response{data=dto.ConfigResponse} "获取成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Failure 403 {object} response.ErrorResponse "无权访问该配置"
// @Failure 404 {object} response.ErrorResponse "配置不存在"
// @Router /api/v1/system/configs/public/{key} [get]
func (h *ConfigHandler) HandleGetPublicConfig(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		response.BadRequest(c, "system.config_key_empty")
		return
	}

	if !publicConfigWhitelist[key] {
		response.Forbidden(c, "system.config_no_access")
		return
	}

	config, err := h.configService.GetConfig(c.Request.Context(), key)
	if err != nil {
		if errors.Is(err, service.ErrConfigNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, config)
}

// ============ 配置管理 ============

// HandleGetConfig 获取单个配置
// @Summary 获取单个系统配置
// @Tags Config
// @Produce json
// @Security BearerAuth
// @Param key path string true "配置键"
// @Success 200 {object} response.Response{data=dto.ConfigResponse} "获取成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Failure 404 {object} response.ErrorResponse "配置不存在"
// @Router /api/v1/system/configs/{key} [get]
func (h *ConfigHandler) HandleGetConfig(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		response.BadRequest(c, "system.config_key_empty")
		return
	}

	config, err := h.configService.GetConfig(c.Request.Context(), key)
	if err != nil {
		if errors.Is(err, service.ErrConfigNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, config)
}

// HandleGetConfigsByCategory 获取分类下的所有配置
// @Summary 按分类获取系统配置
// @Tags Config
// @Produce json
// @Security BearerAuth
// @Param category query string true "配置分类"
// @Success 200 {object} response.Response{data=[]dto.ConfigResponse} "获取成功"
// @Failure 400 {object} response.ErrorResponse "参数错误"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/system/configs/category [get]
func (h *ConfigHandler) HandleGetConfigsByCategory(c *gin.Context) {
	category := c.Query("category")
	if category == "" {
		response.BadRequest(c, "system.category_empty")
		return
	}

	configs, err := h.configService.GetConfigsByCategory(c.Request.Context(), category)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, configs)
}

// HandleGetAllConfigs 获取所有配置
// @Summary 获取所有系统配置
// @Tags Config
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response{data=[]dto.ConfigResponse} "获取成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/system/configs [get]
func (h *ConfigHandler) HandleGetAllConfigs(c *gin.Context) {
	configs, err := h.configService.GetAllConfigs(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, configs)
}

// HandleUpdateConfig 更新单个配置
// @Summary 更新单个系统配置
// @Tags Config
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param key path string true "配置键"
// @Param body body dto.UpdateConfigRequest true "更新配置请求体"
// @Success 200 {object} response.Response "更新成功"
// @Failure 400 {object} response.ErrorResponse "参数错误"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/system/configs/{key} [put]
func (h *ConfigHandler) HandleUpdateConfig(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		response.BadRequest(c, "system.config_key_empty")
		return
	}

	var req dto.UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	userID := c.GetUint64("user_id")
	if err := h.configService.UpdateConfig(c.Request.Context(), key, req.ConfigValue, userID); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, nil)
}

// HandleBatchUpdateConfigs 批量更新配置
// @Summary 批量更新系统配置
// @Tags Config
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body dto.BatchUpdateConfigRequest true "批量更新配置请求体"
// @Success 200 {object} response.Response "更新成功"
// @Failure 400 {object} response.ErrorResponse "参数错误"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/system/configs [put]
func (h *ConfigHandler) HandleBatchUpdateConfigs(c *gin.Context) {
	var req dto.BatchUpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	userID := c.GetUint64("user_id")
	if err := h.configService.BatchUpdateConfigs(c.Request.Context(), req.Configs, userID); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, nil)
}

// ============ 邮件配置 ============

// HandleGetEmailConfig 获取邮件配置
// @Summary 获取邮件配置
// @Tags Config
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response{data=dto.EmailConfig} "获取成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/system/email [get]
func (h *ConfigHandler) HandleGetEmailConfig(c *gin.Context) {
	config, err := h.configService.GetEmailConfig(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, config)
}

// HandleUpdateEmailConfig 更新邮件配置
// @Summary 更新邮件配置
// @Tags Config
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body dto.UpdateEmailConfigRequest true "更新邮件配置请求体"
// @Success 200 {object} response.Response "更新成功"
// @Failure 400 {object} response.ErrorResponse "参数错误"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/system/email [put]
func (h *ConfigHandler) HandleUpdateEmailConfig(c *gin.Context) {
	var req dto.UpdateEmailConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	userID := c.GetUint64("user_id")
	if err := h.configService.UpdateEmailConfig(c.Request.Context(), &req, userID); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, nil)
}

// ============ 安全配置 ============

// HandleGetSecurityConfig 获取安全配置
// @Summary 获取安全配置
// @Tags Config
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response{data=dto.SecurityConfig} "获取成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/system/security [get]
func (h *ConfigHandler) HandleGetSecurityConfig(c *gin.Context) {
	config, err := h.configService.GetSecurityConfig(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, config)
}

// HandleUpdateSecurityConfig 更新安全配置
// @Summary 更新安全配置
// @Tags Config
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body dto.UpdateSecurityConfigRequest true "更新安全配置请求体"
// @Success 200 {object} response.Response "更新成功"
// @Failure 400 {object} response.ErrorResponse "参数错误"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/system/security [put]
func (h *ConfigHandler) HandleUpdateSecurityConfig(c *gin.Context) {
	var req dto.UpdateSecurityConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	userID := c.GetUint64("user_id")
	if err := h.configService.UpdateSecurityConfig(c.Request.Context(), &req, userID); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, nil)
}

// ============ 限流配置 ============

// HandleGetRateLimitConfig 获取限流配置
// @Summary 获取限流配置
// @Tags Config
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response{data=dto.RateLimitConfig} "获取成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/system/ratelimit [get]
func (h *ConfigHandler) HandleGetRateLimitConfig(c *gin.Context) {
	config, err := h.configService.GetRateLimitConfig(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, config)
}

// HandleUpdateRateLimitConfig 更新限流配置
// @Summary 更新限流配置
// @Tags Config
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body dto.UpdateRateLimitConfigRequest true "更新限流配置请求体"
// @Success 200 {object} response.Response "更新成功"
// @Failure 400 {object} response.ErrorResponse "参数错误"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/system/ratelimit [put]
func (h *ConfigHandler) HandleUpdateRateLimitConfig(c *gin.Context) {
	var req dto.UpdateRateLimitConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	userID := c.GetUint64("user_id")
	if err := h.configService.UpdateRateLimitConfig(c.Request.Context(), &req, userID); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, nil)
}

// ============ Webhook 管理 ============

// HandleCreateWebhook 创建 Webhook
// @Summary 创建 Webhook
// @Tags Config
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body dto.CreateWebhookRequest true "创建 Webhook 请求体"
// @Success 201 {object} response.Response{data=dto.WebhookResponse} "创建成功"
// @Failure 400 {object} response.ErrorResponse "参数错误"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/system/webhooks [post]
func (h *ConfigHandler) HandleCreateWebhook(c *gin.Context) {
	var req dto.CreateWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	userID := c.GetUint64("user_id")
	webhook, err := h.configService.CreateWebhook(c.Request.Context(), &req, userID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Created(c, webhook)
}

// HandleGetWebhook 获取 Webhook
// @Summary 获取 Webhook 详情
// @Tags Config
// @Produce json
// @Security BearerAuth
// @Param id path int true "Webhook ID"
// @Success 200 {object} response.Response{data=dto.WebhookResponse} "获取成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Failure 404 {object} response.ErrorResponse "Webhook 不存在"
// @Router /api/v1/system/webhooks/{id} [get]
func (h *ConfigHandler) HandleGetWebhook(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "system.invalid_webhook_id")
		return
	}

	webhook, err := h.configService.GetWebhook(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrWebhookNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, webhook)
}

// HandleUpdateWebhook 更新 Webhook
// @Summary 更新 Webhook
// @Tags Config
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Webhook ID"
// @Param body body dto.UpdateWebhookRequest true "更新 Webhook 请求体"
// @Success 200 {object} response.Response{data=dto.WebhookResponse} "更新成功"
// @Failure 400 {object} response.ErrorResponse "参数错误"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Failure 404 {object} response.ErrorResponse "Webhook 不存在"
// @Router /api/v1/system/webhooks/{id} [put]
func (h *ConfigHandler) HandleUpdateWebhook(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "system.invalid_webhook_id")
		return
	}

	var req dto.UpdateWebhookRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.BadRequestValidation(c, bindErr)
		return
	}

	webhook, err := h.configService.UpdateWebhook(c.Request.Context(), id, &req)
	if err != nil {
		if errors.Is(err, service.ErrWebhookNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, webhook)
}

// HandleDeleteWebhook 删除 Webhook
// @Summary 删除 Webhook
// @Tags Config
// @Produce json
// @Security BearerAuth
// @Param id path int true "Webhook ID"
// @Success 200 {object} response.Response "删除成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Failure 404 {object} response.ErrorResponse "Webhook 不存在"
// @Router /api/v1/system/webhooks/{id} [delete]
func (h *ConfigHandler) HandleDeleteWebhook(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "system.invalid_webhook_id")
		return
	}

	if err := h.configService.DeleteWebhook(c.Request.Context(), id); err != nil {
		if errors.Is(err, service.ErrWebhookNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, nil)
}

// HandleListWebhooks 查询 Webhook 列表
// @Summary 获取 Webhook 列表
// @Tags Config
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Response{data=[]dto.WebhookResponse} "获取成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/system/webhooks [get]
func (h *ConfigHandler) HandleListWebhooks(c *gin.Context) {
	var req dto.ListWebhooksRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	webhooks, total, err := h.configService.ListWebhooks(c.Request.Context(), &req)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.SuccessWithPage(c, webhooks, total, req.GetDefaultPage(), req.GetDefaultPageSize())
}

// ============ Webhook 日志 ============

// HandleListWebhookLogs 查询 Webhook 日志
// @Summary 获取 Webhook 日志
// @Tags Config
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Response{data=[]dto.WebhookLogResponse} "获取成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/system/webhook-logs [get]
func (h *ConfigHandler) HandleListWebhookLogs(c *gin.Context) {
	var req dto.ListWebhookLogsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	logs, total, err := h.configService.ListWebhookLogs(c.Request.Context(), &req)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.SuccessWithPage(c, logs, total, req.GetDefaultPage(), req.GetDefaultPageSize())
}

// ============ 飞书配置 ============

// HandleGetLarkConfig 获取飞书配置
// @Summary 获取飞书通知配置
// @Tags Config
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response{data=dto.LarkConfig} "获取成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/system/lark [get]
func (h *ConfigHandler) HandleGetLarkConfig(c *gin.Context) {
	config, err := h.configService.GetLarkConfig(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, config)
}

// HandleUpdateLarkConfig 更新飞书配置
// @Summary 更新飞书通知配置
// @Tags Config
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body dto.UpdateLarkConfigRequest true "更新飞书配置请求体"
// @Success 200 {object} response.Response "更新成功"
// @Failure 400 {object} response.ErrorResponse "参数错误"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/system/lark [put]
func (h *ConfigHandler) HandleUpdateLarkConfig(c *gin.Context) {
	var req dto.UpdateLarkConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	userID := c.GetUint64("user_id")
	if err := h.configService.UpdateLarkConfig(c.Request.Context(), &req, userID); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, nil)
}

// HandleTestLark 测试飞书通知
// @Summary 测试飞书通知发送
// @Tags Config
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response "发送成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Failure 500 {object} response.ErrorResponse "发送失败"
// @Router /api/v1/system/lark/test [post]
func (h *ConfigHandler) HandleTestLark(c *gin.Context) {
	if h.larkService == nil {
		response.InternalError(c, "system.lark_uninitialized")
		return
	}

	if err := h.larkService.SendTestMessage(c.Request.Context()); err != nil {
		response.InternalError(c, response.T(c, "system.lark_test_failed")+err.Error())
		return
	}

	response.Success(c, nil)
}

// ============ Telegram 配置 ============

// HandleGetTelegramConfig 获取 Telegram 配置
// @Summary 获取 Telegram 通知配置
// @Tags Config
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response{data=dto.TelegramConfig} "获取成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/system/telegram [get]
func (h *ConfigHandler) HandleGetTelegramConfig(c *gin.Context) {
	config, err := h.configService.GetTelegramConfig(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, config)
}

// HandleUpdateTelegramConfig 更新 Telegram 配置
// @Summary 更新 Telegram 通知配置
// @Tags Config
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body dto.UpdateTelegramConfigRequest true "更新 Telegram 配置请求体"
// @Success 200 {object} response.Response "更新成功"
// @Failure 400 {object} response.ErrorResponse "参数错误"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/system/telegram [put]
func (h *ConfigHandler) HandleUpdateTelegramConfig(c *gin.Context) {
	var req dto.UpdateTelegramConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	userID := c.GetUint64("user_id")
	if err := h.configService.UpdateTelegramConfig(c.Request.Context(), &req, userID); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, nil)
}

// HandleTestTelegram 测试 Telegram 通知
// @Summary 测试 Telegram 通知发送
// @Tags Config
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response "发送成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Failure 500 {object} response.ErrorResponse "发送失败"
// @Router /api/v1/system/telegram/test [post]
func (h *ConfigHandler) HandleTestTelegram(c *gin.Context) {
	if h.telegramService == nil {
		response.InternalError(c, "system.telegram_uninitialized")
		return
	}

	if err := h.telegramService.SendTestMessage(c.Request.Context()); err != nil {
		response.InternalError(c, response.T(c, "system.telegram_test_failed")+err.Error())
		return
	}

	response.Success(c, nil)
}

// ============ SSO 配置 ============

// HandleGetSSOConfig 获取 SSO 配置
// @Summary 获取 SSO 配置
// @Tags Config
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response{data=dto.SSOConfig} "获取成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/system/sso [get]
func (h *ConfigHandler) HandleGetSSOConfig(c *gin.Context) {
	config, err := h.configService.GetSSOConfig(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, config)
}

// HandleUpdateSSOConfig 更新 SSO 配置
// @Summary 更新 SSO 配置
// @Tags Config
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body dto.UpdateSSOConfigRequest true "更新 SSO 配置请求体"
// @Success 200 {object} response.Response "更新成功"
// @Failure 400 {object} response.ErrorResponse "参数错误"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/system/sso [put]
func (h *ConfigHandler) HandleUpdateSSOConfig(c *gin.Context) {
	var req dto.UpdateSSOConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	userID := c.GetUint64("user_id")
	if err := h.configService.UpdateSSOConfig(c.Request.Context(), &req, userID); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, nil)
}

// ============ 品牌配置 ============

// HandleGetBrandConfig 获取品牌配置（公开接口）
// @Summary 获取品牌配置
// @Description 无需认证，用于前端加载自定义 Logo、Favicon 等品牌信息
// @Tags Config
// @Produce json
// @Success 200 {object} response.Response{data=dto.BrandConfig} "获取成功"
// @Router /api/v1/brand [get]
func (h *ConfigHandler) HandleGetBrandConfig(c *gin.Context) {
	config, err := h.configService.GetBrandConfig(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, config)
}

// HandleUpdateBrandConfig 更新品牌配置
// @Summary 更新品牌配置
// @Tags Config
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body dto.UpdateBrandConfigRequest true "更新品牌配置请求体"
// @Success 200 {object} response.Response "更新成功"
// @Failure 400 {object} response.ErrorResponse "参数错误"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/system/brand [put]
func (h *ConfigHandler) HandleUpdateBrandConfig(c *gin.Context) {
	var req dto.UpdateBrandConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	userID := c.GetUint64("user_id")
	if err := h.configService.UpdateBrandConfig(c.Request.Context(), &req, userID); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, nil)
}

// brandAssetAllowedExts 品牌资源允许的文件扩展名
var brandAssetAllowedExts = map[string]bool{
	".svg":  true,
	".png":  true,
	".ico":  true,
	".jpg":  true,
	".jpeg": true,
	".webp": true,
}

// HandleUploadBrandAsset 上传品牌资源（Logo/Favicon）
// @Summary 上传品牌资源
// @Description 上传 Logo 或 Favicon，最大 2MB，支持 SVG/PNG/ICO/JPG/WEBP
// @Tags Config
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param type formData string true "资源类型（logo 或 favicon）"
// @Param file formData file true "上传文件"
// @Success 200 {object} response.Response "上传成功（data 包含 url 字段）"
// @Failure 400 {object} response.ErrorResponse "参数错误"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/system/brand/upload [post]
func (h *ConfigHandler) HandleUploadBrandAsset(c *gin.Context) {
	if h.storage == nil {
		response.InternalError(c, "system.storage_uninitialized")
		return
	}

	assetType := c.PostForm("type")
	if assetType != "logo" && assetType != "favicon" {
		response.BadRequest(c, "system.asset_type_invalid")
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "system.choose_file")
		return
	}

	// 检查文件大小（2MB）
	if file.Size > 2*1024*1024 {
		response.BadRequest(c, "system.file_too_large_2mb")
		return
	}

	// 检查文件扩展名
	ext := filepath.Ext(file.Filename)
	if !brandAssetAllowedExts[ext] {
		response.BadRequest(c, "system.unsupported_image")
		return
	}

	// 生成唯一文件名；JoinKey 保证结果落在 brand/ 前缀之下
	filename, err := storage.JoinKey(storage.PrefixBrand, fmt.Sprintf("%s_%d%s", assetType, time.Now().UnixMilli(), ext))
	if err != nil {
		response.BadRequest(c, "system.file_name_invalid")
		return
	}

	src, err := file.Open()
	if err != nil {
		response.InternalError(c, "system.open_file_failed")
		return
	}
	defer src.Close()

	if err := h.storage.Save(c.Request.Context(), filename, src, file.Size, file.Header.Get("Content-Type")); err != nil {
		response.InternalError(c, response.T(c, "system.save_file_failed")+err.Error())
		return
	}
	relPath := filename

	// 构建访问 URL
	assetURL := "/api/v1/brand/assets/" + relPath

	// 更新对应的配置键
	userID := c.GetUint64("user_id")
	configKey := service.KeyBrandLogoURL
	if assetType == "favicon" {
		configKey = service.KeyBrandFaviconURL
	}
	if err := h.configService.UpdateConfig(c.Request.Context(), configKey, assetURL, userID); err != nil {
		response.InternalError(c, response.T(c, "system.config_update_failed")+err.Error())
		return
	}

	response.Success(c, gin.H{"url": assetURL})
}
