// Package middleware 提供 HTTP 中间件
package middleware

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/kerbos/ticketdesk/internal/api/response"
	"github.com/kerbos/ticketdesk/internal/core-user/service"
	"github.com/kerbos/ticketdesk/pkg/jwt"
)

// AuthMiddleware JWT 认证中间件，同时支持 PAT 鉴权
//
// authState 用于校验「令牌签发之后账号是否发生了变化」：
// JWT 自包含、签发后无法撤回，若只验签名，被禁用的账号、改过密码的账号
// 仍能拿旧令牌一直用到自然过期。
func AuthMiddleware(jwtManager *jwt.Manager, tokenSvc service.APITokenService, authState service.AuthStateProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.UnauthorizedT(c, "auth.missing_credential")
			c.Abort()
			return
		}

		// Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			response.UnauthorizedT(c, "auth.bad_auth_format")
			c.Abort()
			return
		}

		raw := parts[1]

		// API Token (PAT) 路径：前缀为 td_pat_ 时走 PAT 鉴权
		if service.IsAPIToken(raw) {
			user, _, err := tokenSvc.Authenticate(c.Request.Context(), raw)
			if err != nil {
				response.UnauthorizedT(c, "auth.api_token_invalid")
				c.Abort()
				return
			}
			c.Set("user_id", user.ID)
			c.Set("username", user.Username)
			c.Set("is_pat", true)
			c.Next()
			return
		}

		// 否则走 JWT 鉴权：必须是 access 类型，
		// refresh / mfa 挑战令牌都不得直接用于访问业务接口
		claims, err := jwtManager.ParseTokenOfType(raw, jwt.TokenTypeAccess)
		if err != nil {
			switch {
			case errors.Is(err, jwt.ErrTokenExpired):
				response.UnauthorizedT(c, "auth.token_expired")
			case errors.Is(err, jwt.ErrTokenMalformed):
				response.UnauthorizedT(c, "auth.token_malformed")
			case errors.Is(err, jwt.ErrTokenWrongType):
				response.UnauthorizedT(c, "auth.token_wrong_type")
			default:
				response.UnauthorizedT(c, "auth.token_invalid")
			}
			c.Abort()
			return
		}

		// 校验账号当前状态与令牌版本（带缓存，正常路径命中 Redis）
		if authState != nil {
			state, err := authState.GetAuthState(c.Request.Context(), claims.UserID)
			if err != nil {
				response.UnauthorizedT(c, "auth.state_check_failed")
				c.Abort()
				return
			}
			if state.Status == 0 {
				response.ForbiddenT(c, "auth.account_disabled")
				c.Abort()
				return
			}
			if claims.TokenVersion != state.TokenVersion {
				response.UnauthorizedT(c, "auth.token_revoked")
				c.Abort()
				return
			}
		}

		// 将用户信息存入上下文
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)

		c.Next()
	}
}

// CORSMiddleware 跨域中间件
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// RecoveryMiddleware 异常恢复中间件
func RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				response.InternalError(c, "common.internal_error")
				c.Abort()
			}
		}()
		c.Next()
	}
}
