// Package handler 提供用户模块 HTTP 处理层
package handler

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/kerbos/ticketdesk/internal/api/response"
	"github.com/kerbos/ticketdesk/internal/core-user/dto"
	"github.com/kerbos/ticketdesk/internal/core-user/service"
)

// APITokenHandler API token HTTP 处理器
type APITokenHandler struct {
	tokenSvc service.APITokenService
}

// NewAPITokenHandler 创建 API token 处理器实例
func NewAPITokenHandler(tokenSvc service.APITokenService) *APITokenHandler {
	return &APITokenHandler{tokenSvc: tokenSvc}
}

// HandleCreate 创建 API token
// 注意：不允许用 PAT 创建新 PAT，避免权限提升
// @Summary 创建 API Token
// @Description 为当前用户创建新的 API Token，不允许用 PAT 自身调用
// @Tags Token
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body dto.CreateTokenRequest true "创建 Token 请求体"
// @Success 201 {object} response.Response{data=dto.CreateTokenResponse} "创建成功"
// @Failure 400 {object} response.ErrorResponse "参数错误"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Failure 403 {object} response.ErrorResponse "不能用 API token 创建新 token"
// @Router /api/v1/users/me/tokens [post]
func (h *APITokenHandler) HandleCreate(c *gin.Context) {
	// 拒绝用 PAT 自己创建 PAT
	if isPAT, _ := c.Get("is_pat"); isPAT == true {
		response.Forbidden(c, "user.token_by_token")
		return
	}

	var req dto.CreateTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	userID := c.GetUint64("user_id")
	result, err := h.tokenSvc.Create(c.Request.Context(), userID, &req)
	if err != nil {
		if errors.Is(err, service.ErrTokenLimitExceeded) {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalError(c, "user.token_create_failed")
		return
	}
	response.Created(c, result)
}

// HandleList 列出当前用户所有 token
// @Summary 列出 API Token
// @Description 列出当前用户的所有 API Token
// @Tags Token
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response{data=[]dto.TokenResponse} "获取成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/users/me/tokens [get]
func (h *APITokenHandler) HandleList(c *gin.Context) {
	userID := c.GetUint64("user_id")
	tokens, err := h.tokenSvc.List(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c, "user.token_list_failed")
		return
	}
	response.Success(c, tokens)
}

// HandleDelete 撤销指定 token
// @Summary 撤销 API Token
// @Description 撤销并删除指定的 API Token
// @Tags Token
// @Produce json
// @Security BearerAuth
// @Param id path int true "Token ID"
// @Success 200 {object} response.Response "撤销成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Failure 404 {object} response.ErrorResponse "Token 不存在"
// @Router /api/v1/users/me/tokens/{id} [delete]
func (h *APITokenHandler) HandleDelete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "user.token_invalid_id")
		return
	}
	userID := c.GetUint64("user_id")
	if err := h.tokenSvc.Delete(c.Request.Context(), userID, id); err != nil {
		if errors.Is(err, service.ErrTokenNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "user.token_revoke_failed")
		return
	}
	response.Success(c, nil)
}
