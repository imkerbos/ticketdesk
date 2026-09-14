// Package handler 提供告警 HTTP 处理器
package handler

import (
	"encoding/json"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/kerbos/ticketdesk/internal/api/response"
	"github.com/kerbos/ticketdesk/internal/integration-alert/dto"
	"github.com/kerbos/ticketdesk/internal/integration-alert/service"
	"github.com/kerbos/ticketdesk/pkg/logger"
)

// AlertHandler 告警处理器
type AlertHandler struct {
	alertService service.AlertService
}

// NewAlertHandler 创建告警处理器
func NewAlertHandler(alertService service.AlertService) *AlertHandler {
	return &AlertHandler{
		alertService: alertService,
	}
}

func resolveUserID(c *gin.Context) (uint64, bool) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "alert.unauthenticated")
		return 0, false
	}
	userID, ok := userIDVal.(uint64)
	if !ok {
		response.Unauthorized(c, "alert.invalid_user_ctx")
		return 0, false
	}
	return userID, true
}

// HandleWebhook 处理告警 Webhook
// @Summary 接收 Prometheus 告警 Webhook
// @Description 接收来自 Alertmanager 的告警推送，无需认证
// @Tags Alert
// @Accept json
// @Produce json
// @Param body body dto.AlertWebhookRequest true "告警 Webhook 请求体"
// @Success 200 {object} response.Response "处理成功"
// @Failure 400 {object} response.ErrorResponse "参数错误"
// @Router /api/v1/alerts/webhook [post]
func (h *AlertHandler) HandleWebhook(c *gin.Context) {
	var req dto.AlertWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	if err := h.alertService.HandleWebhook(c.Request.Context(), &req); err != nil {
		logger.Error("failed to handle webhook", zap.Error(err))
		response.InternalError(c, "alert.webhook_failed")
		return
	}

	response.Success(c, gin.H{"message": "webhook processed"})
}

// HandleNightingaleWebhook 处理夜莺告警 Webhook
// @Summary 接收夜莺告警 Webhook
// @Description 接收来自夜莺监控系统的告警推送，支持单个或数组格式，无需认证
// @Tags Alert
// @Accept json
// @Produce json
// @Param body body []dto.N9eAlertEvent true "夜莺告警事件（数组或单个对象）"
// @Success 200 {object} response.Response "处理成功"
// @Failure 400 {object} response.ErrorResponse "参数错误"
// @Router /api/v1/alerts/nightingale [post]
func (h *AlertHandler) HandleNightingaleWebhook(c *gin.Context) {
	body, err := c.GetRawData()
	if err != nil {
		response.BadRequest(c, response.T(c, "alert.read_body_failed")+err.Error())
		return
	}

	// 先尝试解析为数组
	var events []dto.N9eAlertEvent
	if err := json.Unmarshal(body, &events); err != nil {
		// 失败则尝试解析为单个对象
		var single dto.N9eAlertEvent
		if err := json.Unmarshal(body, &single); err != nil {
			response.BadRequestValidation(c, err)
			return
		}
		events = []dto.N9eAlertEvent{single}
	}

	if err := h.alertService.HandleNightingaleWebhook(c.Request.Context(), events); err != nil {
		logger.Error("failed to handle nightingale webhook", zap.Error(err))
		response.InternalError(c, "alert.n9e_webhook_failed")
		return
	}

	response.Success(c, gin.H{"message": "nightingale webhook processed"})
}

// HandleListAlerts 获取告警列表
// @Summary 获取告警列表
// @Tags Alert
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Param status query string false "告警状态 (firing/resolved)"
// @Param severity query string false "严重级别 (critical/warning/info)"
// @Param source query string false "来源"
// @Param alert_name query string false "告警名称"
// @Param issue_id query int false "关联工单 ID"
// @Param label_filters query string false "标签筛选，格式: key==value,key!=value"
// @Success 200 {object} response.Response{data=dto.AlertListResponse} "获取成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/alerts [get]
func (h *AlertHandler) HandleListAlerts(c *gin.Context) {
	var req dto.AlertListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	result, err := h.alertService.ListAlerts(c.Request.Context(), &req)
	if err != nil {
		logger.Error("failed to list alerts", zap.Error(err))
		response.InternalError(c, "alert.list_failed")
		return
	}

	response.Success(c, result)
}

