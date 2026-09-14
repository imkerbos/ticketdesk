package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/kerbos/ticketdesk/internal/api/response"
)

// setupAllowPrefixes 初始化完成前仍然放行的路径
//
// 只放行向导自己和健康检查：其余接口在没有管理员的状态下不该被当成正常实例使用，
// 让它们明确返回 503 比返回一堆 401 更容易判断处于什么状态。
var setupAllowPrefixes = []string{
	"/api/v1/setup",
	// 品牌配置：向导页和登录页都要用它渲染系统名称与 Logo。
	// 是 /api/v1/brand 这个免认证接口，不是 /system/configs/public ——
	// 后者名字里虽然带 public，实际要求登录态，初始化前根本没人能登。
	"/api/v1/brand",
	"/health",
	"/healthz",
	"/metrics",
}

// SetupRequired 未完成初始化时拦截业务接口
func SetupRequired(initialized func(c *gin.Context) bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, p := range setupAllowPrefixes {
			if strings.HasPrefix(c.Request.URL.Path, p) {
				c.Next()
				return
			}
		}
		if initialized(c) {
			c.Next()
			return
		}
		response.Error(c, http.StatusServiceUnavailable, "SETUP_REQUIRED", "setup.required")
		c.Abort()
	}
}
