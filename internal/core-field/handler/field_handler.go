// Package handler 提供字段HTTP处理器
package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/kerbos/ticketdesk/internal/api/response"
	"github.com/kerbos/ticketdesk/internal/core-field/dto"
	"github.com/kerbos/ticketdesk/internal/core-field/service"
)

// FieldHandler 字段处理器
type FieldHandler struct {
	fieldService service.FieldService
}

// NewFieldHandler 创建字段处理器实例
func NewFieldHandler(fieldService service.FieldService) *FieldHandler {
	return &FieldHandler{
		fieldService: fieldService,
	}
}

// ============ 字段定义 ============

// HandleCreateField 创建自定义字段
// @Summary 创建自定义字段
// @Description 为项目创建自定义字段
// @Tags Field
// @Accept json
// @Produce json
// @Param key path string true "项目Key"
// @Param request body dto.CreateFieldRequest true "创建字段请求"
// @Success 201 {object} response.Response{data=dto.FieldDefinitionResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Failure 403 {object} response.ErrorResponse "需项目管理员"
// @Router /api/v1/projects/{key}/fields [post]
// @Security BearerAuth
func (h *FieldHandler) HandleCreateField(c *gin.Context) {
	projectKey := c.Param("key")

	var req dto.CreateFieldRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	result, err := h.fieldService.CreateField(c.Request.Context(), projectKey, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	response.Created(c, result)
}

// HandleUpdateField 更新字段定义
// @Summary 更新字段定义
// @Description 更新项目的字段定义
// @Tags Field
// @Accept json
// @Produce json
// @Param key path string true "项目Key"
// @Param id path int true "字段ID"
// @Param request body dto.UpdateFieldRequest true "更新字段请求"
// @Success 200 {object} response.Response{data=dto.FieldDefinitionResponse}
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/fields/{id} [put]
// @Security BearerAuth
func (h *FieldHandler) HandleUpdateField(c *gin.Context) {
	projectKey := c.Param("key")
	fieldID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "field.invalid_id")
		return
	}

	var req dto.UpdateFieldRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.BadRequest(c, bindErr.Error())
		return
	}

	result, err := h.fieldService.UpdateField(c.Request.Context(), projectKey, fieldID, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	response.Success(c, result)
}

// HandleDeleteField 删除字段定义
// @Summary 删除字段定义
// @Description 删除项目的自定义字段
// @Tags Field
// @Produce json
// @Param key path string true "项目Key"
// @Param id path int true "字段ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse "不能删除系统字段"
// @Router /api/v1/projects/{key}/fields/{id} [delete]
// @Security BearerAuth
func (h *FieldHandler) HandleDeleteField(c *gin.Context) {
	projectKey := c.Param("key")
	fieldID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "field.invalid_id")
		return
	}

	if err := h.fieldService.DeleteField(c.Request.Context(), projectKey, fieldID); err != nil {
		h.handleError(c, err)
		return
	}

	response.Success(c, nil)
}

// HandleListFields 获取字段列表
// @Summary 获取字段列表
// @Description 获取项目可用的所有字段（系统字段+自定义字段）
// @Tags Field
// @Produce json
// @Param key path string true "项目Key"
// @Success 200 {object} response.Response{data=[]dto.FieldDefinitionResponse}
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/fields [get]
// @Security BearerAuth
func (h *FieldHandler) HandleListFields(c *gin.Context) {
	projectKey := c.Param("key")

	result, err := h.fieldService.ListFields(c.Request.Context(), projectKey)
	if err != nil {
		h.handleError(c, err)
		return
	}

	response.Success(c, result)
}

// ============ 字段方案 ============

// HandleGetFieldScheme 获取工单类型的字段方案
// @Summary 获取字段方案
// @Description 获取工单类型的字段配置方案
// @Tags Field
// @Produce json
// @Param key path string true "项目Key"
// @Param id path int true "工单类型ID"
// @Success 200 {object} response.Response{data=[]dto.FieldSchemeResponse}
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/issue-types/{id}/field-scheme [get]
// @Security BearerAuth
func (h *FieldHandler) HandleGetFieldScheme(c *gin.Context) {
	projectKey := c.Param("key")
	issueTypeID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "field.invalid_issue_type_id")
		return
	}

	result, err := h.fieldService.GetFieldScheme(c.Request.Context(), projectKey, issueTypeID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	response.Success(c, result)
}