// HandleGetAlertStats 获取告警统计数据
// @Summary 获取告警统计数据
// @Tags Alert
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response "获取成功（data 为 AlertStatsResponse）"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/alerts/stats [get]
func (h *AlertHandler) HandleGetAlertStats(c *gin.Context) {
	stats, err := h.alertService.GetAlertStats(c.Request.Context())
	if err != nil {
		logger.Error("failed to get alert stats", zap.Error(err))
		response.InternalError(c, "alert.stats_failed")
		return
	}

	response.Success(c, stats)
}

// HandleGetAlert 获取告警详情
// @Summary 获取告警详情
// @Tags Alert
// @Produce json
// @Security BearerAuth
// @Param id path int true "告警 ID"
// @Success 200 {object} response.Response{data=dto.AlertResponse} "获取成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Failure 404 {object} response.ErrorResponse "告警不存在"
// @Router /api/v1/alerts/{id} [get]
func (h *AlertHandler) HandleGetAlert(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "alert.invalid_id")
		return
	}

	alert, err := h.alertService.GetAlert(c.Request.Context(), id)
	if err != nil {
		logger.Error("failed to get alert", zap.Uint64("id", id), zap.Error(err))
		response.NotFound(c, "alert.not_found")
		return
	}

	response.Success(c, alert)
}

// HandleAckAlert 确认告警
// @Summary 确认告警
// @Tags Alert
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "告警 ID"
// @Param body body dto.AlertAckRequest true "确认告警请求体"
// @Success 200 {object} response.Response "确认成功"
// @Failure 400 {object} response.ErrorResponse "参数错误"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/alerts/{id}/ack [post]
func (h *AlertHandler) HandleAckAlert(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "alert.invalid_id")
		return
	}

	var req dto.AlertAckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	userID, ok := resolveUserID(c)
	if !ok {
		return
	}

	if err := h.alertService.AckAlert(c.Request.Context(), id, userID, &req); err != nil {
		logger.Error("failed to ack alert", zap.Uint64("id", id), zap.Error(err))
		response.InternalError(c, response.T(c, "alert.ack_failed")+err.Error())
		return
	}

	response.Success(c, gin.H{"message": "alert acknowledged"})
}

// HandleResolveAlert 解决告警
// @Summary 解决告警
// @Tags Alert
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "告警 ID"
// @Param body body dto.AlertResolveRequest true "解决告警请求体"
// @Success 200 {object} response.Response "解决成功"
// @Failure 400 {object} response.ErrorResponse "参数错误"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/alerts/{id}/resolve [post]
func (h *AlertHandler) HandleResolveAlert(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "alert.invalid_id")
		return
	}

	var req dto.AlertResolveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	userID, ok := resolveUserID(c)
	if !ok {
		return
	}

	if err := h.alertService.ResolveAlert(c.Request.Context(), id, userID, &req); err != nil {
		logger.Error("failed to resolve alert", zap.Uint64("id", id), zap.Error(err))
		response.InternalError(c, response.T(c, "alert.resolve_failed")+err.Error())
		return
	}

	response.Success(c, gin.H{"message": "alert resolved"})
}

// ============ 告警规则管理 ============

// HandleCreateAlertRule 创建告警规则
// @Summary 创建告警规则
// @Tags Alert
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body dto.CreateAlertRuleRequest true "创建告警规则请求体"
// @Success 201 {object} response.Response{data=dto.AlertRuleResponse} "创建成功"
// @Failure 400 {object} response.ErrorResponse "参数错误"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/alert-rules [post]
func (h *AlertHandler) HandleCreateAlertRule(c *gin.Context) {
	var req dto.CreateAlertRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	rule, err := h.alertService.CreateAlertRule(c.Request.Context(), &req)
	if err != nil {
		logger.Error("failed to create alert rule", zap.Error(err))
		response.InternalError(c, "alert.rule_create_failed")
		return
	}

	response.Created(c, rule)
}

// HandleGetAlertRule 获取告警规则详情
// @Summary 获取告警规则详情
// @Tags Alert
// @Produce json
// @Security BearerAuth
// @Param id path int true "规则 ID"
// @Success 200 {object} response.Response{data=dto.AlertRuleResponse} "获取成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Failure 404 {object} response.ErrorResponse "规则不存在"
// @Router /api/v1/alert-rules/{id} [get]
func (h *AlertHandler) HandleGetAlertRule(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "alert.invalid_rule_id")
		return
	}

	rule, err := h.alertService.GetAlertRule(c.Request.Context(), id)
	if err != nil {
		logger.Error("failed to get alert rule", zap.Uint64("id", id), zap.Error(err))
		response.NotFound(c, "alert.rule_not_found")
		return
	}

	response.Success(c, rule)
}

