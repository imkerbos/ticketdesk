// Package handler 提供站内通知 HTTP 处理层
package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/kerbos/ticketdesk/internal/api/response"
	"github.com/kerbos/ticketdesk/internal/notification-inbox/dto"
	"github.com/kerbos/ticketdesk/internal/notification-inbox/service"
)

// NotificationHandler 通知 HTTP 处理器
type NotificationHandler struct {
	svc service.NotificationService
}

// NewNotificationHandler 创建通知处理器
func NewNotificationHandler(svc service.NotificationService) *NotificationHandler {
	return &NotificationHandler{svc: svc}
}

func resolveUserID(c *gin.Context) (uint64, bool) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "inbox.not_logged_in")
		return 0, false
	}
	userID, ok := userIDVal.(uint64)
	if !ok {
		response.Unauthorized(c, "inbox.user_type_error")
		return 0, false
	}
	return userID, true
}

// HandleListNotifications 获取通知列表
// @Summary 获取站内通知列表
// @Tags Notification
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Param unread query bool false "只看未读"
// @Success 200 {object} response.Response{data=[]dto.NotificationResponse} "获取成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/notifications [get]
func (h *NotificationHandler) HandleListNotifications(c *gin.Context) {
	userID, ok := resolveUserID(c)
	if !ok {
		return
	}

	var req dto.ListNotificationsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "inbox.bad_request")
		return
	}

	notifications, total, err := h.svc.ListNotifications(c.Request.Context(), userID, &req)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.SuccessWithPage(c, notifications, total, req.GetDefaultPage(), req.GetDefaultPageSize())
}

// HandleGetUnreadCount 获取未读通知数量
// @Summary 获取未读通知数量
// @Tags Notification
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response{data=dto.UnreadCountResponse} "获取成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/notifications/unread-count [get]
func (h *NotificationHandler) HandleGetUnreadCount(c *gin.Context) {
	userID, ok := resolveUserID(c)
	if !ok {
		return
	}

	count, err := h.svc.GetUnreadCount(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, dto.UnreadCountResponse{Count: count})
}

// HandleMarkAsRead 标记通知为已读
// @Summary 标记通知为已读
// @Tags Notification
// @Produce json
// @Security BearerAuth
// @Param id path int true "通知 ID"
// @Success 200 {object} response.Response "操作成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/notifications/{id}/read [put]
func (h *NotificationHandler) HandleMarkAsRead(c *gin.Context) {
	userID, ok := resolveUserID(c)
	if !ok {
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "inbox.invalid_id")
		return
	}

	if err := h.svc.MarkAsRead(c.Request.Context(), id, userID); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, nil)
}

// HandleMarkAllAsRead 全部标记为已读
// @Summary 全部通知标记为已读
// @Tags Notification
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response "操作成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/notifications/read-all [put]
func (h *NotificationHandler) HandleMarkAllAsRead(c *gin.Context) {
	userID, ok := resolveUserID(c)
	if !ok {
		return
	}

	if err := h.svc.MarkAllAsRead(c.Request.Context(), userID); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, nil)
}

// HandleDeleteNotification 删除通知
// @Summary 删除通知
// @Tags Notification
// @Produce json
// @Security BearerAuth
// @Param id path int true "通知 ID"
// @Success 200 {object} response.Response "删除成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/notifications/{id} [delete]
func (h *NotificationHandler) HandleDeleteNotification(c *gin.Context) {
	userID, ok := resolveUserID(c)
	if !ok {
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "inbox.invalid_id")
		return
	}

	if err := h.svc.DeleteNotification(c.Request.Context(), id, userID); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, nil)
}
