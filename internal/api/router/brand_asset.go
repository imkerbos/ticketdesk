// Package router: 品牌资源下发
package router

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/kerbos/ticketdesk/internal/api/response"
	"github.com/kerbos/ticketdesk/pkg/storage"
)

// handleBrandAsset 下发品牌资源（Logo / Favicon）
//
// 免认证：登录页在未登录状态下就要显示 Logo。
// 因此这里必须严格限定只能取到 brand/ 前缀下的对象 ——
// 存储根下还有 attachments/，那是需要 issue:view 权限才能下载的工单附件。
func (r *Router) handleBrandAsset(c *gin.Context) {
	// 路由是 /brand/assets/brand/*filepath，filepath 形如 "/logo_123.svg"
	rel := strings.TrimPrefix(c.Param("filepath"), "/")
	if rel == "" {
		response.NotFound(c, "apidoc.not_found")
		return
	}

	// 拼上前缀后再规范化：CleanKey 会消解 ".."，
	// 之后再校验一次前缀，确保没有借助路径穿越跳出 brand/ 目录
	key, err := storage.CleanKey(storage.PrefixBrand + "/" + rel)
	if err != nil || !strings.HasPrefix(key, storage.PrefixBrand+"/") {
		response.NotFound(c, "apidoc.not_found")
		return
	}

	obj, err := r.fileStorage.Open(c.Request.Context(), key)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			response.NotFound(c, "apidoc.not_found")
			return
		}
		response.InternalError(c, "apidoc.read_failed")
		return
	}
	defer obj.Body.Close()

	contentType := obj.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// 品牌资源变动不频繁，允许浏览器缓存；文件名本身带毫秒时间戳，更新后 URL 会变
	c.Header("Cache-Control", "public, max-age=3600")
	c.DataFromReader(http.StatusOK, obj.Size, contentType, obj.Body, nil)
}
