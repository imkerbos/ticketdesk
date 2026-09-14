// Package dto 定义用户模块的数据传输对象
package dto

import "time"

// ============ 请求 DTO ============

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=6,max=50"`
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Username    string `json:"username" binding:"required,min=3,max=50"`
	Email       string `json:"email" binding:"required,email,max=100"`
	Password    string `json:"password" binding:"required,min=6,max=50"`
	DisplayName string `json:"display_name" binding:"max=100"`
}

// RefreshTokenRequest 刷新 Token 请求
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// UpdateUserRequest 更新用户请求
type UpdateUserRequest struct {
	DisplayName    *string `json:"display_name" binding:"omitempty,max=100"`
	AvatarURL      *string `json:"avatar_url" binding:"omitempty,max=255,url"`
	Email          *string `json:"email" binding:"omitempty,email,max=100"`
	LarkOpenID     *string `json:"lark_open_id" binding:"omitempty,max=64"`
	TelegramUserID *string `json:"telegram_user_id" binding:"omitempty,max=32"`
	// Locale 偏好语言。空串是有意义的值 —— 表示"跟随站点设置"，
	// 所以用指针区分"没传"和"清空"。
	Locale *string `json:"locale" binding:"omitempty,oneof=zh-CN en-US"`
}

// CreateUserRequest 创建用户请求（管理员）
type CreateUserRequest struct {
	Username    string   `json:"username" binding:"required,min=3,max=50"`
	Email       string   `json:"email" binding:"required,email,max=100"`
	Password    string   `json:"password" binding:"required,min=6,max=50"`
	DisplayName string   `json:"display_name" binding:"max=100"`
	Status      *int8    `json:"status" binding:"omitempty,oneof=0 1"`
	Roles       []string `json:"roles" binding:"omitempty"`
}

// UpdatePasswordRequest 修改密码请求
type UpdatePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required,min=6,max=50"`
	NewPassword string `json:"new_password" binding:"required,min=6,max=50"`
}

// ResetPasswordRequest 重置密码请求（管理员）
type ResetPasswordRequest struct {
	NewPassword string `json:"new_password" binding:"required,min=6,max=50"`
}

// ForgotPasswordRequest 忘记密码请求
type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email,max=100"`
}

// ResetPasswordWithTokenRequest 使用令牌重置密码请求
type ResetPasswordWithTokenRequest struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6,max=50"`
}

// VerifyResetTokenRequest 验证重置令牌请求
type VerifyResetTokenRequest struct {
	Token string `form:"token" binding:"required"`
}

// ListUsersRequest 用户列表请求
type ListUsersRequest struct {
	Page     int    `form:"page" binding:"omitempty,min=1"`
	PageSize int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	Keyword  string `form:"keyword" binding:"omitempty,max=50"`
	Status   *int8  `form:"status" binding:"omitempty,oneof=0 1"`
}

// GetDefaultPage 获取默认页码
func (r *ListUsersRequest) GetDefaultPage() int {
	if r.Page <= 0 {
		return 1
	}
	return r.Page
}

// GetDefaultPageSize 获取默认每页数量
func (r *ListUsersRequest) GetDefaultPageSize() int {
	if r.PageSize <= 0 {
		return 20
	}
	return r.PageSize
}

// ============ 响应 DTO ============

// LoginResponse 登录响应
//
// 当账号启用了 MFA 时，登录第一步只返回 RequiresMFA + MFAToken，
// 令牌字段为空；需再调用 /auth/mfa/verify 提交 TOTP 码换取正式令牌。
type LoginResponse struct {
	AccessToken  string       `json:"access_token,omitempty"`
	RefreshToken string       `json:"refresh_token,omitempty"`
	ExpiresIn    int64        `json:"expires_in,omitempty"`
	User         UserResponse `json:"user,omitzero"`
	// RequiresMFA 为 true 表示登录未完成，需要二次验证
	RequiresMFA bool `json:"requires_mfa,omitempty"`
	// MFAToken 短期挑战令牌（5 分钟），仅能用于 /auth/mfa/verify
	MFAToken string `json:"mfa_token,omitempty"`
}

// UserResponse 用户信息响应
type UserResponse struct {
	ID             uint64     `json:"id"`
	Username       string     `json:"username"`
	Email          string     `json:"email"`
	DisplayName    string     `json:"display_name"`
	AvatarURL      string     `json:"avatar_url"`
	Status         int8       `json:"status"`
	Roles          []string   `json:"roles,omitempty"`
	MFAEnabled     bool       `json:"mfa_enabled"`
	AuthSource     string     `json:"auth_source"`            // "local" 或 "sso"
	SSOProvider    string     `json:"sso_provider,omitempty"` // SSO 提供方名称（仅 SSO 用户）
	LarkOpenID     string     `json:"lark_open_id,omitempty"`
	TelegramUserID string     `json:"telegram_user_id,omitempty"`
	Locale         string     `json:"locale,omitempty"`
	LastLoginAt    *time.Time `json:"last_login_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// RoleResponse 角色信息响应
type RoleResponse struct {
	ID          uint64 `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
}

// ============ MFA DTO ============

// MFASetupResponse MFA 设置响应
type MFASetupResponse struct {
	Secret     string `json:"secret"`       // Base32 编码的密钥
	OTPAuthURL string `json:"otp_auth_url"` // 用于生成二维码的 URL
	QRCodeData string `json:"qr_code_data"` // Base64 编码的 QR 码图片
	Issuer     string `json:"issuer"`       // 发行者
	Account    string `json:"account"`      // 账户名
}

// MFAStatusResponse MFA 状态响应
type MFAStatusResponse struct {
	Enabled    bool       `json:"enabled"`
	VerifiedAt *time.Time `json:"verified_at,omitempty"`
}

// MFAVerifyRequest MFA 验证请求
type MFAVerifyRequest struct {
	Code string `json:"code" binding:"required,len=6"`
}

// MFALoginRequest MFA 登录验证请求
//
// 刻意不接受 user_id：那样任何人只要知道用户 ID 就能绕过密码这一步。
// 必须携带登录第一步下发的挑战令牌，它本身即是「密码已通过」的凭证。
type MFALoginRequest struct {
	MFAToken string `json:"mfa_token" binding:"required"`
	Code     string `json:"code" binding:"required,len=6"`
}

// MFALoginResponse MFA 登录响应（需要 MFA 验证时返回）
type MFALoginResponse struct {
	RequiresMFA bool   `json:"requires_mfa"`
	UserID      uint64 `json:"user_id,omitempty"`
	Message     string `json:"message,omitempty"`
}
