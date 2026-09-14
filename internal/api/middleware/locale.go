// Package middleware: 请求语言协商
package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/kerbos/ticketdesk/pkg/i18n"
)

// LocaleQueryParam 允许用 ?lang=en-US 覆盖，便于调试与分享特定语言的链接
const LocaleQueryParam = "lang"

// LocaleMiddleware 协商请求语言并写入 context
//
// 优先级：显式 query 参数 > Accept-Language 头 > 默认语言。
// 语言随 request context 往下传，service 层取错误文案时才能拿到，
// 不必在每层函数签名里多带一个参数。
func LocaleMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := i18n.DefaultLang

		if q := c.Query(LocaleQueryParam); q != "" {
			lang = i18n.ParseAcceptLanguage(q)
		} else if h := c.GetHeader("Accept-Language"); h != "" {
			lang = i18n.ParseAcceptLanguage(h)
		}

		// 同时写进 gin context 与 request context：
		// 前者供 handler 直接取用，后者供 service / repository 层透传
		c.Set("lang", string(lang))
		c.Request = c.Request.WithContext(i18n.WithLang(c.Request.Context(), lang))

		// 告诉缓存层响应随语言变化，避免 CDN / 反代把中文响应发给英文用户
		c.Writer.Header().Add("Vary", "Accept-Language")

		c.Next()
	}
}
