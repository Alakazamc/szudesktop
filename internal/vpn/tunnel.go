package vpn

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"os"
	"sync"
	"time"

	tls "github.com/refraction-networking/utls"
)

// tlsDial 建一条「VPN 数据通道」用的 TLS 连接。
// 深信服的 VPN 隧道和 HTTPS 共用 443 端口，靠 ClientHello 的 SessionId 区分：
// 前 4 字节写死 'L','3','I','P'，其余 0，套件固定 RC4-SHA、TLS1.1。
// 这是逆向出来的约定，别"修正"它。
func tlsDial(server string) (*tls.UConn, error) {
	dialConn, err := net.Dial("tcp", server)
	if err != nil {
		return nil, err
	}
	conn := tls.UClient(dialConn, &tls.Config{InsecureSkipVerify: true}, tls.HelloCustom)

	random := make([]byte, 32)
	rand.Read(random) // 填充 ClientRandom，失败无所谓
	conn.SetClientRandom(random)
	conn.SetTLSVers(tls.VersionTLS11, tls.VersionTLS11, []tls.TLSExtension{})
	conn.HandshakeState.Hello.Vers = tls.VersionTLS11
	conn.HandshakeState.Hello.CipherSuites = []uint16{tls.TLS_RSA_WITH_RC4_128_SHA, tls.FAKE_TLS_EMPTY_RENEGOTIATION_INFO_SCSV}
	conn.HandshakeState.Hello.CompressionMethods = []uint8{0}
	conn.HandshakeState.Hello.SessionId = []byte{'L', '3', 'I', 'P', 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	return conn, nil
}

// queryIp 在数据通道上询问分配到的内网 IP。
// 返回的连接必须保持打开（fork 注释：关了后续收发握手会失败），存在 client 里直到 Stop。
func queryIp(server string, token *[48]byte) (ip []byte, conn *tls.UConn, err error) {
	logf("info", "打开数据通道 %s ……", server)
	conn, err = tlsDial(server)
	if err != nil {
		return nil, nil, err
	}
	logf("ok", "TLS 握手成功")

	msg := []byte{0x00, 0x00, 0x00, 0x00}
	msg = append(msg, token[:]...)
	msg = append(msg, []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff, 0xff, 0xff, 0xff}...)
	if _, err = conn.Write(msg); err != nil {
		conn.Close()
		return nil, nil, err
	}
	reply := make([]byte, 0x80)
	if _, err = conn.Read(reply); err != nil {
		conn.Close()
		return nil, nil, err
	}
	if reply[0] != 0x00 {
		conn.Close()
		return nil, nil, errors.New("QueryIp 响应异常（首字节非 0）")
	}
	return reply[4:8], conn, nil
}

// runRxStream 收流：0x06 握手后持续读裸 IPv4 包，交给 gVisor 栈。
// 连接断开或 ctx 取消才返回。
func runRxStream(ctx context.Context, server string, token *[48]byte, ipRev *[4]byte, ep *Endpoint, keep *connKeeper) error {
	conn, err := tlsDial(server)
	if err != nil {
		return err
	}
	keep.setRx(conn)
	defer conn.Close()

	msg := []byte{0x06, 0x00, 0x00, 0x00}
	msg = append(msg, token[:]...)
	msg = append(msg, []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}...)
	msg = append(msg, ipRev[:]...)
	if _, err := conn.Write(msg); err != nil {
		return err
	}
	reply := make([]byte, 1500)
	if _, err := conn.Read(reply); err != nil {
		return err
	}
	if reply[0] != 0x01 {
		return errors.New("收流握手响应异常")
	}
	logf("ok", "收流通道就绪")

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			conn.Close()
		case <-done:
		}
	}()
	defer close(done)

	for {
		n, err := conn.Read(reply)
		if err != nil {
			return err
		}
		if n > 0 {
			ep.WriteTo(reply[:n])
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
}

// runTxStream 发流：0x05 握手后把 gVisor 栈交来的包写进 TLS。
// 重连时重新抢占 endpoint.OnRecv。
func runTxStream(ctx context.Context, server string, token *[48]byte, ipRev *[4]byte, ep *Endpoint, keep *connKeeper) error {
	conn, err := tlsDial(server)
	if err != nil {
		return err
	}
	keep.setTx(conn)
	defer conn.Close()

	msg := []byte{0x05, 0x00, 0x00, 0x00}
	msg = append(msg, token[:]...)
	msg = append(msg, []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}...)
	msg = append(msg, ipRev[:]...)
	if _, err := conn.Write(msg); err != nil {
		return err
	}
	reply := make([]byte, 1500)
	if _, err := conn.Read(reply); err != nil {
		return err
	}
	if reply[0] != 0x02 {
		return errors.New("发流握手响应异常")
	}
	logf("ok", "发流通道就绪")

	errCh := make(chan error, 1)
	closed := make(chan struct{})
	ep.SetOnRecv(func(buf []byte) {
		select {
		case <-closed:
			return
		default:
		}
		if _, err := conn.Write(buf); err != nil {
			select {
			case errCh <- err:
			default:
			}
		}
	})
	defer ep.SetOnRecv(nil)
	defer close(closed)

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// streamLoop 带重试的流守护：断流后等 2 秒重连，最多连续 5 次，ctx 取消即退出。
func streamLoop(ctx context.Context, name string, run func() error) error {
	for attempt := 1; ; attempt++ {
		err := run()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if attempt >= 5 {
			logf("error", "%s 流连续断开 5 次，放弃：%v", name, err)
			return err
		}
		logf("warn", "%s 流断开（第 %d 次），2 秒后重连：%v", name, attempt, err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

// connKeeper 记录当前两条流的连接，Stop 时统一关掉以解除阻塞的 Read。
type connKeeper struct {
	mu sync.Mutex
	rx net.Conn
	tx net.Conn
}

func (k *connKeeper) setRx(c net.Conn) { k.mu.Lock(); k.rx = c; k.mu.Unlock() }
func (k *connKeeper) setTx(c net.Conn) { k.mu.Lock(); k.tx = c; k.mu.Unlock() }

func (k *connKeeper) closeAll() {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.rx != nil {
		k.rx.Close()
	}
	if k.tx != nil {
		k.tx.Close()
	}
}

// dumpHex 调试用（--debug 时才走）。
func dumpHex(b []byte) {
	w := hex.Dumper(os.Stdout)
	w.Write(b)
	w.Close()
}
