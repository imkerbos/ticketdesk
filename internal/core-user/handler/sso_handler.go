// Package handler 提供用户模块的 HTTP 处理器
package handler

import (
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/kerbos/ticketdesk/internal/api/response"
	"github.com/kerbos/ticketdesk/internal/core-user/dto"
	"github.com/kerbos/ticketdesk/internal/core-user/service"
)

// SSOHandler SSO 处理器
type SSOHandler struct {
	ssoService service.SSOService
}

// NewSSOHandler 创建 SSO 处理器实例
func NewSSOHandler(ssoService service.SSOService) *SSOHandler {
	return &SSOHandler{
		ssoService: ssoService,
	}
}

// HandleGetSSOConfig 获取 SSO 配置
// @Summary 获取 SSO 配置
// @Description 获取 SSO 配置信息（是否启用、提供方名称）
// @Tags SSO
// @Produce json
// @Success 200 {object} response.Response{data=dto.SSOConfigResponse}
// @Router /api/v1/auth/sso/config [get]
func (h *SSOHandler) HandleGetSSOConfig(c *gin.Context) {
	result := h.ssoService.GetSSOConfig(c.Request.Context())
	response.Success(c, result)
}

// HandleSSOAuthorize 获取 SSO 授权 URL
// @Summary 获取 SSO 授权 URL
// @Description 生成 SSO 授权 URL，前端跳转到该 URL 进行认证
// @Tags SSO
// @Produce json
// @Success 200 {object} response.Response{data=dto.SSOAuthorizeResponse}
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/auth/sso/authorize [get]
func (h *SSOHandler) HandleSSOAuthorize(c *gin.Context) {
	result, err := h.ssoService.GetAuthURL(c.Request.Context())
	if err != nil {
		if errors.Is(err, service.ErrSSODisabled) {
			response.BadRequest(c, "user.sso_disabled")
			return
		}
		response.InternalError(c, "user.sso_url_failed")
		return
	}

	response.Success(c, result)
}

// HandleSSOCallback 处理 SSO 回调
// @Summary SSO 回调
// @Description 处理 SSO 认证回调，验证 code 并返回 JWT
// @Tags SSO
// @Accept json
// @Produce json
// @Param request body dto.SSOCallbackRequest true "SSO 回调请求"
// @Success 200 {object} response.Response{data=dto.LoginResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Router /api/v1/auth/sso/callback [post]
func (h *SSOHandler) HandleSSOCallback(c *gin.Context) {
	var req dto.SSOCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	result, err := h.ssoService.HandleCallback(c.Request.Context(), &req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrSSODisabled):
			response.BadRequest(c, "user.sso_disabled")
		case errors.Is(err, service.ErrSSOInvalidState):
			response.BadRequest(c, "user.auth_request_invalid")
		case errors.Is(err, service.ErrSSOCodeExchange):
			response.Unauthorized(c, "user.sso_auth_failed")
		case errors.Is(err, service.ErrSSOTokenVerify):
			response.Unauthorized(c, "user.sso_token_failed")
		case errors.Is(err, service.ErrSSONonceMismatch):
			response.Unauthorized(c, "user.sso_expired")
		case errors.Is(err, service.ErrSSOUserDisabled):
			response.Forbidden(c, "user.disabled")
		case errors.Is(err, service.ErrSSOUserNotAllowed):
			response.Forbidden(c, "user.login_not_allowed")
		default:
			response.InternalError(c, "user.sso_login_failed")
		}
		return
	}

	response.Success(c, result)
}
