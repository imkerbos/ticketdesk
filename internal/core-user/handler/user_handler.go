// Package handler 提供用户模块的 HTTP 处理器
package handler

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/kerbos/ticketdesk/internal/api/response"
	"github.com/kerbos/ticketdesk/internal/core-user/dto"
	"github.com/kerbos/ticketdesk/internal/core-user/service"
)

// UserHandler 用户处理器
type UserHandler struct {
	userService service.UserService
	mfaService  service.MFAService
}

// NewUserHandler 创建用户处理器实例
func NewUserHandler(userService service.UserService, mfaService service.MFAService) *UserHandler {
	return &UserHandler{
		userService: userService,
		mfaService:  mfaService,
	}
}

// HandleLogin 处理登录请求
// @Summary 用户登录
// @Description 使用用户名和密码登录
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body dto.LoginRequest true "登录请求"
// @Success 200 {object} response.Response{data=dto.LoginResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse "账号已禁用"
// @Router /api/v1/auth/login [post]
func (h *UserHandler) HandleLogin(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	result, err := h.userService.Login(c.Request.Context(), &req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			response.Unauthorized(c, err.Error())
		case errors.Is(err, service.ErrAccountUsesSSOLogin):
			response.Forbidden(c, err.Error())
		case errors.Is(err, service.ErrUserDisabled):
			response.Forbidden(c, err.Error())
		default:
			response.InternalError(c, "user.login_failed")
		}
		return
	}

	response.Success(c, result)
}

// HandleRegister 处理注册请求
// @Summary 用户注册
// @Description 注册新用户
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body dto.RegisterRequest true "注册请求"
// @Success 201 {object} response.Response{data=dto.UserResponse}
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/auth/register [post]
func (h *UserHandler) HandleRegister(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	result, err := h.userService.Register(c.Request.Context(), &req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUsernameExists):
			response.BadRequest(c, err.Error())
		case errors.Is(err, service.ErrEmailExists):
			response.BadRequest(c, err.Error())
		default:
			response.InternalError(c, "user.register_failed")
		}
		return
	}

	response.Created(c, result)
}

// HandleRefreshToken 处理刷新 Token 请求
// @Summary 刷新 Token
// @Description 使用 Refresh Token 获取新的 Access Token
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body dto.RefreshTokenRequest true "刷新 Token 请求"
// @Success 200 {object} response.Response{data=dto.LoginResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Router /api/v1/auth/refresh [post]
func (h *UserHandler) HandleRefreshToken(c *gin.Context) {
	var req dto.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	result, err := h.userService.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserNotFound):
			response.Unauthorized(c, "user.not_found")
		case errors.Is(err, service.ErrUserDisabled):
			response.Forbidden(c, err.Error())
		default:
			response.Unauthorized(c, "user.token_invalid")
		}
		return
	}

	response.Success(c, result)
}

// HandleGetCurrentUser 获取当前用户信息
// @Summary 获取当前用户信息
// @Description 获取当前登录用户的详细信息
// @Tags User
// @Produce json
// @Success 200 {object} response.Response{data=dto.UserResponse}
// @Failure 401 {object} response.ErrorResponse
// @Router /api/v1/users/me [get]
// @Security BearerAuth
func (h *UserHandler) HandleGetCurrentUser(c *gin.Context) {
	userID := c.GetUint64("user_id")
	if userID == 0 {
		response.Unauthorized(c, "user.info_missing")
		return
	}

	result, err := h.userService.GetCurrentUser(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "user.get_failed")
		return
	}

	response.Success(c, result)
}

