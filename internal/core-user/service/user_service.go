// Package service 提供用户业务逻辑层
package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/kerbos/ticketdesk/internal/core-user/dto"
	"github.com/kerbos/ticketdesk/internal/core-user/repository"
	"github.com/kerbos/ticketdesk/internal/model"
	"github.com/kerbos/ticketdesk/internal/notification/email"
	configService "github.com/kerbos/ticketdesk/internal/system-config/service"
	"github.com/kerbos/ticketdesk/pkg/i18n"
	"github.com/kerbos/ticketdesk/pkg/jwt"
	"github.com/kerbos/ticketdesk/pkg/logger"
)

// 业务错误定义
var (
	ErrUserNotFound          = errors.New("user.not_found")
	ErrUserDisabled          = errors.New("user.disabled")
	ErrUsernameExists        = errors.New("user.username_exists")
	ErrEmailExists           = errors.New("user.email_exists")
	ErrInvalidCredentials    = errors.New("user.bad_credentials")
	ErrInvalidOldPassword    = errors.New("user.old_password_wrong")
	ErrInvalidResetToken     = errors.New("user.reset_token_bad")
	ErrResetTokenExpired     = errors.New("user.reset_token_expired")
	ErrSSOPasswordNotAllowed = errors.New("user.sso_no_password")
	ErrAccountUsesSSOLogin   = errors.New("user.sso_bound")
	ErrInvalidMFAToken       = errors.New("user.mfa_token_invalid")
	ErrTokenRevoked          = errors.New("user.session_expired")
)

// UserService 用户服务接口
type UserService interface {
	Register(ctx context.Context, req *dto.RegisterRequest) (*dto.UserResponse, error)
	Login(ctx context.Context, req *dto.LoginRequest) (*dto.LoginResponse, error)
	RefreshToken(ctx context.Context, refreshToken string) (*dto.LoginResponse, error)
	// CompleteMFALogin 用 MFA 挑战令牌 + TOTP 码换取正式令牌对
	CompleteMFALogin(ctx context.Context, mfaToken, code string) (*dto.LoginResponse, error)
	// SetMFAVerifier 注入 MFA 校验器
	SetMFAVerifier(v MFAVerifier)
	// AuthStateProvider 供认证中间件校验账号状态与令牌版本
	AuthStateProvider
	GetCurrentUser(ctx context.Context, userID uint64) (*dto.UserResponse, error)
	GetUser(ctx context.Context, id uint64) (*dto.UserResponse, error)
	CreateUser(ctx context.Context, req *dto.CreateUserRequest) (*dto.UserResponse, error)
	UpdateUser(ctx context.Context, id uint64, req *dto.UpdateUserRequest) (*dto.UserResponse, error)
	UpdatePassword(ctx context.Context, userID uint64, req *dto.UpdatePasswordRequest) error
	ResetPassword(ctx context.Context, id uint64, req *dto.ResetPasswordRequest) error
	EnableUser(ctx context.Context, id uint64) error
	DisableUser(ctx context.Context, id uint64) error
	DeleteUser(ctx context.Context, id uint64) error
	ListUsers(ctx context.Context, req *dto.ListUsersRequest) ([]*dto.UserResponse, int64, error)
	// 忘记密码相关方法
	ForgotPassword(ctx context.Context, req *dto.ForgotPasswordRequest) error
	VerifyResetToken(ctx context.Context, token string) error
	ResetPasswordWithToken(ctx context.Context, req *dto.ResetPasswordWithTokenRequest) error
}

// userService 用户服务实现
type userService struct {
	userRepo      repository.UserRepository
	userRoleRepo  repository.UserRoleRepository
	jwtManager    *jwt.Manager
	emailService  email.EmailService
	configService configService.ConfigService
	db            *gorm.DB // 用于级联删除事务
	// mfaVerifier 校验 TOTP 码；由 SetMFAVerifier 注入，避免与 MFAService 构造顺序耦合
	mfaVerifier MFAVerifier
}

// MFAVerifier 校验用户的 TOTP 码（由 MFAService 实现）
type MFAVerifier interface {
	VerifyMFA(ctx context.Context, userID uint64, code string) error
}

// SetMFAVerifier 注入 MFA 校验器
func (s *userService) SetMFAVerifier(v MFAVerifier) {
	s.mfaVerifier = v
}