// HandleUpdateAlertRule 更新告警规则
// @Summary 更新告警规则
// @Tags Alert
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "规则 ID"
// @Param body body dto.UpdateAlertRuleRequest true "更新告警规则请求体"
// @Success 200 {object} response.Response{data=dto.AlertRuleResponse} "更新成功"
// @Failure 400 {object} response.ErrorResponse "参数错误"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/alert-rules/{id} [put]
func (h *AlertHandler) HandleUpdateAlertRule(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "alert.invalid_rule_id")
		return
	}

	var req dto.UpdateAlertRuleRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.BadRequestValidation(c, bindErr)
		return
	}

	rule, err := h.alertService.UpdateAlertRule(c.Request.Context(), id, &req)
	if err != nil {
		logger.Error("failed to update alert rule", zap.Uint64("id", id), zap.Error(err))
		response.InternalError(c, "alert.rule_update_failed")
		return
	}

	response.Success(c, rule)
}

// HandleDeleteAlertRule 删除告警规则
// @Summary 删除告警规则
// @Tags Alert
// @Produce json
// @Security BearerAuth
// @Param id path int true "规则 ID"
// @Success 200 {object} response.Response "删除成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/alert-rules/{id} [delete]
func (h *AlertHandler) HandleDeleteAlertRule(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "alert.invalid_rule_id")
		return
	}

	if err := h.alertService.DeleteAlertRule(c.Request.Context(), id); err != nil {
		logger.Error("failed to delete alert rule", zap.Uint64("id", id), zap.Error(err))
		response.InternalError(c, "alert.rule_delete_failed")
		return
	}

	response.Success(c, gin.H{"message": "alert rule deleted"})
}

// HandleListAlertRules 获取告警规则列表
// @Summary 获取告警规则列表
// @Tags Alert
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Response{data=dto.AlertRuleListResponse} "获取成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/alert-rules [get]
func (h *AlertHandler) HandleListAlertRules(c *gin.Context) {
	var req dto.AlertRuleListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	result, err := h.alertService.ListAlertRules(c.Request.Context(), &req)
	if err != nil {
		logger.Error("failed to list alert rules", zap.Error(err))
		response.InternalError(c, "alert.rule_list_failed")
		return
	}

	response.Success(c, result)
}

// HandleGetAlertLabelKeys 获取所有告警标签 key 列表
// @Summary 获取告警标签键列表
// @Tags Alert
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response{data=[]string} "获取成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/alerts/label-keys [get]
func (h *AlertHandler) HandleGetAlertLabelKeys(c *gin.Context) {
	keys, err := h.alertService.GetAlertLabelKeys(c.Request.Context())
	if err != nil {
		logger.Error("failed to get alert label keys", zap.Error(err))
		response.InternalError(c, "alert.labels_failed")
		return
	}

	response.Success(c, keys)
}

// HandleGroupAlerts 按标签分组统计告警
// @Summary 按标签分组统计告警
// @Tags Alert
// @Produce json
// @Security BearerAuth
// @Param group_by query string false "分组字段（标签 key）"
// @Success 200 {object} response.Response{data=[]dto.AlertGroupResponse} "获取成功"
// @Failure 400 {object} response.ErrorResponse "请求参数错误"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/alerts/group [get]
func (h *AlertHandler) HandleGroupAlerts(c *gin.Context) {
	var req dto.AlertGroupRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	result, err := h.alertService.GroupAlerts(c.Request.Context(), &req)
	if err != nil {
		logger.Error("failed to group alerts", zap.Error(err))
		response.InternalError(c, "alert.group_failed")
		return
	}

	response.Success(c, result)
}

// ============ 告警静默管理 ============

