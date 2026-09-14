// Package service 提供用户业务逻辑层
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"gorm.io/gorm"

	"github.com/kerbos/ticketdesk/internal/core-user/dto"
	"github.com/kerbos/ticketdesk/internal/core-user/repository"
	"github.com/kerbos/ticketdesk/internal/model"
	configDto "github.com/kerbos/ticketdesk/internal/system-config/dto"
	configService "github.com/kerbos/ticketdesk/internal/system-config/service"
	"github.com/kerbos/ticketdesk/pkg/jwt"
	"github.com/kerbos/ticketdesk/pkg/logger"
	pkgRedis "github.com/kerbos/ticketdesk/pkg/redis"
)

// SSO 相关错误定义
var (
	ErrSSODisabled       = errors.New("user.sso_disabled")
	ErrSSOInvalidState   = errors.New("user.sso_state_invalid")
	ErrSSOCodeExchange   = errors.New("user.sso_code_failed")
	ErrSSOTokenVerify    = errors.New("user.sso_id_token_failed")
	ErrSSOUserDisabled   = errors.New("user.disabled")
	ErrSSOUserNotAllowed = errors.New("user.login_not_allowed")
	ErrSSONonceMismatch  = errors.New("user.sso_nonce_mismatch")
)

// ssoConfigSnapshot 从数据库读取的 SSO 配置快照
type ssoConfigSnapshot struct {
	Enabled        bool
	ProviderName   string
	ClientID       string
	ClientSecret   string
	IssuerURL      string
	RedirectURI    string
	Scopes         []string
	AutoCreateUser bool
	DefaultRole    string
	ClaimMappings  []configDto.SSOClaimMapping
}

// SSOService SSO 服务接口
type SSOService interface {
	GetSSOConfig(ctx context.Context) *dto.SSOConfigResponse
	GetAuthURL(ctx context.Context) (*dto.SSOAuthorizeResponse, error)
	HandleCallback(ctx context.Context, req *dto.SSOCallbackRequest) (*dto.LoginResponse, error)
}

// ssoService SSO 服务实现
type ssoService struct {
	configSvc    configService.ConfigService
	userRepo     repository.UserRepository
	userRoleRepo repository.UserRoleRepository
	jwtManager   *jwt.Manager

	// OIDC provider 缓存（按 issuerURL 缓存，配置变更时重建）
	mu           sync.Mutex
	cachedIssuer string
	provider     *gooidc.Provider
	oauth2Config *oauth2.Config
	verifier     *gooidc.IDTokenVerifier
}

// NewSSOService 创建 SSO 服务实例
func NewSSOService(
	configSvc configService.ConfigService,
	userRepo repository.UserRepository,
	userRoleRepo repository.UserRoleRepository,
	jwtManager *jwt.Manager,
) SSOService {
	return &ssoService{
		configSvc:    configSvc,
		userRepo:     userRepo,
		userRoleRepo: userRoleRepo,
		jwtManager:   jwtManager,
	}
}

// loadConfig 从数据库加载 SSO 配置
func (s *ssoService) loadConfig(ctx context.Context) (*ssoConfigSnapshot, error) {
	cfg, err := s.configSvc.GetSSOConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("加载 SSO 配置失败: %w", err)
	}

	// 解析 scopes
	var scopes []string
	if cfg.Scopes != "" {
		for _, scope := range strings.Split(cfg.Scopes, ",") {
			scope = strings.TrimSpace(scope)
			if scope != "" {
				scopes = append(scopes, scope)
			}
		}
	}
	if len(scopes) == 0 {
		scopes = []string{gooidc.ScopeOpenID, "profile", "email"}
	}

	// ClientSecret 需要从数据库直接读取（GetSSOConfig 会隐藏它）
	clientSecret, _ := s.configSvc.GetConfigValue(ctx, configService.KeySSOClientSecret)

	return &ssoConfigSnapshot{
		Enabled:        cfg.Enabled,
		ProviderName:   cfg.ProviderName,
		ClientID:       cfg.ClientID,
		ClientSecret:   clientSecret,
		IssuerURL:      cfg.IssuerURL,
		RedirectURI:    cfg.RedirectURI,
		Scopes:         scopes,
		AutoCreateUser: cfg.AutoCreateUser,
		DefaultRole:    cfg.DefaultRole,
		ClaimMappings:  cfg.ClaimMappings,
	}, nil
}