// NewUserService 创建用户服务实例
func NewUserService(
	userRepo repository.UserRepository,
	userRoleRepo repository.UserRoleRepository,
	jwtManager *jwt.Manager,
	emailService email.EmailService,
	configService configService.ConfigService,
	db *gorm.DB,
) UserService {
	return &userService{
		userRepo:      userRepo,
		userRoleRepo:  userRoleRepo,
		jwtManager:    jwtManager,
		emailService:  emailService,
		configService: configService,
		db:            db,
	}
}

// Register 用户注册
func (s *userService) Register(ctx context.Context, req *dto.RegisterRequest) (*dto.UserResponse, error) {
	// 检查用户名是否已存在
	exists, err := s.userRepo.ExistsByUsername(ctx, req.Username)
	if err != nil {
		logger.Error("failed to check username existence", zap.Error(err))
		return nil, fmt.Errorf("检查用户名失败: %w", err)
	}
	if exists {
		return nil, ErrUsernameExists
	}

	// 检查邮箱是否已存在
	exists, err = s.userRepo.ExistsByEmail(ctx, req.Email)
	if err != nil {
		logger.Error("failed to check email existence", zap.Error(err))
		return nil, fmt.Errorf("检查邮箱失败: %w", err)
	}
	if exists {
		return nil, ErrEmailExists
	}

	// 密码加密
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		logger.Error("failed to hash password", zap.Error(err))
		return nil, fmt.Errorf("密码加密失败: %w", err)
	}

	// 创建用户
	user := &model.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		DisplayName:  req.DisplayName,
		Status:       1, // 默认启用
	}

	if user.DisplayName == "" {
		user.DisplayName = user.Username
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		logger.Error("failed to create user", zap.Error(err))
		return nil, fmt.Errorf("创建用户失败: %w", err)
	}

	// 分配默认角色
	if err := s.userRoleRepo.AssignRole(ctx, user.ID, "user"); err != nil {
		logger.Warn("failed to assign default role", zap.Uint64("user_id", user.ID), zap.Error(err))
	}

	logger.Info("user registered successfully",
		zap.Uint64("user_id", user.ID),
		zap.String("username", user.Username),
	)

	return s.toUserResponse(ctx, user), nil
}

// dummyPasswordHash 用户不存在时用于「空跑」一次 bcrypt 的固定摘要。
//
// 若查无此人就直接返回，响应时间会明显短于命中用户的分支（bcrypt 要几十毫秒），
// 攻击者据此即可批量枚举系统里有哪些用户名。这里让两条路径都付出同样的计算代价。
// 该摘要对应一个随机口令，不可能被匹配上。
const dummyPasswordHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

// Login 用户登录
//
// MFA 已启用时不直接下发令牌，改为下发短期挑战令牌，
// 由 /auth/mfa/verify 校验 TOTP 后再换取正式令牌对。
func (s *userService) Login(ctx context.Context, req *dto.LoginRequest) (*dto.LoginResponse, error) {
	// 查找用户
	user, err := s.userRepo.GetByUsername(ctx, req.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 空跑一次 bcrypt 抹平时间差，避免用户名枚举
			_ = bcrypt.CompareHashAndPassword([]byte(dummyPasswordHash), []byte(req.Password))
			return nil, ErrInvalidCredentials
		}
		logger.Error("failed to get user by username", zap.Error(err))
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	// 先验证密码，再暴露账号的任何状态信息。
	// 顺序很重要：若先返回「账号已禁用」或「该账号走 SSO」，
	// 无需口令即可判定用户名是否存在。
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	// 检查用户状态
	if user.Status == 0 {
		return nil, ErrUserDisabled
	}

	// 已绑定 SSO 的账号禁止本地密码登录，强制走 SSO
	// 避免「升级为 SSO 用户后仍可用旧密码绕过」的双轨问题
	if user.SSOProvider != "" {
		return nil, ErrAccountUsesSSOLogin
	}

	// 记录最后登录时间（只更新该列，避免整行回写覆盖并发修改）
	s.touchLastLogin(ctx, user.ID)

	// MFA 已启用：只发挑战令牌，此时尚未完成认证
	if user.MFAEnabled {
		challenge, err := s.jwtManager.GenerateMFAChallengeToken(user.ID, user.Username, user.TokenVersion)
		if err != nil {
			logger.Error("failed to generate mfa challenge token", zap.Error(err))
			return nil, fmt.Errorf("生成 MFA 挑战令牌失败: %w", err)
		}
		logger.Info("login requires mfa", zap.Uint64("user_id", user.ID))
		return &dto.LoginResponse{
			RequiresMFA: true,
			MFAToken:    challenge,
		}, nil
	}

	return s.issueLoginResponse(ctx, user)
}

