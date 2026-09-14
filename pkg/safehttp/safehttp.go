// Package safehttp 提供带 SSRF 防护的 HTTP 客户端
//
// 适用场景：目标地址由用户（管理员）配置，由服务端发起请求 ——
// 外发 Webhook、告警数据源健康检查与轮询等。
//
// 设计取舍：TicketDesk 是运维工具，数据源和通知端点本来就大量位于内网
// （Prometheus 在 10.x、自建 Webhook 接收端等），因此**不**封禁 RFC1918 私网段，
// 否则会直接打断核心功能。这里只封掉没有任何正当用途、
// 且是 SSRF 主要变现目标的地址：
//   - 云厂商实例元数据服务（169.254.169.254 等链路本地地址），可窃取云凭证
//   - 回环地址，用于访问仅监听 127.0.0.1 的管理端口
//   - 未指定地址与组播地址
//
// 校验放在 Dialer.Control 里、针对实际连接的 IP 执行，
// 因此对 DNS 重绑定（先解析成公网 IP 通过校验、再解析到内网）同样有效。
package safehttp

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"syscall"
	"time"
)

// ErrBlockedAddress 目标地址被 SSRF 防护拦截
type ErrBlockedAddress struct {
	IP string
}

func (e *ErrBlockedAddress) Error() string {
	return fmt.Sprintf("目标地址 %s 被安全策略拒绝（链路本地/回环地址不可作为外发目标）", e.IP)
}

// IsBlockedIP 判断 IP 是否属于禁止外连的范围
func IsBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	switch {
	case ip.IsLoopback():
		// 127.0.0.0/8、::1 —— 只监听本地的管理端口
		return true
	case ip.IsLinkLocalUnicast(), ip.IsLinkLocalMulticast():
		// 169.254.0.0/16、fe80::/10 —— 含云元数据服务 169.254.169.254
		return true
	case ip.IsUnspecified():
		// 0.0.0.0、:: —— 在部分系统上等价于回环
		return true
	case ip.IsInterfaceLocalMulticast(), ip.IsMulticast():
		return true
	}
	return false
}

// controlFn 在建立连接前校验实际目标 IP
func controlFn(_ string, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("解析目标地址失败: %w", err)
	}
	ip := net.ParseIP(host)
	if IsBlockedIP(ip) {
		return &ErrBlockedAddress{IP: host}
	}
	return nil
}

// NewClient 创建带 SSRF 防护的 HTTP 客户端
func NewClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
		Control:   controlFn,
	}

	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.DialContext(ctx, network, addr)
			},
			MaxIdleConns:          50,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
		// 跟随跳转时同样要过 Control 校验；这里额外限制跳转次数，
		// 防止被用来做长跳转链耗尽连接
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("重定向次数过多")
			}
			return nil
		},
	}
}
