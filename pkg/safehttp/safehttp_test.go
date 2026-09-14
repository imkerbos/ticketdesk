package safehttp

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestBlocksCloudMetadataAndLoopback 云元数据地址与回环地址必须拒绝外连。
// 外发 Webhook / 数据源地址由管理员配置，若不拦截，
// 一个 URL 即可让服务端代为读取云实例凭证或访问仅监听本地的管理端口。
func TestBlocksCloudMetadataAndLoopback(t *testing.T) {
	blocked := []string{
		"169.254.169.254", // AWS/GCP/阿里云 实例元数据
		"169.254.170.2",   // ECS 任务元数据
		"127.0.0.1",
		"127.0.0.53",
		"::1",
		"fe80::1",
		"0.0.0.0",
	}
	for _, ip := range blocked {
		if !IsBlockedIP(net.ParseIP(ip)) {
			t.Errorf("%s 应被拦截", ip)
		}
	}
}

// TestAllowsPrivateAndPublic 内网地址必须放行：
// TicketDesk 的数据源（Prometheus / 夜莺）与自建 Webhook 接收端本来就在内网，
// 一刀切封禁 RFC1918 会直接打断核心功能。
func TestAllowsPrivateAndPublic(t *testing.T) {
	allowed := []string{
		"10.0.0.1",
		"172.16.5.4",
		"192.168.1.10",
		"8.8.8.8",
		"2400:3200::1",
	}
	for _, ip := range allowed {
		if IsBlockedIP(net.ParseIP(ip)) {
			t.Errorf("%s 不应被拦截（内网数据源是正常用法）", ip)
		}
	}
}

// TestClientRefusesLoopbackConnection 端到端验证：
// 客户端真正发起到回环地址的请求时必须失败。
func TestClientRefusesLoopbackConnection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(5 * time.Second)
	resp, err := client.Get(srv.URL) // httptest 监听 127.0.0.1
	if err == nil {
		resp.Body.Close()
		t.Fatal("到回环地址的请求应被拒绝")
	}
	if !strings.Contains(err.Error(), "安全策略") {
		t.Fatalf("期望被 SSRF 防护拦截，实际错误: %v", err)
	}
}

// TestClientAllowsNormalHost 防护不应误伤正常的对外请求（此处用非回环的本机网卡地址代替）。
func TestClientAllowsNormalHost(t *testing.T) {
	// 找一个本机的非回环 IPv4 地址；找不到则跳过（CI 环境可能只有回环）
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		t.Skipf("无法枚举网卡: %v", err)
	}
	var host string
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && ipnet.IP.To4() != nil && !IsBlockedIP(ipnet.IP) {
			host = ipnet.IP.String()
			break
		}
	}
	if host == "" {
		t.Skip("本机没有可用的非回环 IPv4 地址")
	}

	ln, err := net.Listen("tcp", host+":0")
	if err != nil {
		t.Skipf("无法在 %s 上监听: %v", host, err)
	}
	srv := &http.Server{
		Handler:           http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	resp, err := NewClient(5 * time.Second).Get("http://" + ln.Addr().String())
	if err != nil {
		t.Fatalf("到非回环地址的正常请求不应被拦截: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got %d", resp.StatusCode)
	}
}