// touchLastLogin 更新最后登录时间
func (s *userService) touchLastLogin(ctx context.Context, userID uint64) {
	if err := s.db.WithContext(ctx).Model(&model.User{}).
		Where("id = ?", userID).
		UpdateColumn("last_login_at", time.Now()).Error; err != nil {
		logger.Warn("failed to update last login time", zap.Uint64("user_id", userID), zap.Error(err))
	}
}

// issueLoginResponse 签发正式令牌对
func (s *userService) issueLoginResponse(ctx context.Context, user *model.User) (*dto.LoginResponse, error) {
	accessToken, err := s.jwtManager.GenerateAccessToken(user.ID, user.Username, user.TokenVersion)
	if err != nil {
		logger.Error("failed to generate access token", zap.Error(err))
		return nil, fmt.Errorf("生成 Token 失败: %w", err)
	}

	refreshToken, err := s.jwtManager.GenerateRefreshToken(user.ID, user.Username, user.TokenVersion)
	if err != nil {
		logger.Error("failed to generate refresh token", zap.Error(err))
		return nil, fmt.Errorf("生成 Refresh Token 失败: %w", err)
	}

	logger.Info("user logged in successfully",
		zap.Uint64("user_id", user.ID),
		zap.String("username", user.Username),
	)

	return &dto.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    s.jwtManager.AccessExpireSeconds(),
		User:         *s.toUserResponse(ctx, user),
	}, nil
}

// CompleteMFALogin 校验 MFA 挑战令牌 + TOTP 码，成功后签发正式令牌对
func (s *userService) CompleteMFALogin(ctx context.Context, mfaToken, code string) (*dto.LoginResponse, error) {
	// 必须是 MFA 挑战令牌：access / refresh 都不能拿来跳过这一步
	claims, err := s.jwtManager.ParseTokenOfType(mfaToken, jwt.TokenTypeMFA)
	if err != nil {
		return nil, ErrInvalidMFAToken
	}

	user, err := s.userRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	// 挑战令牌签发后用户被禁用 / 改了密码，则作废
	if user.Status == 0 {
		return nil, ErrUserDisabled
	}
	if claims.TokenVersion != user.TokenVersion {
		return nil, ErrInvalidMFAToken
	}

	if err := s.mfaVerifier.VerifyMFA(ctx, user.ID, code); err != nil {
		return nil, err
	}

	return s.issueLoginResponse(ctx, user)
}

// RefreshToken 刷新 Token
func (s *userService) RefreshToken(ctx context.Context, refreshToken string) (*dto.LoginResponse, error) {
	// 必须是 refresh 类型：access token 不得用于续期，
	// 否则偷到一个短期访问令牌就能无限换新，等同永久有效
	claims, err := s.jwtManager.ParseTokenOfType(refreshToken, jwt.TokenTypeRefresh)
	if err != nil {
		return nil, fmt.Errorf("无效的 Refresh Token: %w", err)
	}

	// 查找用户
	user, err := s.userRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	// 检查用户状态
	if user.Status == 0 {
		return nil, ErrUserDisabled
	}

	// 令牌版本不匹配说明期间改过密码或被强制登出
	if claims.TokenVersion != user.TokenVersion {
		return nil, ErrTokenRevoked
	}

	return s.issueLoginResponse(ctx, user)
}

// GetCurrentUser 获取当前用户信息
func (s *userService) GetCurrentUser(ctx context.Context, userID uint64) (*dto.UserResponse, error) {
	return s.GetUser(ctx, userID)
}

// GetUser 获取用户详情
func (s *userService) GetUser(ctx context.Context, id uint64) (*dto.UserResponse, error) {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		logger.Error("failed to get user", zap.Uint64("id", id), zap.Error(err))
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	return s.toUserResponse(ctx, user), nil
}

