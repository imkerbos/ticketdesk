// Package service 提供用户模块业务逻辑
package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"

	"gorm.io/gorm"

	"github.com/kerbos/ticketdesk/internal/core-user/dto"
	"github.com/kerbos/ticketdesk/internal/core-user/repository"
	"github.com/kerbos/ticketdesk/internal/model"
	"github.com/kerbos/ticketdesk/pkg/safego"
)

// API token 相关常量
const (
	TokenPrefix      = "td_pat_"
	TokenBodyLen     = 32
	TokenAlphabet    = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	MaxTokensPerUser = 20
)

// API token 相关错误
var (
	ErrTokenLimitExceeded = errors.New("user.token_limit")
	ErrTokenInvalid       = errors.New("user.api_token_invalid")
	ErrTokenExpired       = errors.New("user.api_token_expired")
	ErrTokenNotFound      = errors.New("user.token_not_found")
)

// APITokenService API token 业务接口
type APITokenService interface {
	Create(ctx context.Context, userID uint64, req *dto.CreateTokenRequest) (*dto.CreateTokenResponse, error)
	List(ctx context.Context, userID uint64) ([]*dto.TokenResponse, error)
	Delete(ctx context.Context, userID, tokenID uint64) error
	Authenticate(ctx context.Context, plainToken string) (*model.User, *model.APIToken, error)
}

type apiTokenService struct {
	tokenRepo repository.APITokenRepository
	userRepo  repository.UserRepository
}

// NewAPITokenService 创建 API token 业务服务实例
func NewAPITokenService(tokenRepo repository.APITokenRepository, userRepo repository.UserRepository) APITokenService {
	return &apiTokenService{tokenRepo: tokenRepo, userRepo: userRepo}
}

// Create 创建新 API token
func (s *apiTokenService) Create(ctx context.Context, userID uint64, req *dto.CreateTokenRequest) (*dto.CreateTokenResponse, error) {
	// 上限校验
	count, err := s.tokenRepo.CountByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("统计 token 数量失败: %w", err)
	}
	if count >= MaxTokensPerUser {
		return nil, ErrTokenLimitExceeded
	}

	plain, hash, prefix, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("生成 token 失败: %w", err)
	}

	var expiresAt *time.Time
	if req.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(req.ExpiresIn) * time.Second)
		expiresAt = &t
	}

	token := &model.APIToken{
		UserID:    userID,
		Name:      req.Name,
		TokenHash: hash,
		Prefix:    prefix,
		ExpiresAt: expiresAt,
	}
	if err := s.tokenRepo.Create(ctx, token); err != nil {
		return nil, fmt.Errorf("创建 token 记录失败: %w", err)
	}

	return &dto.CreateTokenResponse{
		ID:        token.ID,
		Name:      token.Name,
		Token:     plain,
		Prefix:    token.Prefix,
		ExpiresAt: token.ExpiresAt,
		CreatedAt: token.CreatedAt,
	}, nil
}

// List 查询用户 token 列表
func (s *apiTokenService) List(ctx context.Context, userID uint64) ([]*dto.TokenResponse, error) {
	tokens, err := s.tokenRepo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("查询 token 列表失败: %w", err)
	}
	now := time.Now()
	res := make([]*dto.TokenResponse, len(tokens))
	for i, t := range tokens {
		isExpired := t.ExpiresAt != nil && !t.ExpiresAt.After(now)
		res[i] = &dto.TokenResponse{
			ID:         t.ID,
			Name:       t.Name,
			Prefix:     t.Prefix,
			ExpiresAt:  t.ExpiresAt,
			LastUsedAt: t.LastUsedAt,
			CreatedAt:  t.CreatedAt,
			IsExpired:  isExpired,
		}
	}
	return res, nil
}

// Delete 撤销指定 token
func (s *apiTokenService) Delete(ctx context.Context, userID, tokenID uint64) error {
	t, err := s.tokenRepo.GetByIDAndUserID(ctx, tokenID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrTokenNotFound
		}
		return fmt.Errorf("查询 token 失败: %w", err)
	}
	return s.tokenRepo.Delete(ctx, t.ID)
}

// Authenticate 用明文 token 鉴权，返回对应用户和 token 记录
func (s *apiTokenService) Authenticate(ctx context.Context, plainToken string) (*model.User, *model.APIToken, error) {
	hash := hashToken(plainToken)
	token, err := s.tokenRepo.GetByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrTokenInvalid
		}
		return nil, nil, fmt.Errorf("查询 token 失败: %w", err)
	}

	// 加载用户
	user, err := s.userRepo.GetByID(ctx, token.UserID)
	if err != nil {
		return nil, nil, ErrTokenInvalid
	}
	if user.Status == 0 {
		return nil, nil, ErrTokenInvalid
	}

	// 异步更新 last_used_at，避免阻塞请求
	go func() {
		defer safego.Recover("token.updateLastUsedAt")
		bgCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = s.tokenRepo.UpdateLastUsed(bgCtx, token.ID)
	}()

	return user, token, nil
}

// generateToken 生成新 token，返回 (plain, hash, prefix, err)
func generateToken() (string, string, string, error) {
	body := make([]byte, TokenBodyLen)
	alphabetLen := big.NewInt(int64(len(TokenAlphabet)))
	for i := range body {
		idx, err := rand.Int(rand.Reader, alphabetLen)
		if err != nil {
			return "", "", "", err
		}
		body[i] = TokenAlphabet[idx.Int64()]
	}
	plain := TokenPrefix + string(body)
	return plain, hashToken(plain), plain[:16], nil
}

// hashToken 对明文 token 做 sha256 并返回 hex 字符串
func hashToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// IsAPIToken 判断字符串是否符合 API token 格式
func IsAPIToken(s string) bool {
	return len(s) == len(TokenPrefix)+TokenBodyLen && s[:len(TokenPrefix)] == TokenPrefix
}
