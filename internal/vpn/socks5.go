package vpn

import (
	"context"
	"errors"
	"io"
	"net"

	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

// 本机代理服务：同时认 SOCKS5 和 SOCKS4/4a，只支持 CONNECT，不做认证。
// 协议足够简单，自写两百行，省掉为一个小工具引入 tailscale 巨型依赖。
//
// 为什么非得连 SOCKS4 一起认：Windows 的系统代理（WinINET）表示 SOCKS 代理
// 只有 "socks=host:port" 这一种写法，而 Chromium 系浏览器读到这个前缀会按
// SOCKS4 去握手，不是 SOCKS5。只认 SOCKS5 的话，「一键设置系统代理」会设得
// 看着成功、浏览器却一个页面都打不开，两边还都不报错 —— 最难查的那种故障。

// serveSocks 接收循环。listener 由调用方在 ctx 结束时关闭。
func serveSocks(ctx context.Context, ln net.Listener, ipStack *stack.Stack, selfIp []byte) error {
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		go socksHandle(conn, ipStack, selfIp)
	}
}

// socksHandle 按第一个字节把连接分流给对应版本的握手。
func socksHandle(conn net.Conn, ipStack *stack.Stack, selfIp []byte) {
	defer conn.Close()

	ver := make([]byte, 1)
	if err := readFull(conn, ver); err != nil {
		return
	}
	switch ver[0] {
	case 5:
		socks5Handle(conn, ipStack, selfIp)
	case 4:
		socks4Handle(conn, ipStack, selfIp)
	default:
		logf("warn", "本地代理收到不认识的协议（首字节 0x%02x），已拒绝", ver[0])
	}
}

// socks5Handle 处理 SOCKS5（版本字节已经读掉了）。
func socks5Handle(conn net.Conn, ipStack *stack.Stack, selfIp []byte) {
	// 1. 握手：NMETHODS METHODS → 选 NO-AUTH(0x00)
	nm := make([]byte, 1)
	if err := readFull(conn, nm); err != nil {
		return
	}
	methods := make([]byte, nm[0])
	if err := readFull(conn, methods); err != nil {
		return
	}
	ok := false
	for _, m := range methods {
		if m == 0x00 {
			ok = true
			break
		}
	}
	if !ok {
		conn.Write([]byte{5, 0xff})
		return
	}
	if _, err := conn.Write([]byte{5, 0}); err != nil {
		return
	}

	// 2. 请求：VER CMD RSV ATYP ADDR PORT（这一轮客户端会重发一次 VER）
	req := make([]byte, 4)
	if err := readFull(conn, req); err != nil {
		return
	}
	if req[1] != 1 { // 只支持 CONNECT
		conn.Write([]byte{5, 7, 0, 1, 0, 0, 0, 0, 0, 0})
		return
	}
	var host string
	switch req[3] {
	case 1: // IPv4
		b := make([]byte, 4)
		if err := readFull(conn, b); err != nil {
			return
		}
		host = net.IP(b).String()
	case 3: // 域名
		l := make([]byte, 1)
		if err := readFull(conn, l); err != nil {
			return
		}
		b := make([]byte, l[0])
		if err := readFull(conn, b); err != nil {
			return
		}
		host = string(b)
	default: // IPv6：隧道是 IPv4 的，直接拒
		conn.Write([]byte{5, 8, 0, 1, 0, 0, 0, 0, 0, 0})
		return
	}
	pb := make([]byte, 2)
	if err := readFull(conn, pb); err != nil {
		return
	}
	port := uint16(pb[0])<<8 | uint16(pb[1])

	// 3. 解析目标并通过用户态 TCP/IP 栈连进内网
	inner, fail := dialTarget(ipStack, selfIp, host, port)
	if fail != failNone {
		conn.Write([]byte{5, socks5Code(fail), 0, 1, 0, 0, 0, 0, 0, 0})
		return
	}
	defer inner.Close()

	// 4. 回成功，双向搬运
	if _, err := conn.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0}); err != nil {
		return
	}
	logf("info", "已接通 %s:%d", host, port)
	pipe(conn, inner)
}

