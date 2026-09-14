package jwt

import (
	"errors"
	"testing"

	"github.com/kerbos/ticketdesk/pkg/config"
)

func newTestManager(accessExpire int) *Manager {
	return NewManager(&config.JWTConfig{
		Secret:        "test-secret-for-unit-test",
		AccessExpire:  accessExpire,
		RefreshExpire: 604800,
		Issuer:        "ticketdesk-test",
	})
}

// TestAccessTokenCannotBeUsedAsRefreshToken 三类令牌必须按用途隔离。
//
// 早期实现三者 claims 完全一致，access token 可直接拿去 /auth/refresh 换新令牌 ——
// 偷到一个 2 小时的访问令牌就等于拿到永久访问权。
func TestAccessTokenCannotBeUsedAsRefreshToken(t *testing.T) {
	m := newTestManager(7200)

	access, err := m.GenerateAccessToken(1, "alice", 1)
	if err != nil {
		t.Fatalf("生成 access token 失败: %v", err)
	}

	if _, err := m.ParseTokenOfType(access, TokenTypeRefresh); !errors.Is(err, ErrTokenWrongType) {
		t.Fatalf("access token 不应能当作 refresh token，err = %v", err)
	}
	if _, err := m.ParseTokenOfType(access, TokenTypeAccess); err != nil {
		t.Fatalf("access token 按自身用途校验应通过: %v", err)
	}
}

// TestMFAChallengeTokenCannotAccessAPI MFA 挑战令牌只能用于提交 TOTP 码，
// 不能直接当访问令牌调业务接口——否则密码正确即可绕过二次验证。
func TestMFAChallengeTokenCannotAccessAPI(t *testing.T) {
	m := newTestManager(7200)

	challenge, err := m.GenerateMFAChallengeToken(9, "bob", 3)
	if err != nil {
		t.Fatalf("生成 MFA 挑战令牌失败: %v", err)
	}

	for _, wrong := range []string{TokenTypeAccess, TokenTypeRefresh} {
		if _, err := m.ParseTokenOfType(challenge, wrong); !errors.Is(err, ErrTokenWrongType) {
			t.Fatalf("MFA 挑战令牌不应能当作 %s 使用，err = %v", wrong, err)
		}
	}

	claims, err := m.ParseTokenOfType(challenge, TokenTypeMFA)
	if err != nil {
		t.Fatalf("MFA 挑战令牌按自身用途校验应通过: %v", err)
	}
	if claims.UserID != 9 || claims.TokenVersion != 3 {
		t.Fatalf("claims 不符: userID=%d ver=%d", claims.UserID, claims.TokenVersion)
	}
}

// TestTokenCarriesVersion 令牌必须携带签发时的版本号，
// 供服务端在改密码 / 强制登出后判定其已失效。
func TestTokenCarriesVersion(t *testing.T) {
	m := newTestManager(7200)

	token, err := m.GenerateAccessToken(5, "carol", 42)
	if err != nil {
		t.Fatalf("生成失败: %v", err)
	}
	claims, err := m.ParseTokenOfType(token, TokenTypeAccess)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if claims.TokenVersion != 42 {
		t.Fatalf("令牌版本号丢失: got %d, want 42", claims.TokenVersion)
	}
}

// TestLegacyTokenWithoutTypeRejected 升级前签发的令牌没有 typ 字段。
// 必须一律判为无效：若把空 typ 当 access 放行，旧 refresh token 就能直接访问业务接口。
func TestLegacyTokenWithoutTypeRejected(t *testing.T) {
	m := newTestManager(7200)

	// 用 generateToken 直接造一个 typ 为空的令牌，模拟升级前的存量令牌
	legacy, err := m.generateToken(1, "alice", "", 0, m.accessExpire)
	if err != nil {
		t.Fatalf("生成失败: %v", err)
	}

	if _, err := m.ParseTokenOfType(legacy, TokenTypeAccess); !errors.Is(err, ErrTokenWrongType) {
		t.Fatalf("无 typ 的存量令牌必须拒绝，err = %v", err)
	}
}

// TestExpiredTokenDoesNotPanic 过期令牌走解析路径不得 panic。
func TestExpiredTokenDoesNotPanic(t *testing.T) {
	m := newTestManager(-1) // accessExpire 为负 → access token 签发即过期

	expired, err := m.GenerateAccessToken(1, "alice", 1)
	if err != nil {
		t.Fatalf("生成过期 token 失败: %v", err)
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("解析过期 token 不应 panic: %v", r)
		}
	}()

	if _, err := m.ParseTokenOfType(expired, TokenTypeAccess); !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("期望 ErrTokenExpired，实际: %v", err)
	}
}

// TestRejectsNonHS256 锁定签名算法，杜绝 alg 混淆。
func TestRejectsNonHS256(t *testing.T) {
	m := newTestManager(7200)

	// alg=none 的令牌（header/payload 为合法 base64，签名段为空）
	const noneToken = "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0." +
		"eyJ1c2VyX2lkIjoxLCJ1c2VybmFtZSI6ImFsaWNlIiwidHlwIjoiYWNjZXNzIn0."

	if _, err := m.ParseToken(noneToken); err == nil {
		t.Fatal("alg=none 的令牌必须拒绝")
	}
}
