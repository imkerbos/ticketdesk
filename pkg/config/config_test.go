package config

import "testing"

// TestEffectiveTrustedProxiesDefault 未配置时必须回落到内网网段，
// 而不是空列表（空列表在 Gin 里等价于「不信任任何代理」，
// 反向代理场景下会把所有请求的 ClientIP 记成 Ingress 的 IP）。
func TestEffectiveTrustedProxiesDefault(t *testing.T) {
	cfg := &AppConfig{}
	got := cfg.EffectiveTrustedProxies()

	if len(got) == 0 {
		t.Fatal("默认可信代理列表不应为空")
	}
	for _, want := range []string{"127.0.0.1/32", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"} {
		if !contains(got, want) {
			t.Fatalf("默认列表缺少 %s: %v", want, got)
		}
	}
	// 绝不能默认信任全网：那等于放弃 X-Forwarded-For 的可信度
	for _, bad := range []string{"0.0.0.0/0", "::/0"} {
		if contains(got, bad) {
			t.Fatalf("默认列表不得包含 %s", bad)
		}
	}
}

func TestEffectiveTrustedProxiesOverride(t *testing.T) {
	cfg := &AppConfig{TrustedProxies: []string{"10.42.0.0/16"}}
	got := cfg.EffectiveTrustedProxies()

	if len(got) != 1 || got[0] != "10.42.0.0/16" {
		t.Fatalf("显式配置应覆盖默认值, got %v", got)
	}
}

func contains(list []string, target string) bool {
	for _, v := range list {
		if v == target {
			return true
		}
	}
	return false
}
