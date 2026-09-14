// Package middleware: 用户上传的静态资源的安全响应头
package middleware

import (
	"github.com/gin-gonic/gin"
)

// UploadedAssetSecurityHeaders 给「用户上传后再对外提供」的静态文件加固响应头。
//
// 品牌资源允许上传 .svg，而 /api/v1/brand/assets/** 是无需认证即可访问的静态目录。
// SVG 被浏览器直接导航打开时按 image/svg+xml 渲染，内部的 <script> 会在
// 本应用同源下执行 —— 而访问令牌就存在 localStorage 里，等于一次上传换全站会话劫持。
//
// 这里用 CSP 把该响应的能力降到最低：不允许任何脚本 / 取数 / 框架嵌入，
// 只放行内联样式和 data: 图片，SVG 作为图标展示的效果不受影响。
// 同时 nosniff 阻止 MIME 猜测，避免上传的内容被当成别的类型解析。
func UploadedAssetSecurityHeaders() gin.HandlerFunc {
	const csp = "default-src 'none'; img-src 'self' data:; style-src 'unsafe-inline'; sandbox"

	return func(c *gin.Context) {
		c.Header("Content-Security-Policy", csp)
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Next()
	}
}
