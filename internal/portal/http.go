package portal

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// newHTTPClient 构造一个专门用于校园网认证的 HTTP 客户端。
//
// 这里有两个刻意的设定，都是踩过坑才加的：
//
//   - Proxy 显式设成 nil。Go 默认会读 HTTP_PROXY / HTTPS_PROXY 环境变量，
//     系统上开着代理或加速器时，认证请求会被抓走，结果就是一直转圈或者
//     报一堆莫名其妙的错。认证必须走直连。
//
//   - 支持直接指定服务器 IP。有些代理工具会把域名解析也接管掉，
//     导致认证门户的域名根本解析不出来；这时候只能绕开 DNS 直接连 IP。
func newHTTPClient(host, serverIP string, timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second}

	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if serverIP != "" {
				_, port, err := net.SplitHostPort(addr)
				if err != nil {
					port = "80"
				}
				addr = net.JoinHostPort(serverIP, port)
			}
			return dialer.DialContext(ctx, network, addr)
		},
	}

	// 直接连 IP 时，HTTPS 握手里的域名（SNI）还得是原来的域名，
	// 否则服务端返回的证书对不上，TLS 校验会失败。
	if serverIP != "" && strings.HasPrefix(host, "https://") {
		if u, err := url.Parse(host); err == nil && u.Hostname() != "" {
			transport.TLSClientConfig = &tls.Config{ServerName: u.Hostname()}
		}
	}

	return &http.Client{Transport: transport, Timeout: timeout}
}

// parseJSONP 把 "callback({...})" 这种响应里的 JSON 部分取出来。
// 深澜和 Dr.COM 的接口都用这个格式。
func parseJSONP(body []byte) ([]byte, error) {
	s := strings.TrimSpace(string(body))
	start := strings.Index(s, "(")
	end := strings.LastIndex(s, ")")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("响应不是预期的 JSONP 格式: %s", truncate(s, 160))
	}
	return []byte(s[start+1 : end]), nil
}

// truncate 把过长的文本截断，避免错误信息刷屏。
func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
