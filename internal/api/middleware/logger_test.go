package middleware

import (
	"net/url"
	"strings"
	"testing"
)

// TestRedactQueryHidesCredentials /ws 用 ?token= 传 JWT，
// 直接把 RawQuery 写进访问日志等于把可用凭证明文落盘。
func TestRedactQueryHidesCredentials(t *testing.T) {
	cases := []struct {
		name   string
		raw    string
		secret string
	}{
		{"ws token", "token=eyJhbGciOiJIUzI1NiJ9.payload.sig", "eyJhbGciOiJIUzI1NiJ9.payload.sig"},
		{"pat", "token=td_pat_abcdef123456", "td_pat_abcdef123456"},
		{"refresh token", "refresh_token=rt_secret_value", "rt_secret_value"},
		{"oauth code", "code=authcode_secret", "authcode_secret"},
		{"password", "password=hunter2", "hunter2"},
		{"mixed", "page=1&token=leak_me&size=20", "leak_me"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := redactQuery(tc.raw)
			if strings.Contains(got, tc.secret) {
				t.Fatalf("凭证泄露到日志: redactQuery(%q) = %q", tc.raw, got)
			}
			if !strings.Contains(got, "REDACTED") {
				t.Fatalf("期望出现 [REDACTED] 占位，实际: %q", got)
			}
		})
	}
}

// TestRedactQueryKeepsHarmlessParams 非敏感参数必须原样保留，否则会丢失排障信息。
func TestRedactQueryKeepsHarmlessParams(t *testing.T) {
	raw := "page=2&page_size=20&project_key=OPS"
	got := redactQuery(raw)
	if got != raw {
		t.Fatalf("无敏感参数时不应改写: got %q, want %q", got, raw)
	}
}

func TestRedactQueryEmpty(t *testing.T) {
	if got := redactQuery(""); got != "" {
		t.Fatalf("空 query 应返回空串, got %q", got)
	}
}

// TestRedactQueryPreservesNonSensitiveAlongsideSecret 混合场景下保留可排障字段。
func TestRedactQueryPreservesNonSensitiveAlongsideSecret(t *testing.T) {
	got := redactQuery("page=3&token=leak_me")
	values, err := url.ParseQuery(got)
	if err != nil {
		t.Fatalf("脱敏结果无法解析: %v", err)
	}
	if values.Get("page") != "3" {
		t.Fatalf("非敏感参数被破坏: %q", got)
	}
	if values.Get("token") != "[REDACTED]" {
		t.Fatalf("token 未脱敏: %q", got)
	}
}