// HandleUpdateCurrentUser 更新当前用户资料（自助）
// @Summary 更新当前用户资料
// @Description 当前登录用户更新自己的基本资料（display_name / email / avatar / lark_open_id / telegram_user_id）
// @Tags User
// @Accept json
// @Produce json
// @Param request body dto.UpdateUserRequest true "更新当前用户资料请求"
// @Success 200 {object} response.Response{data=dto.UserResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Router /api/v1/users/me [put]
// @Security BearerAuth
func (h *UserHandler) HandleUpdateCurrentUser(c *gin.Context) {
	userID := c.GetUint64("user_id")
	if userID == 0 {
		response.Unauthorized(c, "user.info_missing")
		return
	}

	var req dto.UpdateUserRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.BadRequestValidation(c, bindErr)
		return
	}

	result, err := h.userService.UpdateUser(c.Request.Context(), userID, &req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserNotFound):
			response.NotFound(c, err.Error())
		case errors.Is(err, service.ErrEmailExists):
			response.BadRequest(c, err.Error())
		default:
			response.InternalError(c, "user.profile_update_failed")
		}
		return
	}

	response.Success(c, result)
}

// HandleGetUser 获取用户详情
// @Summary 获取用户详情
// @Description 根据用户 ID 获取用户详细信息
// @Tags User
// @Produce json
// @Param id path int true "用户 ID"
// @Success 200 {object} response.Response{data=dto.UserResponse}
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/users/{id} [get]
// @Security BearerAuth
func (h *UserHandler) HandleGetUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "user.invalid_id")
		return
	}

	result, err := h.userService.GetUser(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "user.get_failed")
		return
	}

	response.Success(c, result)
}

// HandleCreateUser 创建用户（管理员）
// @Summary 创建用户
// @Description 管理员创建新用户
// @Tags User
// @Accept json
// @Produce json
// @Param request body dto.CreateUserRequest true "创建用户请求"
// @Success 201 {object} response.Response{data=dto.UserResponse}
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/users [post]
// @Security BearerAuth
func (h *UserHandler) HandleCreateUser(c *gin.Context) {
	var req dto.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	result, err := h.userService.CreateUser(c.Request.Context(), &req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUsernameExists):
			response.BadRequest(c, err.Error())
		case errors.Is(err, service.ErrEmailExists):
			response.BadRequest(c, err.Error())
		default:
			response.InternalError(c, "user.create_failed")
		}
		return
	}

	response.Created(c, result)
}

// HandleUpdateUser 更新用户信息
// @Summary 更新用户信息
// @Description 更新指定用户的信息
// @Tags User
// @Accept json
// @Produce json
// @Param id path int true "用户 ID"
// @Param request body dto.UpdateUserRequest true "更新用户请求"
// @Success 200 {object} response.Response{data=dto.UserResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/users/{id} [put]
// @Security BearerAuth
func (h *UserHandler) HandleUpdateUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "user.invalid_id")
		return
	}

	var req dto.UpdateUserRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.BadRequestValidation(c, bindErr)
		return
	}

	result, err := h.userService.UpdateUser(c.Request.Context(), id, &req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserNotFound):
			response.NotFound(c, err.Error())
		case errors.Is(err, service.ErrEmailExists):
			response.BadRequest(c, err.Error())
		default:
			response.InternalError(c, "user.update_failed")
		}
		return
	}

	response.Success(c, result)
}

// HandleUpdatePassword 修改密码
// @Summary 修改密码
// @Description 修改当前用户的密码
// @Tags User
// @Accept json
// @Produce json
// @Param request body dto.UpdatePasswordRequest true "修改密码请求"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/users/me/password [put]
// @Security BearerAuth
func (h *UserHandler) HandleUpdatePassword(c *gin.Context) {
	userID := c.GetUint64("user_id")
	if userID == 0 {
		response.Unauthorized(c, "user.info_missing")
		return
	}

	var req dto.UpdatePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	err := h.userService.UpdatePassword(c.Request.Context(), userID, &req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidOldPassword):
			response.BadRequest(c, err.Error())
		case errors.Is(err, service.ErrUserNotFound):
			response.NotFound(c, err.Error())
		default:
			response.InternalError(c, "user.change_password_failed")
		}
		return
	}

	response.Success(c, gin.H{"message": response.T(c, "user.password_changed")})
}

