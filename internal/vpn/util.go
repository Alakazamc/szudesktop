package vpn

import (
	"fmt"
	"io"
	"net"
	"strings"
)

func sprintf(format string, args ...any) string {
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}

// SetLogger 的便捷包装：接到标准输出（CLI 用）。
func LogToStdout() {
	SetLogger(func(level, msg string) {
		switch level {
		case "ok":
			println("[OK] " + msg)
		case "warn":
			println("[!!] " + msg)
		case "error":
			println("[XX] " + msg)
		default:
			println("[..] " + msg)
		}
	})
}

// dialer 是所有 HTTPS 请求共用的拨号器：学校 VPN 用自签证书，跳过校验。
// （EasierConnect 同款做法；对自签网关没有更优雅的通用解。）

// readAll 把连接读到关闭为止（fork 的 ECAgentToken 需要读满两个响应）。
func readAll(c net.Conn, limit int) ([]byte, error) {
	buf := make([]byte, 0, limit)
	tmp := make([]byte, 4096)
	for {
		n, err := c.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if err != nil {
			if err == io.EOF {
				return buf, nil
			}
			return buf, err
		}
		if len(buf) >= limit {
			return buf, nil
		}
	}
}

// firstLine 取 HTTP 响应第一行，调试日志用。
func firstLine(b []byte) string {
	s := string(b)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}