// socks4Handle 处理 SOCKS4 / SOCKS4a（版本字节已经读掉了）。
//
// 报文：CMD(1) DSTPORT(2) DSTIP(4) USERID(以 NUL 结尾)
// 若 DSTIP 形如 0.0.0.x，说明是 SOCKS4a，USERID 后面还跟一个同样以 NUL
// 结尾的域名 —— 浏览器把域名交给代理去解析时就是这么发的。
func socks4Handle(conn net.Conn, ipStack *stack.Stack, selfIp []byte) {
	head := make([]byte, 7) // CMD + DSTPORT + DSTIP
	if err := readFull(conn, head); err != nil {
		return
	}
	if head[0] != 1 { // 只支持 CONNECT，BIND 用不上
		socks4Reply(conn, false)
		return
	}
	port := uint16(head[1])<<8 | uint16(head[2])

	if _, err := readCString(conn); err != nil { // USERID，不做认证，读掉丢弃
		return
	}

	host := net.IPv4(head[3], head[4], head[5], head[6]).String()
	if head[3] == 0 && head[4] == 0 && head[5] == 0 && head[6] != 0 {
		// SOCKS4a：真正的目标是后面那个域名
		name, err := readCString(conn)
		if err != nil || name == "" {
			socks4Reply(conn, false)
			return
		}
		host = name
	}

	inner, fail := dialTarget(ipStack, selfIp, host, port)
	if fail != failNone {
		socks4Reply(conn, false)
		return
	}
	defer inner.Close()

	if err := socks4Reply(conn, true); err != nil {
		return
	}
	logf("info", "已接通 %s:%d", host, port)
	pipe(conn, inner)
}

// socks4Reply 回 8 字节应答：VN(0) REP DSTPORT(2) DSTIP(4)。
// 后面那 6 个字节客户端不看，填零即可。
func socks4Reply(conn net.Conn, ok bool) error {
	code := byte(0x5B) // 请求被拒绝或失败
	if ok {
		code = 0x5A // 请求被允许
	}
	_, err := conn.Write([]byte{0, code, 0, 0, 0, 0, 0, 0})
	return err
}

// readCString 读一个以 NUL 结尾的字符串（SOCKS4 的 USERID 和域名字段）。
// 设个上限，防着对面拿一条不含 NUL 的长流把内存撑爆。
func readCString(conn net.Conn) (string, error) {
	buf := make([]byte, 0, 64)
	one := make([]byte, 1)
	for len(buf) < 256 {
		if err := readFull(conn, one); err != nil {
			return "", err
		}
		if one[0] == 0 {
			return string(buf), nil
		}
		buf = append(buf, one[0])
	}
	return "", errors.New("SOCKS4 字段一直没有结束符")
}

// dialFail 是连目标失败的原因。两个版本的协议各自映射成自己的应答码。
type dialFail int

const (
	failNone    dialFail = iota
	failResolve          // 域名解析不了
	failIPv6             // 目标只有 IPv6 地址，隧道是 IPv4 的，走不了
	failConnect          // 连不上
)

// dialTarget 解析目标地址并通过用户态 TCP/IP 栈连进内网。
// 域名走本地 DNS 解析；校内域名要等隧道通了才解析得动，见 vpn-notes 里的风险一节。
func dialTarget(ipStack *stack.Stack, selfIp []byte, host string, port uint16) (net.Conn, dialFail) {
	target, err := net.ResolveIPAddr("ip", host)
	if err != nil {
		logf("warn", "域名解析失败 %s: %v", host, err)
		return nil, failResolve
	}
	if target.IP.To4() == nil {
		return nil, failIPv6
	}
	inner, err := dialViaStack(ipStack, selfIp, target.IP.To4(), port)
	if err != nil {
		logf("warn", "连内网失败 %s:%d: %v", host, port, err)
		return nil, failConnect
	}
	return inner, failNone
}

// socks5Code 把失败原因翻成 SOCKS5 的应答码。
func socks5Code(f dialFail) byte {
	switch f {
	case failResolve:
		return 4 // host unreachable
	case failIPv6:
		return 8 // address type not supported
	default:
		return 1 // general SOCKS server failure
	}
}

// pipe 双向搬运，任一方向结束就收摊。
func pipe(a, b net.Conn) {
	done := make(chan struct{}, 2)
	go func() { copyBoth(a, b); done <- struct{}{} }()
	go func() { copyBoth(b, a); done <- struct{}{} }()
	<-done
}

func readFull(conn net.Conn, buf []byte) error {
	_, err := io.ReadFull(conn, buf)
	return err
}

// copyBoth 单向搬运，出错就关两边。
func copyBoth(dst net.Conn, src net.Conn) {
	buf := make([]byte, 32*1024)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				dst.Close()
				src.Close()
				return
			}
		}
		if err != nil {
			dst.Close()
			src.Close()
			return
		}
	}
}
