package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/kerbos/ticketdesk/internal/api/response"
	"github.com/kerbos/ticketdesk/internal/requirement-pool/dto"
	"github.com/kerbos/ticketdesk/internal/requirement-pool/service"
)

// CategoryHandler 需求分类 HTTP 处理器
type CategoryHandler struct {
	service service.CategoryService
	logger  *zap.Logger
}

// NewCategoryHandler 创建需求分类 HTTP 处理器实例
func NewCategoryHandler(service service.CategoryService, logger *zap.Logger) *CategoryHandler {
	return &CategoryHandler{
		service: service,
		logger:  logger,
	}
}

// HandleList 获取需求分类列表
// @Summary 获取需求分类列表
// @Description 获取所有需求分类
// @Tags RequirementCategory
// @Produce json
// @Success 200 {object} response.Response{data=[]dto.CategoryResponse}
// @Failure 500 {object} response.ErrorResponse
// @Router /api/v1/requirement-categories [get]
// @Security BearerAuth
func (h *CategoryHandler) HandleList(c *gin.Context) {
	categories, err := h.service.List(c.Request.Context())
	if err != nil {
		h.logger.Error("failed to list requirement categories", zap.Error(err))
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, categories)
}

// HandleCreate 创建需求分类
// @Summary 创建需求分类
// @Description 创建一个新的需求分类
// @Tags RequirementCategory
// @Accept json
// @Produce json
// @Param request body dto.CreateCategoryRequest true "创建需求分类请求"
// @Success 201 {object} response.Response{data=dto.CategoryResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/v1/requirement-categories [post]
// @Security BearerAuth
func (h *CategoryHandler) HandleCreate(c *gin.Context) {
	var req dto.CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	cat, err := h.service.Create(c.Request.Context(), &req)
	if err != nil {
		if err.Error() == "分类名称已存在" {
			response.BadRequest(c, err.Error())
			return
		}
		h.logger.Error("failed to create requirement category", zap.Error(err))
		response.InternalError(c, err.Error())
		return
	}

	response.Created(c, cat)
}

// HandleUpdate 更新需求分类
// @Summary 更新需求分类
// @Description 更新需求分类信息
// @Tags RequirementCategory
// @Accept json
// @Produce json
// @Param id path int true "分类ID"
// @Param request body dto.UpdateCategoryRequest true "更新需求分类请求"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/v1/requirement-categories/{id} [put]
// @Security BearerAuth
func (h *CategoryHandler) HandleUpdate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "requirement.invalid_category_id")
		return
	}

	var req dto.UpdateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	if err := h.service.Update(c.Request.Context(), id, &req); err != nil {
		if err.Error() == "分类不存在" {
			response.NotFound(c, err.Error())
			return
		}
		h.logger.Error("failed to update requirement category", zap.Error(err), zap.Uint64("id", id))
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, gin.H{"message": response.T(c, "requirement.updated")})
}

// HandleDelete 删除需求分类
// @Summary 删除需求分类
// @Description 删除需求分类（系统预置不可删除，有关联需求时不可删除）
// @Tags RequirementCategory
// @Produce json
// @Param id path int true "分类ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/v1/requirement-categories/{id} [delete]
// @Security BearerAuth
func (h *CategoryHandler) HandleDelete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "requirement.invalid_category_id")
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		errMsg := err.Error()
		switch errMsg {
		case "分类不存在":
			response.NotFound(c, errMsg)
		case "系统预置分类不可删除":
			response.BadRequest(c, errMsg)
		default:
			h.logger.Error("failed to delete requirement category", zap.Error(err), zap.Uint64("id", id))
			response.InternalError(c, errMsg)
		}
		return
	}

	response.Success(c, gin.H{"message": response.T(c, "requirement.deleted")})
}
