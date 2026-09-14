// Package jwt 提供 JWT 认证功能
package jwt

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/kerbos/ticketdesk/pkg/config"
)

var (
	// ErrTokenExpired Token 已过期
	ErrTokenExpired = errors.New("token has expired")
	// ErrTokenInvalid Token 无效
	ErrTokenInvalid = errors.New("token is invalid")
	// ErrTokenMalformed Token 格式错误
	ErrTokenMalformed = errors.New("token is malformed")
	// ErrTokenWrongType Token 类型不符（如拿 access token 去刷新）
	ErrTokenWrongType = errors.New("token type mismatch")
)

// Token 类型常量
//
// 早期版本三种 token 的 claims 结构完全一致，导致 access token 可以当作
// refresh token 反复续期 —— 偷到一个 2 小时的 access token 就等于永久访问。
// 因此签发时写入 typ，校验时必须按用途比对。
const (
	// TokenTypeAccess 访问令牌，用于调用业务接口
	TokenTypeAccess = "access"
	// TokenTypeRefresh 刷新令牌，只能用于换取新的令牌对
	TokenTypeRefresh = "refresh"
	// TokenTypeMFA 二次验证挑战令牌，只能用于提交 TOTP 码
	TokenTypeMFA = "mfa"
)

// mfaChallengeExpire MFA 挑战令牌有效期
const mfaChallengeExpire = 5 * time.Minute

// Claims 自定义 JWT Claims
type Claims struct {
	UserID   uint64 `json:"user_id"`
	Username string `json:"username"`
	// TokenType 令牌用途，见 TokenType* 常量
	TokenType string `json:"typ"`
	// TokenVersion 签发时用户的令牌版本号。
	// 改密码 / 禁用账号 / 主动登出会递增用户侧版本号，使存量令牌立即失效。
	TokenVersion int `json:"ver"`
	jwt.RegisteredClaims
}

// Manager JWT 管理器
type Manager struct {
	secret        []byte
	accessExpire  time.Duration
	refreshExpire time.Duration
	issuer        string
}

// NewManager 创建 JWT 管理器
func NewManager(cfg *config.JWTConfig) *Manager {
	return &Manager{
		secret:        []byte(cfg.Secret),
		accessExpire:  time.Duration(cfg.AccessExpire) * time.Second,
		refreshExpire: time.Duration(cfg.RefreshExpire) * time.Second,
		issuer:        cfg.Issuer,
	}
}

// AccessExpireSeconds 返回 access token 有效期秒数（供登录响应回填）
func (m *Manager) AccessExpireSeconds() int64 {
	return int64(m.accessExpire / time.Second)
}

// GenerateAccessToken 生成 Access Token
func (m *Manager) GenerateAccessToken(userID uint64, username string, tokenVersion int) (string, error) {
	return m.generateToken(userID, username, TokenTypeAccess, tokenVersion, m.accessExpire)
}

// GenerateRefreshToken 生成 Refresh Token
func (m *Manager) GenerateRefreshToken(userID uint64, username string, tokenVersion int) (string, error) {
	return m.generateToken(userID, username, TokenTypeRefresh, tokenVersion, m.refreshExpire)
}

// GenerateMFAChallengeToken 生成 MFA 挑战令牌
// 密码校验通过、但尚未完成 TOTP 验证时签发；只能用于调用 /auth/mfa/verify
func (m *Manager) GenerateMFAChallengeToken(userID uint64, username string, tokenVersion int) (string, error) {
	return m.generateToken(userID, username, TokenTypeMFA, tokenVersion, mfaChallengeExpire)
}

// generateToken 生成 Token
func (m *Manager) generateToken(userID uint64, username, tokenType string, tokenVersion int, expire time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:       userID,
		Username:     username,
		TokenType:    tokenType,
		TokenVersion: tokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expire)),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// ParseToken 解析 Token（不校验用途，仅供内部复用；业务侧请用 ParseTokenOfType）
func (m *Manager) ParseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{},
		func(token *jwt.Token) (interface{}, error) {
			return m.secret, nil
		},
		// 锁定签名算法，杜绝 alg 混淆
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		if errors.Is(err, jwt.ErrTokenMalformed) {
			return nil, ErrTokenMalformed
		}
		return nil, ErrTokenInvalid
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrTokenInvalid
}

// ParseTokenOfType 解析并校验 Token 用途
//
// 升级前签发的令牌没有 typ 字段，这里一律判为无效：
// 若把空 typ 当作 access 放行，旧的 refresh token 就能直接当访问令牌用，
// 恰好是本次要堵的问题。代价是升级后所有人需要重新登录一次。
func (m *Manager) ParseTokenOfType(tokenString, expectedType string) (*Claims, error) {
	claims, err := m.ParseToken(tokenString)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != expectedType {
		return nil, fmt.Errorf("%w: expected %s, got %q", ErrTokenWrongType, expectedType, claims.TokenType)
	}
	return claims, nil
}
