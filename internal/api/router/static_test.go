package router

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/kerbos/ticketdesk/internal/api/middleware"
)

// TestBrandStaticMustNotExposeAttachments 锁定品牌资源静态目录的挂载点。
//
// 曾经的写法是 rg.Static("/brand/assets", "./uploads")，把 uploads 根目录整个暴露出去。
// uploads/attachments 下是工单附件，等于任何人都能无认证下载附件，
// 绕过 /issues/:key/attachments/:id/download 上的 issue:view 校验。
func TestBrandStaticMustNotExposeAttachments(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 构造 uploads/{brand,attachments} 目录结构
	root := t.TempDir()
	uploads := filepath.Join(root, "uploads")
	for _, sub := range []string{"brand", "attachments"} {
		if err := os.MkdirAll(filepath.Join(uploads, sub), 0o755); err != nil {
			t.Fatalf("创建目录失败: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(uploads, "brand", "logo.svg"), []byte("<svg/>"), 0o644); err != nil {
		t.Fatalf("写品牌文件失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(uploads, "attachments", "secret.pdf"), []byte("CONFIDENTIAL"), 0o644); err != nil {
		t.Fatalf("写附件失败: %v", err)
	}

	engine := gin.New()
	v1 := engine.Group("/api/v1")
	// 与 registerPublicRoutes 中的挂载方式保持一致（仅根目录换成临时目录）
	brandAssets := v1.Group("/brand/assets/brand")
	brandAssets.Use(middleware.UploadedAssetSecurityHeaders())
	brandAssets.Static("", filepath.Join(uploads, "brand"))

	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w
	}

	// 品牌资源仍然可取（存量配置里写的就是这个 URL 形态）
	logoResp := get("/api/v1/brand/assets/brand/logo.svg")
	if logoResp.Code != http.StatusOK {
		t.Fatalf("品牌资源应可访问, got %d", logoResp.Code)
	}

	// 品牌资源允许 .svg 且本目录免认证，必须带 CSP + nosniff，
	// 否则上传一个内嵌 <script> 的 SVG 就是同源存储型 XSS（token 存在 localStorage）。
	if csp := logoResp.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'none'") {
		t.Fatalf("品牌资源缺少限制性 CSP, got %q", csp)
	}
	if got := logoResp.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("品牌资源缺少 nosniff, got %q", got)
	}

	// 附件必须取不到
	for _, path := range []string{
		"/api/v1/brand/assets/attachments/secret.pdf",
		"/api/v1/brand/assets/brand/../attachments/secret.pdf",
	} {
		w := get(path)
		if w.Code == http.StatusOK {
			t.Fatalf("附件通过静态路由泄露了: %s -> 200", path)
		}
		if strings.Contains(w.Body.String(), "CONFIDENTIAL") {
			t.Fatalf("附件内容泄露: %s", path)
		}
	}
}

// TestRouterDoesNotMountUploadsRoot 源码级回归护栏：
// 防止有人把挂载点改回 uploads 根目录。
func TestRouterDoesNotMountUploadsRoot(t *testing.T) {
	src, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatalf("读取 router.go 失败: %v", err)
	}
	if strings.Contains(string(src), `Static("/brand/assets", "./uploads")`) {
		t.Fatal("检测到 uploads 根目录被挂成静态资源，会导致工单附件无认证泄露")
	}
}
