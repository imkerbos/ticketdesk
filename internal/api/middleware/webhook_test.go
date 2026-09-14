package middleware

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// stubConfig 固定返回一个配置值
type stubConfig struct{ value string }

func (s stubConfig) GetConfigValue(_ context.Context, _ string) (string, error) {
	return s.value, nil
}

func sign(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func newWebhookEngine(secret string, maxBytes int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	e := gin.New()
	g := e.Group("/alerts")
	g.Use(BodyLimit(maxBytes))
	g.Use(WebhookSignature(stubConfig{value: secret}, "security.webhook_secret"))
	g.POST("/webhook", func(c *gin.Context) {
		// handler 必须仍能读到完整请求体（中间件读过之后要放回去）
		body, _ := io.ReadAll(c.Request.Body)
		c.String(http.StatusOK, string(body))
	})
	return e
}

func post(e *gin.Engine, body, signature string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/alerts/webhook", strings.NewReader(body))
	if signature != "" {
		req.Header.Set(WebhookSignatureHeader, signature)
	}
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	return w
}

// TestWebhookSignatureRejectsUnsigned 配置了密钥后，未签名的请求必须被拒。
// 这些端点对公网开放且不需要认证，签名是唯一的来源校验手段。
func TestWebhookSignatureRejectsUnsigned(t *testing.T) {
	e := newWebhookEngine("s3cr3t", DefaultWebhookMaxBytes)

	if w := post(e, `{"alerts":[]}`, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("无签名请求应返回 401, got %d", w.Code)
	}
	if w := post(e, `{"alerts":[]}`, "sha256=deadbeef"); w.Code != http.StatusUnauthorized {
		t.Fatalf("错误签名应返回 401, got %d", w.Code)
	}
}

// TestWebhookSignatureAcceptsValid 正确签名放行，且 handler 仍能读到完整请求体。
func TestWebhookSignatureAcceptsValid(t *testing.T) {
	const secret = "s3cr3t"
	const body = `{"alerts":[{"fingerprint":"abc"}]}`
	e := newWebhookEngine(secret, DefaultWebhookMaxBytes)

	w := post(e, body, sign(secret, body))
	if w.Code != http.StatusOK {
		t.Fatalf("正确签名应放行, got %d body=%s", w.Code, w.Body.String())
	}
	if w.Body.String() != body {
		t.Fatalf("中间件读取后未还原请求体: got %q", w.Body.String())
	}
}

// TestWebhookSignatureSkippedWhenNoSecret 未配置密钥时保持向后兼容，
// 避免升级瞬间打断线上 Prometheus / 夜莺告警链路。
func TestWebhookSignatureSkippedWhenNoSecret(t *testing.T) {
	e := newWebhookEngine("", DefaultWebhookMaxBytes)

	if w := post(e, `{"alerts":[]}`, ""); w.Code != http.StatusOK {
		t.Fatalf("未配置密钥时应放行, got %d", w.Code)
	}
}

// TestBodyLimitRejectsOversizedPayload 超大请求体必须被截断拒绝，
// 否则无认证端点可被用来打满内存。
func TestBodyLimitRejectsOversizedPayload(t *testing.T) {
	const secret = "s3cr3t"
	const limit = 64
	e := newWebhookEngine(secret, limit)

	big := strings.Repeat("A", limit*4)
	w := post(e, big, sign(secret, big))
	if w.Code == http.StatusOK {
		t.Fatalf("超限请求体不应被处理, got %d", w.Code)
	}
}

// TestValidSignatureConstantTimePrefixes 兼容带/不带 sha256= 前缀两种写法。
func TestValidSignatureAcceptsBareHex(t *testing.T) {
	const secret = "k"
	body := []byte("payload")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	bare := hex.EncodeToString(mac.Sum(nil))

	if !validSignature(secret, body, bare) {
		t.Fatal("裸 hex 签名应被接受")
	}
	if !validSignature(secret, body, "sha256="+bare) {
		t.Fatal("带前缀签名应被接受")
	}
	if validSignature(secret, body, "sha256="+strings.Repeat("0", 64)) {
		t.Fatal("错误签名不应通过")
	}
	if validSignature(secret, bytes.NewBufferString("tampered").Bytes(), "sha256="+bare) {
		t.Fatal("请求体被篡改后签名不应通过")
	}
}
