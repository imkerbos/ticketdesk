// Package middleware: Swagger UI 访问鉴权
package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/kerbos/ticketdesk/internal/api/response"
	userService "github.com/kerbos/ticketdesk/internal/core-user/service"
	"github.com/kerbos/ticketdesk/pkg/jwt"
)

// SwaggerAuthMiddleware 限制 /swagger/* 仅登录用户访问.
// 接受两种凭证 (任一):
//  1. Authorization: Bearer <jwt 或 td_pat_xxx> 头
//  2. cookie `td_swagger_token` (前端 /api-docs 页调 /auth/swagger-session 设置后跳 swagger UI)
//
// 拒绝时返回 401 (JSON), 由前端拦截后引导登录.
func SwaggerAuthMiddleware(jwtManager *jwt.Manager, tokenSvc userService.APITokenService) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := extractCredential(c)
		if raw == "" {
			response.Unauthorized(c, "apidoc.login_required")
			c.Abort()
			return
		}

		// PAT 路径
		if userService.IsAPIToken(raw) {
			user, _, err := tokenSvc.Authenticate(c.Request.Context(), raw)
			if err != nil {
				response.Unauthorized(c, "user.api_token_bad")
				c.Abort()
				return
			}
			c.Set("user_id", user.ID)
			c.Set("username", user.Username)
			c.Next()
			return
		}

		// JWT 路径
		claims, err := jwtManager.ParseToken(raw)
		if err != nil {
			response.Unauthorized(c, "apidoc.session_expired")
			c.Abort()
			return
		}
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Next()
	}
}

// extractCredential 按优先级从多源取 token: header > cookie.
// 刻意不支持 ?token= : URL 里的凭证会进入访问日志、Referer 头和浏览器历史,
// 前端 /api-docs 走的是 /auth/swagger-session 设的 HttpOnly cookie, 不依赖 query.
func extractCredential(c *gin.Context) string {
	// 1. Authorization header
	if h := c.GetHeader("Authorization"); h != "" {
		parts := strings.SplitN(h, " ", 2)
		if len(parts) == 2 && parts[0] == "Bearer" && parts[1] != "" {
			return parts[1]
		}
	}
	// 2. cookie (前端 /api-docs 页设置)
	if v, err := c.Cookie("td_swagger_token"); err == nil && v != "" {
		return v
	}
	return ""
}