// HandleResetPassword 重置用户密码（管理员）
// @Summary 重置用户密码
// @Description 管理员重置指定用户的密码
// @Tags User
// @Accept json
// @Produce json
// @Param id path int true "用户 ID"
// @Param request body dto.ResetPasswordRequest true "重置密码请求"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/users/{id}/reset-password [post]
// @Security BearerAuth
func (h *UserHandler) HandleResetPassword(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "user.invalid_id")
		return
	}

	var req dto.ResetPasswordRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.BadRequestValidation(c, bindErr)
		return
	}

	err = h.userService.ResetPassword(c.Request.Context(), id, &req)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "user.reset_password_failed")
		return
	}

	response.Success(c, gin.H{"message": response.T(c, "user.password_reset")})
}

// HandleEnableUser 启用用户
// @Summary 启用用户
// @Description 启用指定用户
// @Tags User
// @Produce json
// @Param id path int true "用户 ID"
// @Success 200 {object} response.Response
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/users/{id}/enable [post]
// @Security BearerAuth
func (h *UserHandler) HandleEnableUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "user.invalid_id")
		return
	}

	err = h.userService.EnableUser(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "user.enable_failed")
		return
	}

	response.Success(c, gin.H{"message": response.T(c, "user.already_enabled")})
}

// HandleDisableUser 禁用用户
// @Summary 禁用用户
// @Description 禁用指定用户
// @Tags User
// @Produce json
// @Param id path int true "用户 ID"
// @Success 200 {object} response.Response
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/users/{id}/disable [post]
// @Security BearerAuth
func (h *UserHandler) HandleDisableUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "user.invalid_id")
		return
	}

	// 禁止禁用自己
	if currentUserID := c.GetUint64("user_id"); currentUserID == id {
		response.Forbidden(c, "user.cannot_disable_self")
		return
	}
	// 禁止禁用 admin (id=1)
	if id == 1 {
		response.Forbidden(c, "user.cannot_disable_admin")
		return
	}

	err = h.userService.DisableUser(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "user.disable_failed")
		return
	}

	response.Success(c, gin.H{"message": response.T(c, "user.already_disabled")})
}

// HandleDeleteUser 删除用户
// @Summary 删除用户
// @Description 删除指定用户（软删除）
// @Tags User
// @Produce json
// @Param id path int true "用户 ID"
// @Success 200 {object} response.Response
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/users/{id} [delete]
// @Security BearerAuth
func (h *UserHandler) HandleDeleteUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "user.invalid_id")
		return
	}

	// 禁止删除自己
	if currentUserID := c.GetUint64("user_id"); currentUserID == id {
		response.Forbidden(c, "user.cannot_delete_self")
		return
	}
	// 禁止删除 admin (id=1)
	if id == 1 {
		response.Forbidden(c, "user.cannot_delete_admin")
		return
	}

	err = h.userService.DeleteUser(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "user.delete_failed")
		return
	}

	response.Success(c, gin.H{"message": response.T(c, "user.deleted")})
}

// HandleListUsers 获取用户列表
// @Summary 获取用户列表
// @Description 分页获取用户列表
// @Tags User
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Param keyword query string false "搜索关键字"
// @Param status query int false "用户状态 (0-禁用, 1-启用)"
// @Success 200 {object} response.Response{data=response.PageData}
// @Router /api/v1/users [get]
// @Security BearerAuth
func (h *UserHandler) HandleListUsers(c *gin.Context) {
	var req dto.ListUsersRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	users, total, err := h.userService.ListUsers(c.Request.Context(), &req)
	if err != nil {
		response.InternalError(c, "user.list_failed")
		return
	}

	response.SuccessWithPage(c, users, total, req.GetDefaultPage(), req.GetDefaultPageSize())
}

// HandleListAllUsers 获取所有用户（用于选择器）
// @Summary 获取所有用户
// @Description 获取所有启用的用户列表（不分页，用于选择器）
// @Tags User
// @Produce json
// @Success 200 {object} response.Response{data=[]dto.UserResponse}
// @Router /api/v1/users/all [get]
// @Security BearerAuth
func (h *UserHandler) HandleListAllUsers(c *gin.Context) {
	// 获取所有启用的用户
	statusEnabled := int8(1)
	req := &dto.ListUsersRequest{
		Page:     1,
		PageSize: 1000,           // 获取足够多的用户
		Status:   &statusEnabled, // 只获取启用的用户
	}

	users, _, err := h.userService.ListUsers(c.Request.Context(), req)
	if err != nil {
		response.InternalError(c, "user.list_failed")
		return
	}

	// 转换为简要信息
	briefs := make([]map[string]interface{}, len(users))
	for i, u := range users {
		briefs[i] = map[string]interface{}{
			"id":           u.ID,
			"username":     u.Username,
			"display_name": u.DisplayName,
		}
	}

	response.Success(c, briefs)
}