// HandleUpdateFieldScheme 更新工单类型的字段方案
// @Summary 更新字段方案
// @Description 更新工单类型的字段配置方案
// @Tags Field
// @Accept json
// @Produce json
// @Param key path string true "项目Key"
// @Param id path int true "工单类型ID"
// @Param request body dto.UpdateFieldSchemeRequest true "更新字段方案请求"
// @Success 200 {object} response.Response{data=[]dto.FieldSchemeResponse}
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/issue-types/{id}/field-scheme [put]
// @Security BearerAuth
func (h *FieldHandler) HandleUpdateFieldScheme(c *gin.Context) {
	projectKey := c.Param("key")
	issueTypeID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "field.invalid_issue_type_id")
		return
	}

	var req dto.UpdateFieldSchemeRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.BadRequest(c, bindErr.Error())
		return
	}

	result, err := h.fieldService.UpdateFieldScheme(c.Request.Context(), projectKey, issueTypeID, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	response.Success(c, result)
}

// ============ 版本管理 ============

// HandleCreateVersion 创建项目版本
// @Summary 创建项目版本
// @Description 为项目创建新版本
// @Tags Version
// @Accept json
// @Produce json
// @Param key path string true "项目Key"
// @Param request body dto.CreateVersionRequest true "创建版本请求"
// @Success 201 {object} response.Response{data=dto.VersionResponse}
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/versions [post]
// @Security BearerAuth
func (h *FieldHandler) HandleCreateVersion(c *gin.Context) {
	projectKey := c.Param("key")

	var req dto.CreateVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	result, err := h.fieldService.CreateVersion(c.Request.Context(), projectKey, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	response.Created(c, result)
}

// HandleUpdateVersion 更新项目版本
// @Summary 更新项目版本
// @Description 更新项目的版本信息
// @Tags Version
// @Accept json
// @Produce json
// @Param key path string true "项目Key"
// @Param id path int true "版本ID"
// @Param request body dto.UpdateVersionRequest true "更新版本请求"
// @Success 200 {object} response.Response{data=dto.VersionResponse}
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/versions/{id} [put]
// @Security BearerAuth
func (h *FieldHandler) HandleUpdateVersion(c *gin.Context) {
	projectKey := c.Param("key")
	versionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "field.invalid_version_id")
		return
	}

	var req dto.UpdateVersionRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.BadRequest(c, bindErr.Error())
		return
	}

	result, err := h.fieldService.UpdateVersion(c.Request.Context(), projectKey, versionID, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	response.Success(c, result)
}

// HandleDeleteVersion 删除项目版本
// @Summary 删除项目版本
// @Description 删除项目的版本
// @Tags Version
// @Produce json
// @Param key path string true "项目Key"
// @Param id path int true "版本ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/versions/{id} [delete]
// @Security BearerAuth
func (h *FieldHandler) HandleDeleteVersion(c *gin.Context) {
	projectKey := c.Param("key")
	versionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "field.invalid_version_id")
		return
	}

	if err := h.fieldService.DeleteVersion(c.Request.Context(), projectKey, versionID); err != nil {
		h.handleError(c, err)
		return
	}

	response.Success(c, nil)
}

// HandleListVersions 获取项目版本列表
// @Summary 获取版本列表
// @Description 获取项目的所有版本
// @Tags Version
// @Produce json
// @Param key path string true "项目Key"
// @Success 200 {object} response.Response{data=[]dto.VersionResponse}
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/versions [get]
// @Security BearerAuth
func (h *FieldHandler) HandleListVersions(c *gin.Context) {
	projectKey := c.Param("key")

	result, err := h.fieldService.ListVersions(c.Request.Context(), projectKey)
	if err != nil {
		h.handleError(c, err)
		return
	}

	response.Success(c, result)
}