// CreateUser 创建用户（管理员）
func (s *userService) CreateUser(ctx context.Context, req *dto.CreateUserRequest) (*dto.UserResponse, error) {
	// 检查用户名是否已存在
	exists, err := s.userRepo.ExistsByUsername(ctx, req.Username)
	if err != nil {
		logger.Error("failed to check username existence", zap.Error(err))
		return nil, fmt.Errorf("检查用户名失败: %w", err)
	}
	if exists {
		return nil, ErrUsernameExists
	}

	// 检查邮箱是否已存在
	exists, err = s.userRepo.ExistsByEmail(ctx, req.Email)
	if err != nil {
		logger.Error("failed to check email existence", zap.Error(err))
		return nil, fmt.Errorf("检查邮箱失败: %w", err)
	}
	if exists {
		return nil, ErrEmailExists
	}

	// 密码加密
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		logger.Error("failed to hash password", zap.Error(err))
		return nil, fmt.Errorf("密码加密失败: %w", err)
	}

	// 设置默认状态
	status := int8(1) // 默认启用
	if req.Status != nil {
		status = *req.Status
	}

	// 创建用户
	user := &model.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		DisplayName:  req.DisplayName,
		Status:       status,
	}

	if user.DisplayName == "" {
		user.DisplayName = user.Username
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		logger.Error("failed to create user", zap.Error(err))
		return nil, fmt.Errorf("创建用户失败: %w", err)
	}

	// 分配角色
	if len(req.Roles) > 0 {
		for _, roleName := range req.Roles {
			if err := s.userRoleRepo.AssignRole(ctx, user.ID, roleName); err != nil {
				logger.Warn("failed to assign role",
					zap.Uint64("user_id", user.ID),
					zap.String("role", roleName),
					zap.Error(err),
				)
			}
		}
	} else {
		// 如果没有指定角色，分配默认角色
		if err := s.userRoleRepo.AssignRole(ctx, user.ID, "user"); err != nil {
			logger.Warn("failed to assign default role", zap.Uint64("user_id", user.ID), zap.Error(err))
		}
	}

	logger.Info("user created by admin",
		zap.Uint64("user_id", user.ID),
		zap.String("username", user.Username),
	)

	return s.toUserResponse(ctx, user), nil
}

// UpdateUser 更新用户信息
func (s *userService) UpdateUser(ctx context.Context, id uint64, req *dto.UpdateUserRequest) (*dto.UserResponse, error) {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	// 更新字段
	if req.DisplayName != nil {
		user.DisplayName = *req.DisplayName
	}
	if req.AvatarURL != nil {
		user.AvatarURL = *req.AvatarURL
	}
	if req.Email != nil && *req.Email != user.Email {
		// 检查邮箱是否已被使用
		exists, err := s.userRepo.ExistsByEmail(ctx, *req.Email)
		if err != nil {
			return nil, fmt.Errorf("检查邮箱失败: %w", err)
		}
		if exists {
			return nil, ErrEmailExists
		}
		user.Email = *req.Email
	}
	if req.LarkOpenID != nil {
		user.LarkOpenID = *req.LarkOpenID
	}
	if req.TelegramUserID != nil {
		user.TelegramUserID = *req.TelegramUserID
	}
	if req.Locale != nil {
		user.Locale = *req.Locale
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		logger.Error("failed to update user", zap.Uint64("id", id), zap.Error(err))
		return nil, fmt.Errorf("更新用户失败: %w", err)
	}

	logger.Info("user updated successfully", zap.Uint64("user_id", id))

	return s.toUserResponse(ctx, user), nil
}

// UpdatePassword 修改密码
func (s *userService) UpdatePassword(ctx context.Context, userID uint64, req *dto.UpdatePasswordRequest) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("查询用户失败: %w", err)
	}

	// SSO 用户不允许修改密码
	if user.SSOProvider != "" {
		return ErrSSOPasswordNotAllowed
	}

	// 验证原密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword)); err != nil {
		return ErrInvalidOldPassword
	}

	// 加密新密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("密码加密失败: %w", err)
	}

	user.PasswordHash = string(hashedPassword)
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("更新密码失败: %w", err)
	}

	// 改密后必须踢掉所有存量会话，否则旧令牌仍可用到自然过期
	if err := s.bumpTokenVersion(ctx, userID); err != nil {
		logger.Error("failed to bump token version after password change",
			zap.Uint64("user_id", userID), zap.Error(err))
	}

	logger.Info("user password updated successfully", zap.Uint64("user_id", userID))

	return nil
}

