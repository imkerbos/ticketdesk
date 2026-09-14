// Package service: 登录态校验所需的用户快照
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/kerbos/ticketdesk/internal/model"

	"github.com/kerbos/ticketdesk/pkg/cache"
)

// authStateTTL 鉴权态缓存时长。
// 取值权衡：改密码 / 禁用账号会主动失效缓存，正常路径立即生效；
// 该 TTL 只是多副本部署下未及时收到失效通知时的最坏收敛时间。
const authStateTTL = 60 * time.Second

func authStateKey(userID uint64) string {
	return fmt.Sprintf("auth:state:%d", userID)
}

// AuthState 鉴权所需的用户状态快照
type AuthState struct {
	Status       int8 `json:"status"`
	TokenVersion int  `json:"ver"`
}

// AuthStateProvider 供认证中间件查询用户当前状态
//
// 存在的原因：JWT 是自包含的，签发后无法撤回。原先中间件只验签名，
// 于是「禁用用户」「改完密码」之后旧令牌仍能继续用到自然过期（最长 2 小时）。
type AuthStateProvider interface {
	// GetAuthState 读取用户鉴权态（带缓存）
	GetAuthState(ctx context.Context, userID uint64) (*AuthState, error)
	// InvalidateAuthState 主动失效缓存，用于状态变更后立即生效
	InvalidateAuthState(ctx context.Context, userID uint64)
}

// GetAuthState 读取用户鉴权态，优先走 Redis
func (s *userService) GetAuthState(ctx context.Context, userID uint64) (*AuthState, error) {
	key := authStateKey(userID)

	var cached AuthState
	if cache.GetJSON(ctx, key, &cached) {
		return &cached, nil
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	state := AuthState{Status: user.Status, TokenVersion: user.TokenVersion}
	cache.SetJSON(ctx, key, state, authStateTTL)
	return &state, nil
}

// InvalidateAuthState 清除鉴权态缓存
func (s *userService) InvalidateAuthState(ctx context.Context, userID uint64) {
	cache.Del(ctx, authStateKey(userID))
}

// bumpTokenVersion 递增用户令牌版本号，使其已签发的全部 JWT 立即失效
// 用于：修改密码、通过邮件重置密码、禁用账号、管理员重置密码
func (s *userService) bumpTokenVersion(ctx context.Context, userID uint64) error {
	if err := s.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", userID).
		UpdateColumn("token_version", gorm.Expr("token_version + 1")).Error; err != nil {
		return fmt.Errorf("递增令牌版本失败: %w", err)
	}
	s.InvalidateAuthState(ctx, userID)
	return nil
}
