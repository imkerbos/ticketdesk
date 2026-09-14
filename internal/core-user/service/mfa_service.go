// Package service 提供 MFA 服务
package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"time"

	qrcode "github.com/skip2/go-qrcode"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/kerbos/ticketdesk/internal/core-user/dto"
	"github.com/kerbos/ticketdesk/internal/core-user/repository"
	"github.com/kerbos/ticketdesk/pkg/cache"
	"github.com/kerbos/ticketdesk/pkg/logger"
)

// MFA 相关错误
var (
	ErrMFAAlreadyEnabled  = errors.New("user.mfa_enabled")
	ErrMFANotEnabled      = errors.New("user.mfa_not_enabled")
	ErrMFAInvalidCode     = errors.New("user.code_wrong")
	ErrMFASetupNotStarted = errors.New("user.mfa_not_started")
	ErrMFACodeReused      = errors.New("user.code_used")
)

// TOTP 配置
const (
	TOTPIssuer     = "TicketDesk"
	TOTPPeriod     = 30 // 验证码有效期（秒）
	TOTPDigits     = 6  // 验证码位数
	TOTPSecretSize = 20 // 密钥长度（字节）
)

// MFAService MFA 服务接口
type MFAService interface {
	// SetupMFA 开始 MFA 设置，返回密钥和二维码 URI
	SetupMFA(ctx context.Context, userID uint64) (*dto.MFASetupResponse, error)
	// VerifyAndEnableMFA 验证并启用 MFA
	VerifyAndEnableMFA(ctx context.Context, userID uint64, code string) error
	// DisableMFA 禁用 MFA
	DisableMFA(ctx context.Context, userID uint64, code string) error
	// VerifyMFA 验证 MFA 码
	VerifyMFA(ctx context.Context, userID uint64, code string) error
	// IsMFAEnabled 检查用户是否启用了 MFA
	IsMFAEnabled(ctx context.Context, userID uint64) (bool, error)
	// GetMFAStatus 获取 MFA 状态
	GetMFAStatus(ctx context.Context, userID uint64) (*dto.MFAStatusResponse, error)
}

// mfaService MFA 服务实现
type mfaService struct {
	userRepo repository.UserRepository
}

// NewMFAService 创建 MFA 服务实例
func NewMFAService(userRepo repository.UserRepository) MFAService {
	return &mfaService{
		userRepo: userRepo,
	}
}

// SetupMFA 开始 MFA 设置
func (s *mfaService) SetupMFA(ctx context.Context, userID uint64) (*dto.MFASetupResponse, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("获取用户失败: %w", err)
	}

	if user.MFAEnabled {
		return nil, ErrMFAAlreadyEnabled
	}

	// 生成新的密钥
	secret, err := generateSecret()
	if err != nil {
		return nil, fmt.Errorf("生成密钥失败: %w", err)
	}

	// 保存密钥（但不启用 MFA）
	user.MFASecret = secret
	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("保存密钥失败: %w", err)
	}

	// 生成二维码 URI
	otpAuthURL := generateOTPAuthURL(user.Username, secret)

	// 生成 QR 码图片（PNG 格式，200x200 像素）
	qrCodePNG, err := qrcode.Encode(otpAuthURL, qrcode.Medium, 200)
	if err != nil {
		logger.Error("failed to generate QR code", zap.Error(err))
		return nil, fmt.Errorf("生成二维码失败: %w", err)
	}

	// 将 QR 码转换为 base64
	qrCodeData := base64.StdEncoding.EncodeToString(qrCodePNG)

	logger.Info("MFA setup started", zap.Uint64("user_id", userID))

	return &dto.MFASetupResponse{
		Secret:     secret,
		OTPAuthURL: otpAuthURL,
		QRCodeData: "data:image/png;base64," + qrCodeData,
		Issuer:     TOTPIssuer,
		Account:    user.Username,
	}, nil
}

// VerifyAndEnableMFA 验证并启用 MFA
func (s *mfaService) VerifyAndEnableMFA(ctx context.Context, userID uint64, code string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("获取用户失败: %w", err)
	}

	if user.MFAEnabled {
		return ErrMFAAlreadyEnabled
	}

	if user.MFASecret == "" {
		return ErrMFASetupNotStarted
	}

	// 验证码
	ok, _ := verifyTOTP(user.MFASecret, code)
	if !ok {
		return ErrMFAInvalidCode
	}

	// 启用 MFA
	now := time.Now()
	user.MFAEnabled = true
	user.MFAVerifiedAt = &now
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("启用 MFA 失败: %w", err)
	}

	logger.Info("MFA enabled", zap.Uint64("user_id", userID))

	return nil
}

// DisableMFA 禁用 MFA
func (s *mfaService) DisableMFA(ctx context.Context, userID uint64, code string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("获取用户失败: %w", err)
	}

	if !user.MFAEnabled {
		return ErrMFANotEnabled
	}

	// 验证码
	ok, _ := verifyTOTP(user.MFASecret, code)
	if !ok {
		return ErrMFAInvalidCode
	}

	// 禁用 MFA
	user.MFAEnabled = false
	user.MFASecret = ""
	user.MFAVerifiedAt = nil
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("禁用 MFA 失败: %w", err)
	}

	logger.Info("MFA disabled", zap.Uint64("user_id", userID))

	return nil
}