// ResetPassword 重置密码（管理员）
func (s *userService) ResetPassword(ctx context.Context, id uint64, req *dto.ResetPasswordRequest) error {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("查询用户失败: %w", err)
	}

	// 加密新密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("密码加密失败: %w", err)
	}

	user.PasswordHash = string(hashedPassword)
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("重置密码失败: %w", err)
	}

	// 管理员重置密码同样要让该用户的存量令牌立即失效
	if err := s.bumpTokenVersion(ctx, id); err != nil {
		logger.Error("failed to bump token version after admin reset",
			zap.Uint64("user_id", id), zap.Error(err))
	}

	logger.Info("user password reset by admin", zap.Uint64("user_id", id))

	return nil
}

// EnableUser 启用用户
func (s *userService) EnableUser(ctx context.Context, id uint64) error {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("查询用户失败: %w", err)
	}

	if user.Status == 1 {
		return nil // 已经是启用状态
	}

	user.Status = 1
	if err := s.userRepo.Update(ctx, user); err != nil {
		logger.Error("failed to enable user", zap.Uint64("id", id), zap.Error(err))
		return fmt.Errorf("启用用户失败: %w", err)
	}

	s.InvalidateAuthState(ctx, id)

	logger.Info("user enabled successfully", zap.Uint64("user_id", id))

	return nil
}

// DisableUser 禁用用户
func (s *userService) DisableUser(ctx context.Context, id uint64) error {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("查询用户失败: %w", err)
	}

	if user.Status == 0 {
		return nil // 已经是禁用状态
	}

	user.Status = 0
	if err := s.userRepo.Update(ctx, user); err != nil {
		logger.Error("failed to disable user", zap.Uint64("id", id), zap.Error(err))
		return fmt.Errorf("禁用用户失败: %w", err)
	}

	// 立即失效缓存，让认证中间件下一次请求就拦住该用户
	s.InvalidateAuthState(ctx, id)

	logger.Info("user disabled successfully", zap.Uint64("user_id", id))

	return nil
}

