// Package service SSO 升级 + 本地登录拦截单元测试
package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/kerbos/ticketdesk/internal/core-user/dto"
	"github.com/kerbos/ticketdesk/internal/core-user/repository"
	"github.com/kerbos/ticketdesk/internal/model"
)

// fakeUserRepo 仅实现测试用到的方法；其余返回 panic 暴露未预期调用
type fakeUserRepo struct {
	getByUsernameFn func(ctx context.Context, username string) (*model.User, error)
	updateFn        func(ctx context.Context, user *model.User) error
	updateCalls     []*model.User
}

func (f *fakeUserRepo) Create(_ context.Context, _ *model.User) error { panic("not used") }
func (f *fakeUserRepo) GetByID(_ context.Context, _ uint64) (*model.User, error) {
	panic("not used")
}
func (f *fakeUserRepo) GetByIDs(_ context.Context, _ []uint64) ([]*model.User, error) {
	panic("not used")
}
func (f *fakeUserRepo) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	if f.getByUsernameFn != nil {
		return f.getByUsernameFn(ctx, username)
	}
	return nil, gorm.ErrRecordNotFound
}
func (f *fakeUserRepo) GetByEmail(_ context.Context, _ string) (*model.User, error) {
	panic("not used")
}
func (f *fakeUserRepo) GetByResetToken(_ context.Context, _ string) (*model.User, error) {
	panic("not used")
}
func (f *fakeUserRepo) GetBySSOSubject(_ context.Context, _, _ string) (*model.User, error) {
	panic("not used")
}
func (f *fakeUserRepo) Update(ctx context.Context, user *model.User) error {
	f.updateCalls = append(f.updateCalls, user)
	if f.updateFn != nil {
		return f.updateFn(ctx, user)
	}
	return nil
}
func (f *fakeUserRepo) Delete(_ context.Context, _ uint64) error { panic("not used") }
func (f *fakeUserRepo) List(_ context.Context, _, _ int, _ string, _ *int8) ([]*model.User, int64, error) {
	panic("not used")
}
func (f *fakeUserRepo) ExistsByUsername(_ context.Context, _ string) (bool, error) {
	panic("not used")
}
func (f *fakeUserRepo) ExistsByEmail(_ context.Context, _ string) (bool, error) {
	panic("not used")
}

// 编译期保证 fakeUserRepo 实现 UserRepository
var _ repository.UserRepository = (*fakeUserRepo)(nil)

// --- A: Login 拦截 SSO 用户 ---

// TestLogin_RejectsSSOLinkedAccount 本地登录入口对已绑 SSO 的账号应拒绝
// 防回归：避免「升级为 SSO 后旧密码仍可登录」的双轨绕过
func TestLogin_RejectsSSOLinkedAccount(t *testing.T) {
	repo := &fakeUserRepo{
		getByUsernameFn: func(_ context.Context, _ string) (*model.User, error) {
			// 已绑 SSO 的用户：SSOProvider 非空
			// 密码 hash 即使是有效的本地哈希也不应被验证
			validHash, _ := bcrypt.GenerateFromPassword([]byte("oldpass"), bcrypt.MinCost)
			return &model.User{
				Username:     "alice",
				PasswordHash: string(validHash),
				Status:       1,
				SSOProvider:  "eiam",
				SSOSubject:   "sub-123",
			}, nil
		},
	}

	svc := &userService{userRepo: repo}
	_, err := svc.Login(context.Background(), &dto.LoginRequest{
		Username: "alice",
		Password: "oldpass", // 即使密码正确也应被拒
	})

	if !errors.Is(err, ErrAccountUsesSSOLogin) {
		t.Fatalf("期望 ErrAccountUsesSSOLogin, 实际 %v", err)
	}
}

// TestLogin_AcceptsLocalUser 未绑 SSO 的本地用户仍可正常密码登录
// 防误伤：A 改动不能破坏纯本地用户的登录
func TestLogin_AcceptsLocalUser(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("good"), bcrypt.MinCost)
	repo := &fakeUserRepo{
		getByUsernameFn: func(_ context.Context, _ string) (*model.User, error) {
			u := &model.User{
				Username:     "bob",
				PasswordHash: string(hash),
				Status:       1,
				SSOProvider:  "", // 纯本地
			}
			u.ID = 1
			return u, nil
		},
	}

	// 不构造 jwtManager；Login 在 SSO 检查通过后会走 bcrypt 验证，
	// 错密码场景到 bcrypt 即返回 ErrInvalidCredentials，不会触达 jwt
	svc := &userService{userRepo: repo}
	_, err := svc.Login(context.Background(), &dto.LoginRequest{
		Username: "bob",
		Password: "wrong",
	})

	// 期望被密码错误拦截，而不是被 SSO 拦截
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("期望 ErrInvalidCredentials, 实际 %v", err)
	}
}

// --- B: linkLocalUserToSSO 清 PasswordHash ---

// TestLinkLocalUserToSSO_ClearsPassword 升级时必须写占位 PasswordHash，
// 且占位值无法通过 bcrypt 验证任何明文（彻底废掉本地密码登录）
func TestLinkLocalUserToSSO_ClearsPassword(t *testing.T) {
	originalHash, _ := bcrypt.GenerateFromPassword([]byte("oldpass"), bcrypt.MinCost)
	user := &model.User{
		Username:     "alice",
		PasswordHash: string(originalHash),
	}
	user.ID = 42

	repo := &fakeUserRepo{}
	svc := &ssoService{userRepo: repo}

	if err := svc.linkLocalUserToSSO(context.Background(), user, "eiam", "sub-xyz"); err != nil {
		t.Fatalf("linkLocalUserToSSO err: %v", err)
	}

	if user.SSOProvider != "eiam" {
		t.Errorf("SSOProvider 未写入, 实际 %q", user.SSOProvider)
	}
	if user.SSOSubject != "sub-xyz" {
		t.Errorf("SSOSubject 未写入, 实际 %q", user.SSOSubject)
	}
	if user.PasswordHash == string(originalHash) {
		t.Errorf("PasswordHash 未被替换，仍是旧 bcrypt 哈希")
	}
	if user.PasswordHash != ssoPasswordPlaceholder {
		t.Errorf("PasswordHash 期望占位 %q, 实际 %q", ssoPasswordPlaceholder, user.PasswordHash)
	}
	// 占位值必须不是 bcrypt 格式，否则可能被意外匹配
	if strings.HasPrefix(user.PasswordHash, "$2") {
		t.Errorf("占位 PasswordHash 不能是 bcrypt 格式，否则有被破解风险")
	}
	// bcrypt 对占位值的任意明文校验都必须失败
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("oldpass")); err == nil {
		t.Errorf("占位 PasswordHash 不应能通过 bcrypt 验证")
	}
	if len(repo.updateCalls) != 1 {
		t.Errorf("Update 应被调用 1 次, 实际 %d", len(repo.updateCalls))
	}
}
