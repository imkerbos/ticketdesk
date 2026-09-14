// Package handler 提供告警数据源 HTTP 处理器
package handler

import (
	"encoding/json"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/kerbos/ticketdesk/internal/api/response"
	"github.com/kerbos/ticketdesk/internal/integration-alert/dto"
	"github.com/kerbos/ticketdesk/internal/integration-alert/service"
	"github.com/kerbos/ticketdesk/internal/model"
	"github.com/kerbos/ticketdesk/pkg/logger"
)

// DatasourceHandler 数据源处理器
type DatasourceHandler struct {
	datasourceService service.DatasourceService
	alertService      service.AlertService
}

// NewDatasourceHandler 创建数据源处理器
func NewDatasourceHandler(datasourceService service.DatasourceService, alertService service.AlertService) *DatasourceHandler {
	return &DatasourceHandler{
		datasourceService: datasourceService,
		alertService:      alertService,
	}
}

// HandleListDatasources 获取数据源列表
// @Summary 获取告警数据源列表
// @Tags Datasource
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.Response{data=dto.DatasourceListResponse} "获取成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/alert-datasources [get]
func (h *DatasourceHandler) HandleListDatasources(c *gin.Context) {
	var req dto.DatasourceListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	result, err := h.datasourceService.List(c.Request.Context(), &req)
	if err != nil {
		logger.Error("failed to list datasources", zap.Error(err))
		response.InternalError(c, "alert.datasource_list_failed")
		return
	}

	response.Success(c, result)
}

// HandleGetDatasource 获取数据源详情
// @Summary 获取告警数据源详情
// @Tags Datasource
// @Produce json
// @Security BearerAuth
// @Param id path int true "数据源 ID"
// @Success 200 {object} response.Response{data=dto.DatasourceResponse} "获取成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Failure 404 {object} response.ErrorResponse "数据源不存在"
// @Router /api/v1/alert-datasources/{id} [get]
func (h *DatasourceHandler) HandleGetDatasource(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "alert.invalid_datasource_id")
		return
	}

	ds, err := h.datasourceService.GetByID(c.Request.Context(), id)
	if err != nil {
		logger.Error("failed to get datasource", zap.Uint64("id", id), zap.Error(err))
		response.NotFound(c, "alert.datasource_not_found")
		return
	}

	response.Success(c, ds)
}

// HandleCreateDatasource 创建数据源
// @Summary 创建告警数据源
// @Tags Datasource
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body dto.CreateDatasourceRequest true "创建数据源请求体"
// @Success 201 {object} response.Response{data=dto.DatasourceResponse} "创建成功"
// @Failure 400 {object} response.ErrorResponse "参数错误"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/alert-datasources [post]
func (h *DatasourceHandler) HandleCreateDatasource(c *gin.Context) {
	var req dto.CreateDatasourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	userID, ok := resolveUserID(c)
	if !ok {
		return
	}

	ds, err := h.datasourceService.Create(c.Request.Context(), &req, userID)
	if err != nil {
		logger.Error("failed to create datasource", zap.Error(err))
		response.InternalError(c, response.T(c, "alert.datasource_create_failed")+err.Error())
		return
	}

	response.Created(c, ds)
}

// HandleUpdateDatasource 更新数据源
// @Summary 更新告警数据源
// @Tags Datasource
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "数据源 ID"
// @Param body body dto.UpdateDatasourceRequest true "更新数据源请求体"
// @Success 200 {object} response.Response{data=dto.DatasourceResponse} "更新成功"
// @Failure 400 {object} response.ErrorResponse "参数错误"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/alert-datasources/{id} [put]
func (h *DatasourceHandler) HandleUpdateDatasource(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "alert.invalid_datasource_id")
		return
	}

	var req dto.UpdateDatasourceRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.BadRequestValidation(c, bindErr)
		return
	}

	ds, err := h.datasourceService.Update(c.Request.Context(), id, &req)
	if err != nil {
		logger.Error("failed to update datasource", zap.Uint64("id", id), zap.Error(err))
		response.InternalError(c, response.T(c, "alert.datasource_update_failed")+err.Error())
		return
	}

	response.Success(c, ds)
}

// HandleDeleteDatasource 删除数据源
// @Summary 删除告警数据源
// @Tags Datasource
// @Produce json
// @Security BearerAuth
// @Param id path int true "数据源 ID"
// @Success 200 {object} response.Response "删除成功"
// @Failure 400 {object} response.ErrorResponse "参数错误"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/alert-datasources/{id} [delete]
func (h *DatasourceHandler) HandleDeleteDatasource(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "alert.invalid_datasource_id")
		return
	}

	if err := h.datasourceService.Delete(c.Request.Context(), id); err != nil {
		logger.Error("failed to delete datasource", zap.Uint64("id", id), zap.Error(err))
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "datasource deleted"})
}