// VerifyMFA 验证 MFA 码
func (s *mfaService) VerifyMFA(ctx context.Context, userID uint64, code string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("获取用户失败: %w", err)
	}

	if !user.MFAEnabled {
		return ErrMFANotEnabled
	}

	ok, counter := verifyTOTP(user.MFASecret, code)
	if !ok {
		return ErrMFAInvalidCode
	}

	// 同一个验证码只允许用一次，防止在 ±1 窗口内被重放
	if !consumeTOTPCounter(ctx, userID, counter) {
		return ErrMFACodeReused
	}

	return nil
}

// IsMFAEnabled 检查用户是否启用了 MFA
func (s *mfaService) IsMFAEnabled(ctx context.Context, userID uint64) (bool, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, ErrUserNotFound
		}
		return false, fmt.Errorf("获取用户失败: %w", err)
	}

	return user.MFAEnabled, nil
}

// GetMFAStatus 获取 MFA 状态
func (s *mfaService) GetMFAStatus(ctx context.Context, userID uint64) (*dto.MFAStatusResponse, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("获取用户失败: %w", err)
	}

	return &dto.MFAStatusResponse{
		Enabled:    user.MFAEnabled,
		VerifiedAt: user.MFAVerifiedAt,
	}, nil
}

// ============ TOTP 辅助函数 ============

// generateSecret 生成随机密钥
func generateSecret() (string, error) {
	secret := make([]byte, TOTPSecretSize)
	if _, err := rand.Read(secret); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret), nil
}

// generateOTPAuthURL 生成 OTP Auth URL（用于二维码）
// 格式: otpauth://totp/ISSUER:ACCOUNT?secret=SECRET&issuer=ISSUER&algorithm=SHA1&digits=6&period=30
func generateOTPAuthURL(account, secret string) string {
	// URL 编码 issuer 和 account
	encodedIssuer := url.PathEscape(TOTPIssuer)
	encodedAccount := url.PathEscape(account)

	// 构建 URL，注意 label 部分的格式是 issuer:account
	// secret 不需要 URL 编码因为它是 base32 字符
	return fmt.Sprintf(
		"otpauth://totp/%s:%s?secret=%s&issuer=%s&algorithm=SHA1&digits=%d&period=%d",
		encodedIssuer, encodedAccount, secret, url.QueryEscape(TOTPIssuer), TOTPDigits, TOTPPeriod,
	)
}

// verifyTOTP 验证 TOTP 码
// 支持当前时间窗口和前后各一个窗口，返回是否通过及命中的计数器值
//
// 比较使用 subtle.ConstantTimeCompare：字符串 == 会在首个不同字节处提前返回，
// 攻击者可据此逐位推导正确验证码。
func verifyTOTP(secret, code string) (bool, uint64) {
	// 解码密钥
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		return false, 0
	}

	// 当前时间戳
	now := time.Now().Unix()

	matched := false
	var matchedCounter uint64

	// 检查当前窗口和前后各一个窗口；不提前 break，避免命中位置造成时间差
	for _, offset := range []int64{-1, 0, 1} {
		timestamp := now + offset*TOTPPeriod
		counter := uint64(timestamp / TOTPPeriod)
		expectedCode := generateHOTP(key, counter)
		if subtle.ConstantTimeCompare([]byte(expectedCode), []byte(code)) == 1 {
			matched = true
			matchedCounter = counter
		}
	}

	return matched, matchedCounter
}

// consumeTOTPCounter 标记某个 TOTP 时间片已被使用，返回 true 表示本次是首次使用
//
// TOTP 码在 ±1 窗口内有效（约 90 秒），期间同一个码可以重复提交。
// 攻击者只要窃取到一次验证码（肩窥、钓鱼页面、中间人）即可在这段时间内重放登录。
// 这里用 Redis SETNX 把「用户 + 时间片」标记为已消费，做到一次性。
// Redis 不可用时返回 true 放行，避免把 MFA 变成单点故障。
func consumeTOTPCounter(ctx context.Context, userID uint64, counter uint64) bool {
	key := fmt.Sprintf("mfa:used:%d:%d", userID, counter)
	// TTL 略大于 ±1 窗口跨度，确保覆盖该码的完整有效期
	return cache.SetNX(ctx, key, 1, time.Duration(TOTPPeriod*3)*time.Second)
}

// generateHOTP 生成 HOTP 码
func generateHOTP(key []byte, counter uint64) string {
	// 将 counter 转换为 8 字节大端序
	msg := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		msg[i] = byte(counter & 0xff)
		counter >>= 8
	}

	// HMAC-SHA1
	h := hmacSHA1(key, msg)

	// 动态截取
	offset := h[len(h)-1] & 0x0f
	code := int32(h[offset]&0x7f)<<24 |
		int32(h[offset+1]&0xff)<<16 |
		int32(h[offset+2]&0xff)<<8 |
		int32(h[offset+3]&0xff)

	// 取模得到指定位数的码
	code %= 1000000 // 6 位

	return fmt.Sprintf("%06d", code)
}

// hmacSHA1 计算 HMAC-SHA1
func hmacSHA1(key, msg []byte) []byte {
	mac := hmac.New(sha1.New, key)
	mac.Write(msg)
	return mac.Sum(nil)
}
