// Package handler 提供初始化向导的 HTTP 接口
package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kerbos/ticketdesk/internal/api/response"
	"github.com/kerbos/ticketdesk/internal/setup/dto"
	"github.com/kerbos/ticketdesk/internal/setup/service"
)

// SetupHandler 初始化向导处理器
type SetupHandler struct {
	svc service.Service
}

// NewSetupHandler 创建初始化向导处理器
func NewSetupHandler(svc service.Service) *SetupHandler {
	return &SetupHandler{svc: svc}
}

// HandleStatus 查询初始化状态
// @Summary 查询初始化状态
// @Tags Setup
// @Produce json
// @Success 200 {object} response.Response{data=dto.StatusResponse}
// @Router /setup/status [get]
func (h *SetupHandler) HandleStatus(c *gin.Context) {
	response.Success(c, dto.StatusResponse{Initialized: h.svc.Initialized(c.Request.Context())})
}

// HandleSetup 执行初始化
// @Summary 执行首次初始化
// @Tags Setup
// @Accept json
// @Produce json
// @Param request body dto.SetupRequest true "初始化信息"
// @Success 200 {object} response.Response
// @Router /setup [post]
func (h *SetupHandler) HandleSetup(c *gin.Context) {
	var req dto.SetupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	switch err := h.svc.Setup(c.Request.Context(), &req); {
	case err == nil:
		response.Success(c, nil)
	case errors.Is(err, service.ErrAlreadyInitialized):
		// 409 而不是 400：这不是请求写错了，是状态已经变了。
		// 并发初始化时落败的那一方会走到这里。
		response.Error(c, http.StatusConflict, "CONFLICT", "setup.already_initialized")
	case errors.Is(err, service.ErrBadToken):
		response.UnauthorizedT(c, "setup.bad_token")
	default:
		response.InternalErrorT(c, "setup.failed")
	}
}