// ensureProvider 确保 OIDC provider 已初始化，配置变更时自动重建
func (s *ssoService) ensureProvider(ctx context.Context, cfg *ssoConfigSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 如果 issuerURL 没变且 provider 已存在，复用
	if s.provider != nil && s.cachedIssuer == cfg.IssuerURL {
		// 更新 oauth2Config 中可能变化的字段（clientID/secret/redirectURI/scopes）
		s.oauth2Config.ClientID = cfg.ClientID
		s.oauth2Config.ClientSecret = cfg.ClientSecret
		s.oauth2Config.RedirectURL = cfg.RedirectURI
		s.oauth2Config.Scopes = cfg.Scopes
		return nil
	}

	// issuerURL 变了或首次初始化，重新 discovery
	provider, err := gooidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return fmt.Errorf("OIDC provider discovery failed: %w", err)
	}

	s.provider = provider
	s.cachedIssuer = cfg.IssuerURL
	s.oauth2Config = &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURI,
		Endpoint:     provider.Endpoint(),
		Scopes:       cfg.Scopes,
	}
	s.verifier = provider.Verifier(&gooidc.Config{
		ClientID: cfg.ClientID,
	})

	logger.Info("OIDC provider initialized",
		zap.String("issuer_url", cfg.IssuerURL),
	)

	return nil
}

// GetSSOConfig 获取 SSO 配置（前端展示用）
func (s *ssoService) GetSSOConfig(ctx context.Context) *dto.SSOConfigResponse {
	cfg, err := s.loadConfig(ctx)
	if err != nil || !cfg.Enabled {
		return &dto.SSOConfigResponse{Enabled: false}
	}
	return &dto.SSOConfigResponse{
		Enabled:      true,
		ProviderName: cfg.ProviderName,
	}
}

// GetAuthURL 生成 SSO 授权 URL
func (s *ssoService) GetAuthURL(ctx context.Context) (*dto.SSOAuthorizeResponse, error) {
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return nil, err
	}
	if !cfg.Enabled {
		return nil, ErrSSODisabled
	}

	if err := s.ensureProvider(ctx, cfg); err != nil {
		return nil, fmt.Errorf("SSO 初始化失败: %w", err)
	}

	// 生成随机 state
	state, err := generateRandomString(32)
	if err != nil {
		return nil, fmt.Errorf("生成 state 失败: %w", err)
	}

	// 生成随机 nonce
	nonce, err := generateRandomString(32)
	if err != nil {
		return nil, fmt.Errorf("生成 nonce 失败: %w", err)
	}

	// 存入 Redis（TTL 10 分钟）
	rdb := pkgRedis.GetClient()
	stateKey := fmt.Sprintf("sso:state:%s", state)
	nonceKey := fmt.Sprintf("sso:nonce:%s", state)

	if err := rdb.Set(ctx, stateKey, "1", 10*time.Minute).Err(); err != nil {
		return nil, fmt.Errorf("保存 state 到 Redis 失败: %w", err)
	}
	if err := rdb.Set(ctx, nonceKey, nonce, 10*time.Minute).Err(); err != nil {
		return nil, fmt.Errorf("保存 nonce 到 Redis 失败: %w", err)
	}

	authURL := s.oauth2Config.AuthCodeURL(state, gooidc.Nonce(nonce))

	return &dto.SSOAuthorizeResponse{
		AuthorizeURL: authURL,
	}, nil
}