// DeleteUser 硬删除用户及关联数据（不删除用户创建的工单/评论，保留为孤立记录）
func (s *userService) DeleteUser(ctx context.Context, id uint64) error {
	// 检查用户是否存在
	_, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("查询用户失败: %w", err)
	}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 删除用户角色关联
		if err := tx.Unscoped().Where("user_id = ?", id).Delete(&model.UserRole{}).Error; err != nil {
			return fmt.Errorf("删除用户角色失败: %w", err)
		}

		// 2. 删除项目成员关系
		if err := tx.Unscoped().Where("user_id = ?", id).Delete(&model.ProjectMember{}).Error; err != nil {
			return fmt.Errorf("删除项目成员失败: %w", err)
		}

		// 3. 删除项目角色成员关系
		if err := tx.Unscoped().Where("user_id = ?", id).Delete(&model.ProjectRoleMember{}).Error; err != nil {
			return fmt.Errorf("删除项目角色成员失败: %w", err)
		}

		// 4. 删除站内通知
		if err := tx.Unscoped().Where("user_id = ?", id).Delete(&model.Notification{}).Error; err != nil {
			return fmt.Errorf("删除通知失败: %w", err)
		}

		// 5. 工单：nullable assignee 置 NULL；reporter NOT NULL 但无 FK，悬挂引用前端做兜底
		if err := tx.Exec("UPDATE issues SET assignee_id = NULL WHERE assignee_id = ?", id).Error; err != nil {
			return fmt.Errorf("解绑工单指派人失败: %w", err)
		}

		// 6. 该用户产生的工单内容：评论/附件/关注/工时/审批，全删
		if err := tx.Unscoped().Where("user_id = ?", id).Delete(&model.IssueComment{}).Error; err != nil {
			return fmt.Errorf("删除工单评论失败: %w", err)
		}
		if err := tx.Unscoped().Where("uploaded_by = ?", id).Delete(&model.IssueAttachment{}).Error; err != nil {
			return fmt.Errorf("删除工单附件失败: %w", err)
		}
		if err := tx.Unscoped().Where("user_id = ?", id).Delete(&model.IssueWatcher{}).Error; err != nil {
			return fmt.Errorf("删除工单关注关系失败: %w", err)
		}
		if err := tx.Unscoped().Where("user_id = ?", id).Delete(&model.IssueWorklog{}).Error; err != nil {
			return fmt.Errorf("删除工单工时失败: %w", err)
		}
		if err := tx.Unscoped().Where("approver_id = ?", id).Delete(&model.ApprovalRecord{}).Error; err != nil {
			return fmt.Errorf("删除审批记录失败: %w", err)
		}

		// 7. 需求：assignee/reporter 置 NULL，created_by NOT NULL 且有 FK 约束 → 转给系统管理员（id=1）保留历史
		if err := tx.Exec("UPDATE requirements SET assignee_id = NULL WHERE assignee_id = ?", id).Error; err != nil {
			return fmt.Errorf("解绑需求指派人失败: %w", err)
		}
		if err := tx.Exec("UPDATE requirements SET reporter_id = NULL WHERE reporter_id = ?", id).Error; err != nil {
			return fmt.Errorf("解绑需求报告人失败: %w", err)
		}
		// 不转移自己（避免 admin 被删时把 created_by 设回 admin 自身）
		if id != 1 {
			if err := tx.Exec("UPDATE requirements SET created_by = 1 WHERE created_by = ?", id).Error; err != nil {
				return fmt.Errorf("转移需求创建人失败: %w", err)
			}
		}

		// 8. 需求评论/附件：用户产生的内容，删
		if err := tx.Unscoped().Where("user_id = ?", id).Delete(&model.RequirementComment{}).Error; err != nil {
			return fmt.Errorf("删除需求评论失败: %w", err)
		}
		if err := tx.Unscoped().Where("uploaded_by = ?", id).Delete(&model.RequirementAttachment{}).Error; err != nil {
			return fmt.Errorf("删除需求附件失败: %w", err)
		}

		// 9. 项目元数据：通知渠道 created_by / 项目 lead_user_id 转给 admin
		if id != 1 {
			if err := tx.Exec("UPDATE project_notification_channels SET created_by = 1 WHERE created_by = ?", id).Error; err != nil {
				return fmt.Errorf("转移通知渠道创建人失败: %w", err)
			}
			if err := tx.Exec("UPDATE projects SET lead_user_id = 1 WHERE lead_user_id = ?", id).Error; err != nil {
				return fmt.Errorf("转移项目负责人失败: %w", err)
			}
		}

		// 10. API Token 全删
		if err := tx.Unscoped().Where("user_id = ?", id).Delete(&model.APIToken{}).Error; err != nil {
			return fmt.Errorf("删除 API Token 失败: %w", err)
		}

		// 11. 删除用户本身
		if err := tx.Unscoped().Delete(&model.User{}, id).Error; err != nil {
			return fmt.Errorf("删除用户失败: %w", err)
		}

		return nil
	}); err != nil {
		logger.Error("failed to cascade delete user", zap.Uint64("id", id), zap.Error(err))
		return fmt.Errorf("删除用户失败: %w", err)
	}

	logger.Info("user cascade deleted successfully", zap.Uint64("user_id", id))

	return nil
}

// ListUsers 分页查询用户列表
func (s *userService) ListUsers(ctx context.Context, req *dto.ListUsersRequest) ([]*dto.UserResponse, int64, error) {
	page := req.GetDefaultPage()
	pageSize := req.GetDefaultPageSize()
	offset := (page - 1) * pageSize

	users, total, err := s.userRepo.List(ctx, offset, pageSize, req.Keyword, req.Status)
	if err != nil {
		logger.Error("failed to list users", zap.Error(err))
		return nil, 0, fmt.Errorf("查询用户列表失败: %w", err)
	}

	// 批量取角色，避免每条记录单独查一次（一页 20 条 = 21 次查询）
	userIDs := make([]uint64, len(users))
	for i, u := range users {
		userIDs[i] = u.ID
	}
	rolesByUser, err := s.userRoleRepo.GetRoleNamesByUserIDs(ctx, userIDs)
	if err != nil {
		logger.Warn("failed to batch load user roles", zap.Error(err))
		rolesByUser = map[uint64][]string{}
	}

	responses := make([]*dto.UserResponse, len(users))
	for i, user := range users {
		responses[i] = s.toUserResponseWithRoles(user, rolesByUser[user.ID])
	}

	return responses, total, nil
}