// ============ MFA 相关处理器 ============

// HandleGetMFAStatus 获取 MFA 状态
// @Summary 获取 MFA 状态
// @Description 获取当前用户的 MFA 状态
// @Tags MFA
// @Produce json
// @Success 200 {object} response.Response{data=dto.MFAStatusResponse}
// @Router /api/v1/users/me/mfa [get]
// @Security BearerAuth
func (h *UserHandler) HandleGetMFAStatus(c *gin.Context) {
	userID := c.GetUint64("user_id")
	if userID == 0 {
		response.Unauthorized(c, "user.info_missing")
		return
	}

	status, err := h.mfaService.GetMFAStatus(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "user.mfa_status_failed")
		return
	}

	response.Success(c, status)
}

// HandleSetupMFA 开始 MFA 设置
// @Summary 开始 MFA 设置
// @Description 生成 MFA 密钥和二维码 URL
// @Tags MFA
// @Produce json
// @Success 200 {object} response.Response{data=dto.MFASetupResponse}
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/users/me/mfa/setup [post]
// @Security BearerAuth
func (h *UserHandler) HandleSetupMFA(c *gin.Context) {
	userID := c.GetUint64("user_id")
	if userID == 0 {
		response.Unauthorized(c, "user.info_missing")
		return
	}

	result, err := h.mfaService.SetupMFA(c.Request.Context(), userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrMFAAlreadyEnabled):
			response.BadRequest(c, err.Error())
		case errors.Is(err, service.ErrUserNotFound):
			response.NotFound(c, err.Error())
		default:
			response.InternalError(c, "user.mfa_setup_failed")
		}
		return
	}

	response.Success(c, result)
}

// HandleEnableMFA 验证并启用 MFA
// @Summary 验证并启用 MFA
// @Description 验证 TOTP 码并启用 MFA
// @Tags MFA
// @Accept json
// @Produce json
// @Param request body dto.MFAVerifyRequest true "MFA 验证请求"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/users/me/mfa/enable [post]
// @Security BearerAuth
func (h *UserHandler) HandleEnableMFA(c *gin.Context) {
	userID := c.GetUint64("user_id")
	if userID == 0 {
		response.Unauthorized(c, "user.info_missing")
		return
	}

	var req dto.MFAVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	err := h.mfaService.VerifyAndEnableMFA(c.Request.Context(), userID, req.Code)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrMFAAlreadyEnabled):
			response.BadRequest(c, err.Error())
		case errors.Is(err, service.ErrMFASetupNotStarted):
			response.BadRequest(c, err.Error())
		case errors.Is(err, service.ErrMFAInvalidCode):
			response.BadRequest(c, err.Error())
		case errors.Is(err, service.ErrUserNotFound):
			response.NotFound(c, err.Error())
		default:
			response.InternalError(c, "user.mfa_enable_failed")
		}
		return
	}

	response.Success(c, gin.H{"message": response.T(c, "user.mfa_enabled")})
}

// HandleDisableMFA 禁用 MFA
// @Summary 禁用 MFA
// @Description 验证 TOTP 码并禁用 MFA
// @Tags MFA
// @Accept json
// @Produce json
// @Param request body dto.MFAVerifyRequest true "MFA 验证请求"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/users/me/mfa/disable [post]
// @Security BearerAuth
func (h *UserHandler) HandleDisableMFA(c *gin.Context) {
	userID := c.GetUint64("user_id")
	if userID == 0 {
		response.Unauthorized(c, "user.info_missing")
		return
	}

	var req dto.MFAVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	err := h.mfaService.DisableMFA(c.Request.Context(), userID, req.Code)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrMFANotEnabled):
			response.BadRequest(c, err.Error())
		case errors.Is(err, service.ErrMFAInvalidCode):
			response.BadRequest(c, err.Error())
		case errors.Is(err, service.ErrUserNotFound):
			response.NotFound(c, err.Error())
		default:
			response.InternalError(c, "user.mfa_disable_failed")
		}
		return
	}

	response.Success(c, gin.H{"message": response.T(c, "user.mfa_disabled")})
}