// HandleCallback 处理 SSO 回调
func (s *ssoService) HandleCallback(ctx context.Context, req *dto.SSOCallbackRequest) (*dto.LoginResponse, error) {
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return nil, err
	}
	if !cfg.Enabled {
		return nil, ErrSSODisabled
	}

	if err := s.ensureProvider(ctx, cfg); err != nil {
		return nil, fmt.Errorf("SSO 初始化失败: %w", err)
	}

	// 1. 验证 state（一次性消费，防重放）
	rdb := pkgRedis.GetClient()
	stateKey := fmt.Sprintf("sso:state:%s", req.State)
	result, err := rdb.GetDel(ctx, stateKey).Result()
	if err != nil || result == "" {
		return nil, ErrSSOInvalidState
	}

	// 2. 获取 nonce
	nonceKey := fmt.Sprintf("sso:nonce:%s", req.State)
	expectedNonce, err := rdb.GetDel(ctx, nonceKey).Result()
	if err != nil || expectedNonce == "" {
		return nil, ErrSSOInvalidState
	}

	// 3. 用 code 换 tokens
	oauth2Token, err := s.oauth2Config.Exchange(ctx, req.Code)
	if err != nil {
		logger.Error("SSO code exchange failed", zap.Error(err))
		return nil, ErrSSOCodeExchange
	}

	// 4. 提取并验证 ID Token
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		logger.Error("SSO response missing id_token")
		return nil, ErrSSOTokenVerify
	}

	idToken, err := s.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		logger.Error("SSO ID Token verification failed", zap.Error(err))
		return nil, ErrSSOTokenVerify
	}

	// 5. 验证 nonce
	if idToken.Nonce != expectedNonce {
		logger.Error("SSO nonce mismatch")
		return nil, ErrSSONonceMismatch
	}

	// 6. 提取 claims（内置字段 + 扩展属性）
	userInfo, extraAttrs, err := s.extractUserInfo(idToken, cfg)
	if err != nil {
		return nil, fmt.Errorf("提取用户信息失败: %w", err)
	}

	// 7. 匹配或创建用户
	user, err := s.matchOrCreateUser(ctx, userInfo, cfg)
	if err != nil {
		return nil, err
	}

	// 8. 检查用户状态
	if user.Status == 0 {
		return nil, ErrSSOUserDisabled
	}

	// 9. 更新登录信息 + 扩展属性
	now := time.Now()
	user.LastLoginAt = &now
	if userInfo.DisplayName != "" {
		user.DisplayName = userInfo.DisplayName
	}
	if userInfo.AvatarURL != "" {
		user.AvatarURL = userInfo.AvatarURL
	}
	if len(extraAttrs) > 0 {
		extraJSON, err := json.Marshal(extraAttrs)
		if err == nil {
			user.ExtraAttributes = string(extraJSON)
		}
	}
	if err := s.userRepo.Update(ctx, user); err != nil {
		// 之前这里只 warn 不 return，导致 Save 失败时用户拿到 token 但 DB 没写入登录状态，
		// 表现为「SSO 登录成功但仍显本地用户 / 从未登录」。改为返回错误，让前端立刻看到具体 mysql 错误
		logger.Error("failed to update SSO user info",
			zap.Uint64("user_id", user.ID),
			zap.String("username", user.Username),
			zap.Error(err),
		)
		return nil, fmt.Errorf("更新 SSO 用户登录信息失败: %w", err)
	}

	// 10. 生成 TicketDesk JWT
	accessToken, err := s.jwtManager.GenerateAccessToken(user.ID, user.Username, user.TokenVersion)
	if err != nil {
		return nil, fmt.Errorf("生成 Access Token 失败: %w", err)
	}

	refreshToken, err := s.jwtManager.GenerateRefreshToken(user.ID, user.Username, user.TokenVersion)
	if err != nil {
		return nil, fmt.Errorf("生成 Refresh Token 失败: %w", err)
	}

	logger.Info("SSO login successful",
		zap.Uint64("user_id", user.ID),
		zap.String("username", user.Username),
		zap.String("sso_subject", userInfo.Subject),
	)

	// 获取用户角色
	roles, _ := s.userRoleRepo.GetUserRoleNames(ctx, user.ID)

	return &dto.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    7200,
		User: dto.UserResponse{
			ID:          user.ID,
			Username:    user.Username,
			Email:       user.Email,
			DisplayName: user.DisplayName,
			AvatarURL:   user.AvatarURL,
			Status:      user.Status,
			MFAEnabled:  user.MFAEnabled,
			AuthSource:  "sso",
			SSOProvider: user.SSOProvider,
			LastLoginAt: user.LastLoginAt,
			Roles:       roles,
			CreatedAt:   user.CreatedAt,
			UpdatedAt:   user.UpdatedAt,
		},
	}, nil
}

// extractUserInfo 从 ID Token 提取用户信息（支持动态 claim 映射）
func (s *ssoService) extractUserInfo(idToken *gooidc.IDToken, cfg *ssoConfigSnapshot) (*dto.SSOUserInfo, map[string]string, error) {
	var claims map[string]interface{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, nil, fmt.Errorf("解析 ID Token claims 失败: %w", err)
	}

	info := &dto.SSOUserInfo{
		Subject: idToken.Subject,
	}
	extraAttrs := make(map[string]string)

	// 遍历动态 claim 映射
	for _, m := range cfg.ClaimMappings {
		val := getStringClaim(claims, m.ClaimName)
		if val == "" {
			continue
		}
		switch m.LocalField {
		case "username":
			info.Username = val
		case "email":
			info.Email = val
		case "display_name":
			info.DisplayName = val
		case "avatar":
			info.AvatarURL = val
		default:
			// 自定义扩展属性
			extraAttrs[m.LocalField] = val
		}
	}

	if info.Username == "" && info.Email == "" {
		return nil, nil, fmt.Errorf("SSO 用户信息不完整: username 和 email 均为空")
	}

	if info.Username == "" {
		info.Username = info.Email
	}

	return info, extraAttrs, nil
}

