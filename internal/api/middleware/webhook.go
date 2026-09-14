// Package middleware: 告警 Webhook 入口的加固
package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/kerbos/ticketdesk/internal/api/response"
	"github.com/kerbos/ticketdesk/pkg/logger"
)

// WebhookSignatureHeader 签名请求头，格式 "sha256=<hex>"
const WebhookSignatureHeader = "X-TicketDesk-Signature"

// DefaultWebhookMaxBytes 告警 Webhook 请求体上限（1 MiB）
//
// 这些端点无需认证，而处理逻辑会读取整个请求体（夜莺入口直接 c.GetRawData()）。
// 不设上限意味着任何人都能用一个超大 POST 撑爆内存。
const DefaultWebhookMaxBytes int64 = 1 << 20

// BodyLimit 限制请求体大小，超出返回 413
func BodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}

// WebhookSignature 校验告警 Webhook 的 HMAC-SHA256 签名
//
// 仅在系统配置里设置了 webhook 密钥时启用：这些端点已有大量存量接入
// （Prometheus Alertmanager、夜莺等），强制开启会直接打断线上告警链路。
// 未配置时放行并留一条 Warn 日志，提示该入口目前对公网无鉴权。
//
// 签名算法：hex(HMAC_SHA256(secret, rawBody))，放在 X-TicketDesk-Signature 头，
// 允许带 "sha256=" 前缀。比较使用 hmac.Equal，避免按字节提前返回泄露信息。
func WebhookSignature(cfg ConfigValueGetter, configKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		secret := ""
		if cfg != nil && configKey != "" {
			if v, err := cfg.GetConfigValue(c.Request.Context(), configKey); err == nil {
				secret = strings.TrimSpace(v)
			}
		}

		// 未配置密钥：保持向后兼容，放行
		if secret == "" {
			logger.Warn("webhook signature not enforced: secret not configured",
				zap.String("path", c.Request.URL.Path),
				zap.String("config_key", configKey),
			)
			c.Next()
			return
		}

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			// 触发 MaxBytesReader 上限时也会走到这里
			response.ErrorT(c, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "common.payload_too_large")
			c.Abort()
			return
		}
		// 读完要放回去，后续 handler 仍需解析
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		if !validSignature(secret, body, c.GetHeader(WebhookSignatureHeader)) {
			logger.Warn("webhook signature verification failed",
				zap.String("path", c.Request.URL.Path),
				zap.String("ip", c.ClientIP()),
			)
			response.UnauthorizedT(c, "common.signature_failed")
			c.Abort()
			return
		}

		c.Next()
	}
}

// validSignature 比对签名
func validSignature(secret string, body []byte, provided string) bool {
	provided = strings.TrimSpace(provided)
	if provided == "" {
		return false
	}
	provided = strings.TrimPrefix(provided, "sha256=")

	got, err := hex.DecodeString(provided)
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hmac.Equal(got, mac.Sum(nil))
}
