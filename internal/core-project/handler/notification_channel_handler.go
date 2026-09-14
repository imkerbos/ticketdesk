// Package handler 提供项目通知渠道的 HTTP 处理器
package handler

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/kerbos/ticketdesk/internal/api/response"
	"github.com/kerbos/ticketdesk/internal/core-project/dto"
	"github.com/kerbos/ticketdesk/internal/core-project/service"
)

// NotificationChannelHandler 通知渠道处理器
type NotificationChannelHandler struct {
	channelService service.NotificationChannelService
	projectService service.ProjectService
}

// NewNotificationChannelHandler 创建通知渠道处理器实例
func NewNotificationChannelHandler(
	channelService service.NotificationChannelService,
	projectService service.ProjectService,
) *NotificationChannelHandler {
	return &NotificationChannelHandler{
		channelService: channelService,
		projectService: projectService,
	}
}

// resolveProjectID 根据项目 Key 解析项目 ID
func (h *NotificationChannelHandler) resolveProjectID(c *gin.Context) (uint64, bool) {
	key := c.Param("key")
	project, err := h.projectService.GetProject(c.Request.Context(), key)
	if err != nil {
		if errors.Is(err, service.ErrProjectNotFound) {
			response.NotFound(c, "workflow.project_not_found")
		} else {
			response.InternalError(c, "project.get_failed")
		}
		return 0, false
	}
	return project.ID, true
}

// HandleListChannels 获取项目的通知渠道列表
// @Summary 获取项目通知渠道列表
// @Description 获取指定项目的所有通知渠道
// @Tags ProjectNotificationChannel
// @Produce json
// @Param key path string true "项目 Key"
// @Success 200 {object} response.Response{data=[]dto.NotificationChannelResponse}
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/notification-channels [get]
// @Security BearerAuth
func (h *NotificationChannelHandler) HandleListChannels(c *gin.Context) {
	projectID, ok := h.resolveProjectID(c)
	if !ok {
		return
	}

	channels, err := h.channelService.ListChannels(c.Request.Context(), projectID)
	if err != nil {
		response.InternalError(c, "project.channel_query_failed")
		return
	}

	response.Success(c, channels)
}

// HandleCreateChannel 创建通知渠道
// @Summary 创建通知渠道
// @Description 为项目创建一个新的通知渠道
// @Tags ProjectNotificationChannel
// @Accept json
// @Produce json
// @Param key path string true "项目 Key"
// @Param request body dto.CreateNotificationChannelRequest true "创建通知渠道请求"
// @Success 201 {object} response.Response{data=dto.NotificationChannelResponse}
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/notification-channels [post]
// @Security BearerAuth
func (h *NotificationChannelHandler) HandleCreateChannel(c *gin.Context) {
	projectID, ok := h.resolveProjectID(c)
	if !ok {
		return
	}

	var req dto.CreateNotificationChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	userID := c.GetUint64("user_id")

	result, err := h.channelService.CreateChannel(c.Request.Context(), projectID, &req, userID)
	if err != nil {
		if errors.Is(err, service.ErrInvalidConfig) {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalError(c, "project.channel_create_failed")
		return
	}

	response.Created(c, result)
}

// HandleGetChannel 获取通知渠道详情
// @Summary 获取通知渠道详情
// @Description 根据 ID 获取通知渠道详细信息
// @Tags ProjectNotificationChannel
// @Produce json
// @Param key path string true "项目 Key"
// @Param id path int true "渠道 ID"
// @Success 200 {object} response.Response{data=dto.NotificationChannelResponse}
// @Failure 400 {object} response.ErrorResponse "无效的渠道 ID"
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/notification-channels/{id} [get]
// @Security BearerAuth
func (h *NotificationChannelHandler) HandleGetChannel(c *gin.Context) {
	_, ok := h.resolveProjectID(c)
	if !ok {
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "project.channel_invalid_id")
		return
	}

	result, err := h.channelService.GetChannel(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrChannelNotFound) {
			response.NotFound(c, "project.channel_not_found")
			return
		}
		response.InternalError(c, "project.channel_get_failed")
		return
	}

	response.Success(c, result)
}

// HandleUpdateChannel 更新通知渠道
// @Summary 更新通知渠道
// @Description 更新通知渠道信息
// @Tags ProjectNotificationChannel
// @Accept json
// @Produce json
// @Param key path string true "项目 Key"
// @Param id path int true "渠道 ID"
// @Param request body dto.UpdateNotificationChannelRequest true "更新通知渠道请求"
// @Success 200 {object} response.Response{data=dto.NotificationChannelResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/notification-channels/{id} [put]
// @Security BearerAuth
func (h *NotificationChannelHandler) HandleUpdateChannel(c *gin.Context) {
	_, ok := h.resolveProjectID(c)
	if !ok {
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "project.channel_invalid_id")
		return
	}

	var req dto.UpdateNotificationChannelRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.BadRequestValidation(c, bindErr)
		return
	}

	result, err := h.channelService.UpdateChannel(c.Request.Context(), id, &req)
	if err != nil {
		if errors.Is(err, service.ErrChannelNotFound) {
			response.NotFound(c, "project.channel_not_found")
			return
		}
		if errors.Is(err, service.ErrInvalidConfig) {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalError(c, "project.channel_update_failed")
		return
	}

	response.Success(c, result)
}

// HandleDeleteChannel 删除通知渠道
// @Summary 删除通知渠道
// @Description 删除指定的通知渠道
// @Tags ProjectNotificationChannel
// @Param key path string true "项目 Key"
// @Param id path int true "渠道 ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.ErrorResponse "无效的渠道 ID"
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/notification-channels/{id} [delete]
// @Security BearerAuth
func (h *NotificationChannelHandler) HandleDeleteChannel(c *gin.Context) {
	_, ok := h.resolveProjectID(c)
	if !ok {
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "project.channel_invalid_id")
		return
	}

	if err := h.channelService.DeleteChannel(c.Request.Context(), id); err != nil {
		if errors.Is(err, service.ErrChannelNotFound) {
			response.NotFound(c, "project.channel_not_found")
			return
		}
		response.InternalError(c, "project.channel_delete_failed")
		return
	}

	response.Success(c, nil)
}

// HandleTestChannel 测试通知渠道
// @Summary 测试通知渠道
// @Description 发送测试消息到指定通知渠道
// @Tags ProjectNotificationChannel
// @Param key path string true "项目 Key"
// @Param id path int true "渠道 ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.ErrorResponse "无效的渠道 ID 或发送失败"
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/notification-channels/{id}/test [post]
// @Security BearerAuth
func (h *NotificationChannelHandler) HandleTestChannel(c *gin.Context) {
	_, ok := h.resolveProjectID(c)
	if !ok {
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "project.channel_invalid_id")
		return
	}

	if err := h.channelService.TestChannel(c.Request.Context(), id); err != nil {
		if errors.Is(err, service.ErrChannelNotFound) {
			response.NotFound(c, "project.channel_not_found")
			return
		}
		response.BadRequest(c, response.T(c, "project.test_failed")+err.Error())
		return
	}

	response.Success(c, gin.H{"message": response.T(c, "project.test_sent")})
}