// matchOrCreateUser 匹配已有用户或创建新用户
func (s *ssoService) matchOrCreateUser(ctx context.Context, info *dto.SSOUserInfo, cfg *ssoConfigSnapshot) (*model.User, error) {
	providerName := cfg.ProviderName
	if providerName == "" {
		providerName = "sso"
	}

	// 优先级 1: SSO Subject 精确匹配
	user, err := s.userRepo.GetBySSOSubject(ctx, providerName, info.Subject)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("查询 SSO 用户失败: %w", err)
	}

	// 优先级 2: Email 匹配
	if info.Email != "" {
		user, err = s.userRepo.GetByEmail(ctx, info.Email)
		if err == nil {
			if err := s.linkLocalUserToSSO(ctx, user, providerName, info.Subject); err != nil {
				logger.Error("failed to link SSO to existing user by email",
					zap.Uint64("user_id", user.ID),
					zap.String("email", info.Email),
					zap.Error(err),
				)
				return nil, fmt.Errorf("将本地账号关联到 SSO 失败: %w", err)
			}
			logger.Info("SSO linked to existing user by email",
				zap.Uint64("user_id", user.ID),
				zap.String("email", info.Email),
			)
			return user, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("查询用户失败: %w", err)
		}
	}

	// 优先级 3: Username 匹配
	user, err = s.userRepo.GetByUsername(ctx, info.Username)
	if err == nil {
		if err := s.linkLocalUserToSSO(ctx, user, providerName, info.Subject); err != nil {
			logger.Error("failed to link SSO to existing user by username",
				zap.Uint64("user_id", user.ID),
				zap.String("username", info.Username),
				zap.Error(err),
			)
			return nil, fmt.Errorf("将本地账号关联到 SSO 失败: %w", err)
		}
		logger.Info("SSO linked to existing user by username",
			zap.Uint64("user_id", user.ID),
			zap.String("username", info.Username),
		)
		return user, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	// 优先级 4: JIT 自动创建用户
	if !cfg.AutoCreateUser {
		return nil, ErrSSOUserNotAllowed
	}

	return s.createSSOUser(ctx, info, providerName, cfg.DefaultRole)
}

// linkLocalUserToSSO 将本地账号升级为 SSO 账号
// 写入 SSOProvider/SSOSubject 的同时清空 PasswordHash（占位为不可用值），
// 切断本地密码登录通道，防止「升级后旧密码仍可登录」的双轨绕过
func (s *ssoService) linkLocalUserToSSO(ctx context.Context, user *model.User, provider, subject string) error {
	user.SSOProvider = provider
	user.SSOSubject = subject
	// PasswordHash 字段 not null，写一个非 bcrypt 格式的占位值
	// bcrypt.CompareHashAndPassword 对非法 hash 直接返回错误，无法匹配任何明文
	user.PasswordHash = ssoPasswordPlaceholder
	return s.userRepo.Update(ctx, user)
}

// ssoPasswordPlaceholder SSO 升级后写入 PasswordHash 的占位值
// 非 bcrypt 格式（不以 $2 开头），bcrypt 解析会直接失败
const ssoPasswordPlaceholder = "!sso-login-only"

// createSSOUser 创建 SSO 用户
func (s *ssoService) createSSOUser(ctx context.Context, info *dto.SSOUserInfo, providerName, defaultRole string) (*model.User, error) {
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return nil, fmt.Errorf("生成随机密码失败: %w", err)
	}
	hashedPassword, err := bcrypt.GenerateFromPassword(randomBytes, bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("密码加密失败: %w", err)
	}

	displayName := info.DisplayName
	if displayName == "" {
		displayName = info.Username
	}

	email := info.Email
	if email == "" {
		email = info.Username + "@sso.local"
	}

	user := &model.User{
		Username:        info.Username,
		Email:           email,
		PasswordHash:    string(hashedPassword),
		DisplayName:     displayName,
		AvatarURL:       info.AvatarURL,
		Status:          1,
		SSOProvider:     providerName,
		SSOSubject:      info.Subject,
		ExtraAttributes: "{}",
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("创建 SSO 用户失败: %w", err)
	}

	if defaultRole == "" {
		defaultRole = "user"
	}
	if err := s.userRoleRepo.AssignRole(ctx, user.ID, defaultRole); err != nil {
		logger.Warn("failed to assign default role to SSO user",
			zap.Uint64("user_id", user.ID),
			zap.String("role", defaultRole),
			zap.Error(err),
		)
	}

	logger.Info("SSO user created",
		zap.Uint64("user_id", user.ID),
		zap.String("username", user.Username),
		zap.String("sso_subject", info.Subject),
	)

	return user, nil
}

// getStringClaim 从 claims map 安全获取字符串值
func getStringClaim(claims map[string]interface{}, key string) string {
	if val, ok := claims[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// generateRandomString 生成指定长度的随机十六进制字符串
func generateRandomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