// HandleVerifyMFA 验证 MFA 码并完成登录
// @Summary 验证 MFA 码
// @Description 登录第二步：提交登录第一步下发的挑战令牌与 TOTP 码，换取正式令牌对
// @Tags MFA
// @Accept json
// @Produce json
// @Param request body dto.MFALoginRequest true "MFA 登录请求"
// @Success 200 {object} response.Response{data=dto.LoginResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse "挑战令牌无效或已过期"
// @Router /api/v1/auth/mfa/verify [post]
func (h *UserHandler) HandleVerifyMFA(c *gin.Context) {
	var req dto.MFALoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	result, err := h.userService.CompleteMFALogin(c.Request.Context(), req.MFAToken, req.Code)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidMFAToken):
			response.Unauthorized(c, err.Error())
		case errors.Is(err, service.ErrMFANotEnabled):
			response.BadRequest(c, err.Error())
		case errors.Is(err, service.ErrMFAInvalidCode):
			response.BadRequest(c, "user.code_wrong")
		case errors.Is(err, service.ErrUserDisabled):
			response.Forbidden(c, err.Error())
		case errors.Is(err, service.ErrUserNotFound):
			response.NotFound(c, err.Error())
		default:
			response.InternalError(c, "user.mfa_verify_failed")
		}
		return
	}

	response.Success(c, result)
}

// HandleForgotPassword 处理忘记密码请求
// @Summary 忘记密码
// @Description 请求重置密码，系统将发送重置链接到用户邮箱
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body dto.ForgotPasswordRequest true "忘记密码请求"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/auth/forgot-password [post]
func (h *UserHandler) HandleForgotPassword(c *gin.Context) {
	var req dto.ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	err := h.userService.ForgotPassword(c.Request.Context(), &req)
	if err != nil {
		response.InternalError(c, "user.request_failed")
		return
	}

	response.Success(c, gin.H{"message": response.T(c, "user.reset_mail_sent")})
}

// HandleVerifyResetToken 验证重置密码令牌
// @Summary 验证重置密码令牌
// @Description 验证重置密码令牌是否有效
// @Tags Auth
// @Produce json
// @Param token query string true "重置密码令牌"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/auth/verify-reset-token [get]
func (h *UserHandler) HandleVerifyResetToken(c *gin.Context) {
	var req dto.VerifyResetTokenRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	err := h.userService.VerifyResetToken(c.Request.Context(), req.Token)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidResetToken):
			response.BadRequest(c, "user.reset_token_invalid")
		case errors.Is(err, service.ErrResetTokenExpired):
			response.BadRequest(c, "user.reset_token_expired")
		default:
			response.InternalError(c, "user.verify_token_failed")
		}
		return
	}

	response.Success(c, gin.H{"message": response.T(c, "user.token_valid")})
}

// HandleResetPasswordWithToken 使用令牌重置密码
// @Summary 使用令牌重置密码
// @Description 使用重置密码令牌设置新密码
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body dto.ResetPasswordWithTokenRequest true "重置密码请求"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/auth/reset-password [post]
func (h *UserHandler) HandleResetPasswordWithToken(c *gin.Context) {
	var req dto.ResetPasswordWithTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	err := h.userService.ResetPasswordWithToken(c.Request.Context(), &req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidResetToken):
			response.BadRequest(c, "user.reset_token_invalid")
		case errors.Is(err, service.ErrResetTokenExpired):
			response.BadRequest(c, "user.reset_token_expired")
		default:
			response.InternalError(c, "user.reset_password_failed")
		}
		return
	}

	response.Success(c, gin.H{"message": response.T(c, "user.password_reset")})
}