// ============ 组件管理 ============

// HandleCreateComponent 创建项目组件
// @Summary 创建项目组件
// @Description 为项目创建新组件
// @Tags Component
// @Accept json
// @Produce json
// @Param key path string true "项目Key"
// @Param request body dto.CreateComponentRequest true "创建组件请求"
// @Success 201 {object} response.Response{data=dto.ComponentResponse}
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/components [post]
// @Security BearerAuth
func (h *FieldHandler) HandleCreateComponent(c *gin.Context) {
	projectKey := c.Param("key")

	var req dto.CreateComponentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	result, err := h.fieldService.CreateComponent(c.Request.Context(), projectKey, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	response.Created(c, result)
}

// HandleUpdateComponent 更新项目组件
// @Summary 更新项目组件
// @Description 更新项目的组件信息
// @Tags Component
// @Accept json
// @Produce json
// @Param key path string true "项目Key"
// @Param id path int true "组件ID"
// @Param request body dto.UpdateComponentRequest true "更新组件请求"
// @Success 200 {object} response.Response{data=dto.ComponentResponse}
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/components/{id} [put]
// @Security BearerAuth
func (h *FieldHandler) HandleUpdateComponent(c *gin.Context) {
	projectKey := c.Param("key")
	componentID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "field.invalid_component_id")
		return
	}

	var req dto.UpdateComponentRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.BadRequest(c, bindErr.Error())
		return
	}

	result, err := h.fieldService.UpdateComponent(c.Request.Context(), projectKey, componentID, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	response.Success(c, result)
}

// HandleDeleteComponent 删除项目组件
// @Summary 删除项目组件
// @Description 删除项目的组件
// @Tags Component
// @Produce json
// @Param key path string true "项目Key"
// @Param id path int true "组件ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/components/{id} [delete]
// @Security BearerAuth
func (h *FieldHandler) HandleDeleteComponent(c *gin.Context) {
	projectKey := c.Param("key")
	componentID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "field.invalid_component_id")
		return
	}

	if err := h.fieldService.DeleteComponent(c.Request.Context(), projectKey, componentID); err != nil {
		h.handleError(c, err)
		return
	}

	response.Success(c, nil)
}

// HandleListComponents 获取项目组件列表
// @Summary 获取组件列表
// @Description 获取项目的所有组件
// @Tags Component
// @Produce json
// @Param key path string true "项目Key"
// @Success 200 {object} response.Response{data=[]dto.ComponentResponse}
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/components [get]
// @Security BearerAuth
func (h *FieldHandler) HandleListComponents(c *gin.Context) {
	projectKey := c.Param("key")

	result, err := h.fieldService.ListComponents(c.Request.Context(), projectKey)
	if err != nil {
		h.handleError(c, err)
		return
	}

	response.Success(c, result)
}

// ============ 标签管理 ============

// HandleCreateLabel 创建标签
// @Summary 创建标签
// @Description 为项目创建新标签
// @Tags Label
// @Accept json
// @Produce json
// @Param key path string true "项目Key"
// @Param request body dto.CreateLabelRequest true "创建标签请求"
// @Success 201 {object} response.Response{data=dto.LabelResponse}
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/labels [post]
// @Security BearerAuth
func (h *FieldHandler) HandleCreateLabel(c *gin.Context) {
	projectKey := c.Param("key")

	var req dto.CreateLabelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	result, err := h.fieldService.CreateLabel(c.Request.Context(), projectKey, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	response.Created(c, result)
}

// HandleUpdateLabel 更新标签
// @Summary 更新标签
// @Description 更新项目的标签信息
// @Tags Label
// @Accept json
// @Produce json
// @Param key path string true "项目Key"
// @Param id path int true "标签ID"
// @Param request body dto.UpdateLabelRequest true "更新标签请求"
// @Success 200 {object} response.Response{data=dto.LabelResponse}
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/labels/{id} [put]
// @Security BearerAuth
func (h *FieldHandler) HandleUpdateLabel(c *gin.Context) {
	projectKey := c.Param("key")
	labelID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "field.invalid_label_id")
		return
	}

	var req dto.UpdateLabelRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.BadRequest(c, bindErr.Error())
		return
	}

	result, err := h.fieldService.UpdateLabel(c.Request.Context(), projectKey, labelID, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	response.Success(c, result)
}

