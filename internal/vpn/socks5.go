package vpn

import (
	"context"
	"io"
	"net"

	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

// SOCKS5 服务（无认证，仅 CONNECT）。协议足够简单，自写 ~120 行，
// 省掉为一个小工具引入 tailscale 巨型依赖。

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
		go socksHandle(ctx, conn, ipStack, selfIp)
	}
}

// socksHandle 处理一条 SOCKS5 连接。
func socksHandle(ctx context.Context, conn net.Conn, ipStack *stack.Stack, selfIp []byte) {
	defer conn.Close()

	// 1. 握手：VER NMETHODS METHODS → 选 NO-AUTH(0x00)
	hdr := make([]byte, 2)
	if err := readFull(conn, hdr); err != nil {
		return
	}
	if hdr[0] != 5 {
		return // 不是 SOCKS5
	}
	methods := make([]byte, hdr[1])
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

	// 2. 请求：VER CMD RSV ATYP ADDR PORT
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

	// 3. 解析域名（本地 DNS；校内域名要等隧道通了才解析得动，见 vpn-notes 风险）
	target, err := net.ResolveIPAddr("ip", host)
	if err != nil {
		logf("warn", "域名解析失败 %s: %v", host, err)
		conn.Write([]byte{5, 4, 0, 1, 0, 0, 0, 0, 0, 0})
		return
	}
	if target.IP.To4() == nil {
		conn.Write([]byte{5, 8, 0, 1, 0, 0, 0, 0, 0, 0})
		return
	}

	// 4. 通过用户态 TCP/IP 栈连内网
	inner, err := dialViaStack(ipStack, selfIp, target.IP.To4(), port)
	if err != nil {
		logf("warn", "连内网失败 %s:%d: %v", host, port, err)
		conn.Write([]byte{5, 1, 0, 1, 0, 0, 0, 0, 0, 0})
		return
	}
	defer inner.Close()

	// 5. 回成功，双向搬运
	if _, err := conn.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0}); err != nil {
		return
	}
	logf("info", "已接通 %s:%d", host, port)

	done := make(chan struct{}, 2)
	go func() { copyBoth(conn, inner); done <- struct{}{} }()
	go func() { copyBoth(inner, conn); done <- struct{}{} }()
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
