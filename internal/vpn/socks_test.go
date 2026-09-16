package vpn

import (
	"io"
	"net"
	"testing"
	"time"
)

// 这组测试盯的是一个很容易悄悄退回去的坑：本机代理必须同时认 SOCKS4 和 SOCKS5。
//
// Windows 的系统代理只有 "socks=host:port" 一种写法，Chromium 读到它会按
// SOCKS4 握手。如果哪天有人把入口改回「首字节不是 5 就 return」，
// 「一键设置系统代理」就会变成设了也上不了网，而且日志里一个字都不会报。
//
// 这里只测握手阶段 —— 到 dialTarget 之前就能分出胜负，不需要真隧道，
// 所以 stack 传 nil 也碰不到。

// pipePair 起一个 handler 在管道另一头，返回测试这边的连接。
func pipePair(t *testing.T) net.Conn {
	t.Helper()
	mine, theirs := net.Pipe()
	go socksHandle(theirs, nil, nil)
	t.Cleanup(func() { mine.Close() })
	_ = mine.SetDeadline(time.Now().Add(3 * time.Second))
	return mine
}

// SOCKS4 的非 CONNECT 请求应该收到一个规规矩矩的 0x5B 拒绝应答。
// 关键不在「被拒绝」，而在于它走进了 SOCKS4 分支、按 SOCKS4 的格式回话 ——
// 这正是修复前做不到的事（首字节 4 会被直接丢弃、连接静默关闭）。
func TestSocks4Dispatch(t *testing.T) {
	conn := pipePair(t)

	// VER=4 CMD=2(BIND，故意不是 CONNECT) PORT=80 IP=1.2.3.4
	if _, err := conn.Write([]byte{4, 2, 0, 80, 1, 2, 3, 4}); err != nil {
		t.Fatalf("写请求失败: %v", err)
	}

	reply := make([]byte, 8)
	if _, err := io.ReadFull(conn, reply); err != nil {
		t.Fatalf("SOCKS4 请求没有拿到应答（修复前就是这样静默断开的）: %v", err)
	}
	if reply[0] != 0 || reply[1] != 0x5B {
		t.Errorf("应答格式不对: 期待 VN=0 REP=0x5B, 实际 VN=%d REP=0x%02X", reply[0], reply[1])
	}
}

// SOCKS5 的路不能被这次改动带坏：客户端不提供 NO-AUTH 时应回 0xFF。
func TestSocks5StillWorks(t *testing.T) {
	conn := pipePair(t)

	// VER=5 NMETHODS=1 METHOD=0x02(GSSAPI，我们不支持)
	if _, err := conn.Write([]byte{5, 1, 0x02}); err != nil {
		t.Fatalf("写握手失败: %v", err)
	}

	reply := make([]byte, 2)
	if _, err := io.ReadFull(conn, reply); err != nil {
		t.Fatalf("SOCKS5 握手没有应答: %v", err)
	}
	if reply[0] != 5 || reply[1] != 0xff {
		t.Errorf("期待 [5 0xff]，实际 [%d 0x%02X]", reply[0], reply[1])
	}
}

// 不认识的协议就该干净地关掉，不要挂着不动、也不要乱回字节。
func TestSocksUnknownVersion(t *testing.T) {
	conn := pipePair(t)

	if _, err := conn.Write([]byte{3}); err != nil {
		t.Fatalf("写失败: %v", err)
	}
	buf := make([]byte, 1)
	if _, err := io.ReadFull(conn, buf); err == nil {
		t.Errorf("不认识的协议不该回数据，却收到 0x%02X", buf[0])
	}
}

// readCString 是 SOCKS4 里 USERID / 域名字段的解析器。
// 正常情况读到 NUL 为止；一直没有 NUL 时必须自己截断，
// 不能让对面拿一条长流把内存撑爆。
func TestReadCString(t *testing.T) {
	t.Run("正常读到结束符", func(t *testing.T) {
		mine, theirs := net.Pipe()
		defer mine.Close()
		go func() {
			mine.Write([]byte("szu.edu.cn\x00tail"))
		}()
		_ = theirs.SetDeadline(time.Now().Add(3 * time.Second))

		got, err := readCString(theirs)
		if err != nil {
			t.Fatalf("不该出错: %v", err)
		}
		if got != "szu.edu.cn" {
			t.Errorf("期待 szu.edu.cn，实际 %q", got)
		}
	})

	t.Run("没有结束符时截断报错", func(t *testing.T) {
		mine, theirs := net.Pipe()
		defer mine.Close()
		go func() {
			junk := make([]byte, 512)
			for i := range junk {
				junk[i] = 'x'
			}
			mine.Write(junk)
		}()
		_ = theirs.SetDeadline(time.Now().Add(3 * time.Second))

		if _, err := readCString(theirs); err == nil {
			t.Error("一直没有 NUL，应该报错而不是无限读下去")
		}
	})
}