// HandleDeleteLabel 删除标签
// @Summary 删除标签
// @Description 删除项目的标签
// @Tags Label
// @Produce json
// @Param key path string true "项目Key"
// @Param id path int true "标签ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/labels/{id} [delete]
// @Security BearerAuth
func (h *FieldHandler) HandleDeleteLabel(c *gin.Context) {
	projectKey := c.Param("key")
	labelID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "field.invalid_label_id")
		return
	}

	if err := h.fieldService.DeleteLabel(c.Request.Context(), projectKey, labelID); err != nil {
		h.handleError(c, err)
		return
	}

	response.Success(c, nil)
}

// HandleListLabels 获取标签列表
// @Summary 获取标签列表
// @Description 获取项目的所有标签
// @Tags Label
// @Produce json
// @Param key path string true "项目Key"
// @Success 200 {object} response.Response{data=[]dto.LabelResponse}
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/labels [get]
// @Security BearerAuth
func (h *FieldHandler) HandleListLabels(c *gin.Context) {
	projectKey := c.Param("key")

	result, err := h.fieldService.ListLabels(c.Request.Context(), projectKey)
	if err != nil {
		h.handleError(c, err)
		return
	}

	response.Success(c, result)
}

// ============ 字段值 ============

// HandleGetIssueFieldValues 获取工单的字段值
// @Summary 获取工单字段值
// @Description 获取指定工单的所有自定义字段值
// @Tags FieldValue
// @Produce json
// @Param issue_id path int true "工单ID"
// @Success 200 {object} response.Response{data=[]dto.FieldValueResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse "工单不存在"
// @Router /api/v1/issues/{issue_id}/field-values [get]
// @Security BearerAuth
func (h *FieldHandler) HandleGetIssueFieldValues(c *gin.Context) {
	issueID, err := strconv.ParseUint(c.Param("issue_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "field.invalid_issue_id")
		return
	}

	result, err := h.fieldService.GetFieldValues(c.Request.Context(), issueID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	response.Success(c, result)
}

// ============ 全局字段管理 ============

// HandleListGlobalFields 获取全局字段列表
func (h *FieldHandler) HandleListGlobalFields(c *gin.Context) {
	result, err := h.fieldService.ListGlobalFields(c.Request.Context())
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, result)
}

// HandleCreateGlobalField 创建全局自定义字段
func (h *FieldHandler) HandleCreateGlobalField(c *gin.Context) {
	var req dto.CreateFieldRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	result, err := h.fieldService.CreateGlobalField(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Created(c, result)
}

// HandleUpdateGlobalField 更新全局字段
func (h *FieldHandler) HandleUpdateGlobalField(c *gin.Context) {
	fieldID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "field.invalid_id")
		return
	}
	var req dto.UpdateFieldRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.BadRequest(c, bindErr.Error())
		return
	}
	result, err := h.fieldService.UpdateGlobalField(c.Request.Context(), fieldID, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, result)
}

// HandleDeleteGlobalField 删除全局字段
func (h *FieldHandler) HandleDeleteGlobalField(c *gin.Context) {
	fieldID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "field.invalid_id")
		return
	}
	if err := h.fieldService.DeleteGlobalField(c.Request.Context(), fieldID); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, nil)
}

// HandleGetFieldUsage 获取字段使用情况
func (h *FieldHandler) HandleGetFieldUsage(c *gin.Context) {
	fieldID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "field.invalid_id")
		return
	}
	result, err := h.fieldService.GetFieldUsage(c.Request.Context(), fieldID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, result)
}

// ============ 方案模板 ============

// HandleListTemplates 获取所有方案模板
func (h *FieldHandler) HandleListTemplates(c *gin.Context) {
	result, err := h.fieldService.ListTemplates(c.Request.Context())
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, result)
}

