// Package handler 提供活动日志 HTTP 处理层
package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/kerbos/ticketdesk/internal/activity/dto"
	"github.com/kerbos/ticketdesk/internal/activity/service"
	"github.com/kerbos/ticketdesk/internal/api/response"
)

// ActivityHandler 活动日志处理器
type ActivityHandler struct {
	activityService service.ActivityService
}

// NewActivityHandler 创建活动日志处理器实例
func NewActivityHandler(activityService service.ActivityService) *ActivityHandler {
	return &ActivityHandler{
		activityService: activityService,
	}
}

// HandleListActivities 处理活动列表查询
// @Summary 查询活动列表
// @Description 分页查询活动日志列表
// @Tags Activity
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Param entity_type query string false "实体类型"
// @Param entity_key query string false "实体标识"
// @Param entity_id query int false "实体 ID"
// @Param user_id query int false "用户 ID"
// @Success 200 {object} response.Response{data=response.PageResponse{items=[]dto.ActivityResponse}}
// @Router /api/v1/activities [get]
// @Security BearerAuth
func (h *ActivityHandler) HandleListActivities(c *gin.Context) {
	var req dto.ListActivitiesRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	activities, total, err := h.activityService.ListActivities(c.Request.Context(), &req)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.SuccessWithPage(c, activities, total, req.GetDefaultPage(), req.GetDefaultPageSize())
}

// HandleGetRecentActivities 处理获取最近活动
// @Summary 获取最近活动
// @Description 获取最近的活动日志
// @Tags Activity
// @Accept json
// @Produce json
// @Param limit query int false "数量限制" default(10)
// @Success 200 {object} response.Response{data=[]dto.ActivityResponse}
// @Router /api/v1/activities/recent [get]
// @Security BearerAuth
func (h *ActivityHandler) HandleGetRecentActivities(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "10")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 10
	}

	activities, err := h.activityService.GetRecentActivities(c.Request.Context(), limit)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, activities)
}
