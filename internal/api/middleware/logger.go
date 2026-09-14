// Package middleware 提供 HTTP 中间件
package middleware

import (
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/kerbos/ticketdesk/pkg/logger"
)

// sensitiveQueryKeys 需要在日志中脱敏的 query 参数名（小写）。
// /ws 用 ?token= 传 JWT（浏览器无法给 WebSocket 握手加自定义头），
// 直接记录 RawQuery 会把可用的凭证明文写进应用日志。
var sensitiveQueryKeys = map[string]bool{
	"token":         true,
	"access_token":  true,
	"refresh_token": true,
	"id_token":      true,
	"code":          true,
	"password":      true,
	"secret":        true,
	"client_secret": true,
}

// redactQuery 将 query string 中的敏感参数值替换为 [REDACTED]，其余原样保留。
// 解析失败时整体丢弃，宁可少记也不泄露。
func redactQuery(raw string) string {
	if raw == "" {
		return ""
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return "[unparsable]"
	}
	redacted := false
	for key, vals := range values {
		if !sensitiveQueryKeys[strings.ToLower(key)] {
			continue
		}
		for i := range vals {
			vals[i] = "[REDACTED]"
		}
		redacted = true
	}
	if !redacted {
		return raw
	}
	return values.Encode()
}

// LoggerMiddleware 请求日志中间件
func LoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := redactQuery(c.Request.URL.RawQuery)

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()

		fields := []zap.Field{
			zap.Int("status", statusCode),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", query),
			zap.String("ip", c.ClientIP()),
			zap.String("user_agent", c.Request.UserAgent()),
			zap.Duration("latency", latency),
		}

		if userID, exists := c.Get("user_id"); exists {
			fields = append(fields, zap.Any("user_id", userID))
		}

		switch {
		case len(c.Errors) > 0:
			fields = append(fields, zap.String("errors", c.Errors.String()))
			logger.Error("request error", fields...)
		case statusCode >= 500:
			logger.Error("server error", fields...)
		case statusCode >= 400:
			logger.Warn("client error", fields...)
		default:
			logger.Info("request completed", fields...)
		}
	}
}