// HandleTestConnection 测试数据源连接（创建前）
// @Summary 测试数据源连接
// @Description 在保存数据源之前测试连接是否可用
// @Tags Datasource
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body dto.TestDatasourceRequest true "测试连接请求体"
// @Success 200 {object} response.Response{data=dto.TestDatasourceResponse} "测试结果"
// @Failure 400 {object} response.ErrorResponse "参数错误"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/alert-datasources/test [post]
func (h *DatasourceHandler) HandleTestConnection(c *gin.Context) {
	var req dto.TestDatasourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	result, err := h.datasourceService.TestConnection(c.Request.Context(), &req)
	if err != nil {
		logger.Error("failed to test connection", zap.Error(err))
		response.InternalError(c, "alert.test_failed")
		return
	}

	response.Success(c, result)
}

// HandleTestConnectionByID 测试已保存数据源的连接
// @Summary 测试已保存数据源连接
// @Tags Datasource
// @Produce json
// @Security BearerAuth
// @Param id path int true "数据源 ID"
// @Success 200 {object} response.Response{data=dto.TestDatasourceResponse} "测试结果"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Failure 404 {object} response.ErrorResponse "数据源不存在"
// @Router /api/v1/alert-datasources/{id}/test [post]
func (h *DatasourceHandler) HandleTestConnectionByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "alert.invalid_datasource_id")
		return
	}

	result, err := h.datasourceService.TestConnectionByID(c.Request.Context(), id)
	if err != nil {
		logger.Error("failed to test connection by id", zap.Uint64("id", id), zap.Error(err))
		response.InternalError(c, "alert.test_failed")
		return
	}

	response.Success(c, result)
}

// HandleDatasourceWebhook 通用数据源 Webhook 入口
// @Summary 数据源 Webhook 接收
// @Description 通过数据源名称路由处理 Prometheus 或夜莺告警 Webhook
// @Tags Datasource
// @Accept json
// @Produce json
// @Param name path string true "数据源名称"
// @Success 200 {object} response.Response "处理成功"
// @Failure 400 {object} response.ErrorResponse "参数错误"
// @Failure 404 {object} response.ErrorResponse "数据源不存在"
// @Router /api/v1/alerts/datasource/{name}/webhook [post]
func (h *DatasourceHandler) HandleDatasourceWebhook(c *gin.Context) {
	name := c.Param("name")

	ds, err := h.datasourceService.GetByName(c.Request.Context(), name)
	if err != nil {
		logger.Error("datasource not found for webhook", zap.String("name", name), zap.Error(err))
		response.NotFound(c, response.T(c, "alert.datasource_not_found_detail")+name)
		return
	}

	if ds.Status == 0 {
		response.BadRequest(c, "alert.datasource_disabled")
		return
	}

	switch ds.Type {
	case model.DatasourceTypePrometheus:
		var req dto.AlertWebhookRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequestValidation(c, err)
			return
		}
		if err := h.alertService.HandleWebhookWithSource(c.Request.Context(), &req, ds.Name, ds.ID); err != nil {
			logger.Error("failed to handle prometheus webhook", zap.String("source", ds.Name), zap.Error(err))
			response.InternalError(c, "alert.webhook_failed")
			return
		}

	case model.DatasourceTypeNightingale:
		body, err := c.GetRawData()
		if err != nil {
			response.BadRequest(c, response.T(c, "alert.read_body_failed")+err.Error())
			return
		}
		var events []dto.N9eAlertEvent
		if err := json.Unmarshal(body, &events); err != nil {
			var single dto.N9eAlertEvent
			if err := json.Unmarshal(body, &single); err != nil {
				response.BadRequestValidation(c, err)
				return
			}
			events = []dto.N9eAlertEvent{single}
		}
		if err := h.alertService.HandleNightingaleWebhookWithSource(c.Request.Context(), events, ds.Name, ds.ID); err != nil {
			logger.Error("failed to handle nightingale webhook", zap.String("source", ds.Name), zap.Error(err))
			response.InternalError(c, "alert.n9e_webhook_failed")
			return
		}

	default:
		response.BadRequest(c, response.T(c, "alert.unsupported_datasource")+ds.Type)
		return
	}

	response.Success(c, gin.H{"message": "webhook processed", "source": ds.Name})
}