// toUserResponse 将用户模型转换为响应 DTO
func (s *userService) toUserResponse(ctx context.Context, user *model.User) *dto.UserResponse {
	authSource := "local"
	if user.SSOProvider != "" {
		authSource = "sso"
	}

	resp := &dto.UserResponse{
		ID:             user.ID,
		Username:       user.Username,
		Email:          user.Email,
		DisplayName:    user.DisplayName,
		AvatarURL:      user.AvatarURL,
		Status:         user.Status,
		MFAEnabled:     user.MFAEnabled,
		AuthSource:     authSource,
		SSOProvider:    user.SSOProvider,
		LarkOpenID:     user.LarkOpenID,
		TelegramUserID: user.TelegramUserID,
		Locale:         user.Locale,
		LastLoginAt:    user.LastLoginAt,
		CreatedAt:      user.CreatedAt,
		UpdatedAt:      user.UpdatedAt,
	}

	// 获取用户角色
	roles, err := s.userRoleRepo.GetUserRoleNames(ctx, user.ID)
	if err != nil {
		logger.Warn("failed to get user roles", zap.Uint64("user_id", user.ID), zap.Error(err))
	} else {
		resp.Roles = roles
	}

	return resp
}

// toUserResponseWithRoles 用已批量取好的角色组装响应，不再单独查库
func (s *userService) toUserResponseWithRoles(user *model.User, roles []string) *dto.UserResponse {
	authSource := "local"
	if user.SSOProvider != "" {
		authSource = "sso"
	}

	return &dto.UserResponse{
		ID:             user.ID,
		Username:       user.Username,
		Email:          user.Email,
		DisplayName:    user.DisplayName,
		AvatarURL:      user.AvatarURL,
		Status:         user.Status,
		MFAEnabled:     user.MFAEnabled,
		AuthSource:     authSource,
		SSOProvider:    user.SSOProvider,
		LarkOpenID:     user.LarkOpenID,
		TelegramUserID: user.TelegramUserID,
		Locale:         user.Locale,
		LastLoginAt:    user.LastLoginAt,
		CreatedAt:      user.CreatedAt,
		UpdatedAt:      user.UpdatedAt,
		Roles:          roles,
	}
}

// hashResetToken 计算重置令牌的存储摘要
//
// 令牌本身只出现在发给用户的邮件链接里，数据库只保存摘要。
// 之前是明文入库：任何能读到 users 表的人（备份、只读从库、SQL 注入）
// 都能直接拿它重置任意账号的密码。
// 这里用 SHA-256 而非 bcrypt：令牌是 32 字节的密码学随机值，
// 不存在被字典穷举的风险，且校验走的是索引等值查询，需要可确定性计算。
func hashResetToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// ForgotPassword 请求重置密码
func (s *userService) ForgotPassword(ctx context.Context, req *dto.ForgotPasswordRequest) error {
	// 查找用户
	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 为了安全，即使用户不存在也返回成功，不泄露用户信息
			logger.Info("forgot password request for non-existent email", zap.String("email", req.Email))
			return nil
		}
		logger.Error("failed to get user by email", zap.Error(err))
		return fmt.Errorf("查询用户失败: %w", err)
	}

	// 检查用户状态
	if user.Status == 0 {
		// 禁用的用户也返回成功，不泄露信息
		logger.Info("forgot password request for disabled user", zap.Uint64("user_id", user.ID))
		return nil
	}

	// 生成重置密码令牌（32字节随机字符串）
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		logger.Error("failed to generate reset token", zap.Error(err))
		return fmt.Errorf("生成重置令牌失败: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	// 设置令牌过期时间（30分钟）
	expiresAt := time.Now().Add(30 * time.Minute)
	// 入库的是摘要，明文仅通过邮件发给用户本人
	user.ResetPasswordToken = hashResetToken(token)
	user.ResetPasswordExpires = &expiresAt

	// 保存令牌
	if err := s.userRepo.Update(ctx, user); err != nil {
		logger.Error("failed to save reset token", zap.Error(err))
		return fmt.Errorf("保存重置令牌失败: %w", err)
	}

	// 发送重置密码邮件
	if err := s.sendResetPasswordEmail(ctx, user, token); err != nil {
		logger.Error("failed to send reset password email",
			zap.Uint64("user_id", user.ID),
			zap.String("email", user.Email),
			zap.Error(err),
		)
		// 邮件发送失败不影响流程，返回成功
	}

	logger.Info("reset password email sent",
		zap.Uint64("user_id", user.ID),
		zap.String("email", user.Email),
	)

	return nil
}

// VerifyResetToken 验证重置密码令牌
func (s *userService) VerifyResetToken(ctx context.Context, token string) error {
	// 查找用户
	user, err := s.userRepo.GetByResetToken(ctx, hashResetToken(token))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrInvalidResetToken
		}
		return fmt.Errorf("查询用户失败: %w", err)
	}

	// 检查令牌是否过期
	if user.ResetPasswordExpires == nil || time.Now().After(*user.ResetPasswordExpires) {
		return ErrResetTokenExpired
	}

	return nil
}