// HandleCreateAlertSilence 创建告警静默
// @Summary 创建告警静默
// @Tags Alert
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body dto.CreateAlertSilenceRequest true "创建静默请求体"
// @Success 201 {object} response.Response{data=dto.AlertSilenceResponse} "创建成功"
// @Failure 400 {object} response.ErrorResponse "参数错误"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/alert-silences [post]
func (h *AlertHandler) HandleCreateAlertSilence(c *gin.Context) {
	var req dto.CreateAlertSilenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	userID, ok := resolveUserID(c)
	if !ok {
		return
	}

	silence, err := h.alertService.CreateAlertSilence(c.Request.Context(), &req, userID)
	if err != nil {
		logger.Error("failed to create alert silence", zap.Error(err))
		response.InternalError(c, "alert.silence_create_failed")
		return
	}

	response.Created(c, silence)
}

// HandleGetAlertSilence 获取告警静默详情
// @Summary 获取告警静默详情
// @Tags Alert
// @Produce json
// @Security BearerAuth
// @Param id path int true "静默 ID"
// @Success 200 {object} response.Response{data=dto.AlertSilenceResponse} "获取成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Failure 404 {object} response.ErrorResponse "静默不存在"
// @Router /api/v1/alert-silences/{id} [get]
func (h *AlertHandler) HandleGetAlertSilence(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "alert.invalid_silence_id")
		return
	}

	silence, err := h.alertService.GetAlertSilence(c.Request.Context(), id)
	if err != nil {
		logger.Error("failed to get alert silence", zap.Uint64("id", id), zap.Error(err))
		response.NotFound(c, "alert.silence_not_found")
		return
	}

	response.Success(c, silence)
}

// HandleUpdateAlertSilence 更新告警静默
// @Summary 更新告警静默
// @Tags Alert
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "静默 ID"
// @Param body body dto.UpdateAlertSilenceRequest true "更新静默请求体"
// @Success 200 {object} response.Response{data=dto.AlertSilenceResponse} "更新成功"
// @Failure 400 {object} response.ErrorResponse "参数错误"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/alert-silences/{id} [put]
func (h *AlertHandler) HandleUpdateAlertSilence(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "alert.invalid_silence_id")
		return
	}

	var req dto.UpdateAlertSilenceRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.BadRequestValidation(c, bindErr)
		return
	}

	silence, err := h.alertService.UpdateAlertSilence(c.Request.Context(), id, &req)
	if err != nil {
		logger.Error("failed to update alert silence", zap.Uint64("id", id), zap.Error(err))
		response.InternalError(c, "alert.silence_update_failed")
		return
	}

	response.Success(c, silence)
}

// HandleDeleteAlertSilence 删除告警静默
// @Summary 删除告警静默
// @Tags Alert
// @Produce json
// @Security BearerAuth
// @Param id path int true "静默 ID"
// @Success 200 {object} response.Response "删除成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/alert-silences/{id} [delete]
func (h *AlertHandler) HandleDeleteAlertSilence(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "alert.invalid_silence_id")
		return
	}

	if err := h.alertService.DeleteAlertSilence(c.Request.Context(), id); err != nil {
		logger.Error("failed to delete alert silence", zap.Uint64("id", id), zap.Error(err))
		response.InternalError(c, "alert.silence_delete_failed")
		return
	}

	response.Success(c, gin.H{"message": "alert silence deleted"})
}

// HandleCancelAlertSilence 取消告警静默
// @Summary 取消告警静默
// @Tags Alert
// @Produce json
// @Security BearerAuth
// @Param id path int true "静默 ID"
// @Success 200 {object} response.Response "取消成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/alert-silences/{id}/cancel [post]
func (h *AlertHandler) HandleCancelAlertSilence(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "alert.invalid_silence_id")
		return
	}

	if err := h.alertService.CancelAlertSilence(c.Request.Context(), id); err != nil {
		logger.Error("failed to cancel alert silence", zap.Uint64("id", id), zap.Error(err))
		response.InternalError(c, "alert.silence_cancel_failed")
		return
	}

	response.Success(c, gin.H{"message": "alert silence canceled"})
}

// HandleListAlertSilences 获取告警静默列表
// @Summary 获取告警静默列表
// @Tags Alert
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Response{data=dto.AlertSilenceListResponse} "获取成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/alert-silences [get]
func (h *AlertHandler) HandleListAlertSilences(c *gin.Context) {
	var req dto.AlertSilenceListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	result, err := h.alertService.ListAlertSilences(c.Request.Context(), &req)
	if err != nil {
		logger.Error("failed to list alert silences", zap.Error(err))
		response.InternalError(c, "alert.silence_list_failed")
		return
	}

	response.Success(c, result)
}