// HandleCreateTemplate 创建方案模板
func (h *FieldHandler) HandleCreateTemplate(c *gin.Context) {
	var req dto.CreateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	userID := c.GetUint64("user_id")
	result, err := h.fieldService.CreateTemplate(c.Request.Context(), userID, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Created(c, result)
}

// HandleGetTemplate 获取模板详情
func (h *FieldHandler) HandleGetTemplate(c *gin.Context) {
	templateID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "field.invalid_template_id")
		return
	}
	result, err := h.fieldService.GetTemplate(c.Request.Context(), templateID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, result)
}

// HandleUpdateTemplate 更新模板
func (h *FieldHandler) HandleUpdateTemplate(c *gin.Context) {
	templateID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "field.invalid_template_id")
		return
	}
	var req dto.UpdateTemplateRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.BadRequest(c, bindErr.Error())
		return
	}
	result, err := h.fieldService.UpdateTemplate(c.Request.Context(), templateID, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, result)
}

// HandleDeleteTemplate 删除模板
func (h *FieldHandler) HandleDeleteTemplate(c *gin.Context) {
	templateID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "field.invalid_template_id")
		return
	}
	if err := h.fieldService.DeleteTemplate(c.Request.Context(), templateID); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, nil)
}

// HandleUpdateTemplateItems 更新模板字段项
func (h *FieldHandler) HandleUpdateTemplateItems(c *gin.Context) {
	templateID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "field.invalid_template_id")
		return
	}
	var req dto.UpdateTemplateItemsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.fieldService.UpdateTemplateItems(c.Request.Context(), templateID, &req); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, nil)
}

// HandleApplyTemplate 套用模板到工单类型
func (h *FieldHandler) HandleApplyTemplate(c *gin.Context) {
	projectKey := c.Param("key")
	issueTypeID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "field.invalid_issue_type_id")
		return
	}
	var req dto.ApplyTemplateRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.BadRequest(c, bindErr.Error())
		return
	}
	result, err := h.fieldService.ApplyTemplate(c.Request.Context(), projectKey, issueTypeID, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, result)
}

// handleError 统一错误处理
func (h *FieldHandler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrProjectNotFound):
		response.NotFound(c, "workflow.project_not_found")
	case errors.Is(err, service.ErrFieldNotFound):
		response.NotFound(c, "field.not_found")
	case errors.Is(err, service.ErrFieldKeyExists):
		response.Error(c, http.StatusConflict, "CONFLICT", "field.key_exists")
	case errors.Is(err, service.ErrCannotDeleteSystem):
		response.Forbidden(c, "field.system_undeletable")
	case errors.Is(err, service.ErrCannotModifySystem):
		response.Forbidden(c, "field.system_immutable")
	case errors.Is(err, service.ErrIssueTypeNotFound):
		response.NotFound(c, "project.issue_type_not_found")
	case errors.Is(err, service.ErrVersionNotFound):
		response.NotFound(c, "field.version_not_found")
	case errors.Is(err, service.ErrVersionNameExists):
		response.Error(c, http.StatusConflict, "CONFLICT", "field.version_name_exists")
	case errors.Is(err, service.ErrComponentNotFound):
		response.NotFound(c, "field.component_not_found")
	case errors.Is(err, service.ErrComponentNameExists):
		response.Error(c, http.StatusConflict, "CONFLICT", "field.component_name_exists")
	case errors.Is(err, service.ErrLabelNotFound):
		response.NotFound(c, "field.label_not_found")
	case errors.Is(err, service.ErrLabelNameExists):
		response.Error(c, http.StatusConflict, "CONFLICT", "field.label_name_exists")
	case errors.Is(err, service.ErrTemplateNotFound):
		response.NotFound(c, "field.template_not_found")
	case errors.Is(err, service.ErrTemplateNameExists):
		response.Error(c, http.StatusConflict, "CONFLICT", "field.template_name_exists")
	default:
		response.InternalError(c, err.Error())
	}
}