// ResetPasswordWithToken 使用令牌重置密码
func (s *userService) ResetPasswordWithToken(ctx context.Context, req *dto.ResetPasswordWithTokenRequest) error {
	// 查找用户
	user, err := s.userRepo.GetByResetToken(ctx, hashResetToken(req.Token))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrInvalidResetToken
		}
		return fmt.Errorf("查询用户失败: %w", err)
	}

	// 检查令牌是否过期
	if user.ResetPasswordExpires == nil || time.Now().After(*user.ResetPasswordExpires) {
		return ErrResetTokenExpired
	}

	// 加密新密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("密码加密失败: %w", err)
	}

	// 更新密码并清除令牌
	user.PasswordHash = string(hashedPassword)
	user.ResetPasswordToken = ""
	user.ResetPasswordExpires = nil

	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("更新密码失败: %w", err)
	}

	// 通过邮件重置密码同样要让存量会话全部失效
	if err := s.bumpTokenVersion(ctx, user.ID); err != nil {
		logger.Error("failed to bump token version after reset",
			zap.Uint64("user_id", user.ID), zap.Error(err))
	}

	logger.Info("password reset successfully", zap.Uint64("user_id", user.ID))

	return nil
}

// sendResetPasswordEmail 发送重置密码邮件
func (s *userService) sendResetPasswordEmail(ctx context.Context, user *model.User, token string) error {
	// 从配置中读取站点域名
	siteURL, err := s.configService.GetConfigValue(ctx, configService.KeyGeneralSiteURL)
	if err != nil || siteURL == "" {
		// 如果没有配置站点域名，使用默认值
		siteURL = "http://localhost:5173"
		logger.Warn("site URL not configured, using default",
			zap.String("default_url", siteURL),
		)
	}

	// 构建重置链接
	resetURL := fmt.Sprintf("%s/reset-password?token=%s", siteURL, token)

	// 邮件发给具体的人，语言跟着收件人走；user.Locale 为空时回落到站点语言
	lang := user.Locale
	subject := i18n.U(lang, "mail.reset_subject")
	htmlBody := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: linear-gradient(135deg, #3b82f6 0%%, #8b5cf6 100%%); color: white; padding: 30px; text-align: center; border-radius: 8px 8px 0 0; }
        .content { background: #f8fafc; padding: 30px; border-radius: 0 0 8px 8px; }
        .button { display: inline-block; padding: 12px 30px; background: linear-gradient(135deg, #3b82f6 0%%, #8b5cf6 100%%); color: white; text-decoration: none; border-radius: 6px; margin: 20px 0; }
        .footer { text-align: center; margin-top: 20px; color: #6b7280; font-size: 14px; }
        .warning { background: #fef3c7; border-left: 4px solid #f59e0b; padding: 12px; margin: 20px 0; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>%s</h1>
        </div>
        <div class="content">
            <p>%s</p>
            <p>%s</p>
            <p style="text-align: center;">
                <a href="%s" class="button">%s</a>
            </p>
            <p>%s</p>
            <p style="word-break: break-all; background: white; padding: 10px; border-radius: 4px;">%s</p>
            <div class="warning">
                <strong>⚠️ %s</strong>
                <ul style="margin: 10px 0;">
                    <li>%s</li>
                    <li>%s</li>
                    <li>%s</li>
                </ul>
            </div>
        </div>
        <div class="footer">
            <p>&copy; 2026 TicketDesk. All rights reserved.</p>
        </div>
    </div>
</body>
</html>
`,
		i18n.U(lang, "mail.reset_title"),
		i18n.Uf(lang, "mail.reset_greeting", user.DisplayName),
		i18n.U(lang, "mail.reset_intro"),
		resetURL,
		i18n.U(lang, "mail.reset_button"),
		i18n.U(lang, "mail.reset_or_copy"),
		resetURL,
		i18n.U(lang, "mail.reset_warning"),
		i18n.U(lang, "mail.reset_expiry"),
		i18n.U(lang, "mail.reset_ignore"),
		i18n.U(lang, "mail.reset_private"),
	)

	return s.emailService.SendHTMLEmail(ctx, []string{user.Email}, subject, htmlBody)
}
