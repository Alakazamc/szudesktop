package vpn

import (
	"context"
	"errors"
	"net"
	"sync"
)

// Client 是一条 VPN 连接的完整生命周期管理。
// 用法（UI 后端 / CLI 通用）：
//
//	c := vpn.New("ssl.szu.edu.cn:443", "127.0.0.1:7891")
//	vpn.SetLogger(func(level, msg string) { ... 显示到界面 ... })
//	if err := c.Login(user, pass); errors.Is(err, vpn.ErrNextAuthSMS) { ...收验证码... c.ContinueAuth(code) }
//	go c.Start(ctx)   // 隧道 + SOCKS5，直到 ctx 取消或 Stop
type Client struct {
	Server    string // host:port，如 ssl.szu.edu.cn:443
	SocksBind string // SOCKS5 监听地址，默认 127.0.0.1:7891

	mu      sync.Mutex
	state   State
	lastErr string
	ip      net.IP

	user  string
	pass  string
	twfId string
	token *[48]byte
	needs State // 待验证类型（NeedSMS/NeedTOTP），StateIdle = 无

	queryConn closer
	keep      *connKeeper
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// closer 只关心能关掉的连接（utls.UConn 等）。
type closer interface{ Close() error }

// New 建客户端。server 形如 "ssl.szu.edu.cn:443"。
func New(server, socksBind string) *Client {
	if socksBind == "" {
		socksBind = "127.0.0.1:7891"
	}
	return &Client{
		Server:    server,
		SocksBind: socksBind,
		keep:      &connKeeper{},
	}
}

// State 返回当前状态、分配到的内网 IP 和最近一次错误。
func (c *Client) Status() (State, string, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ip := ""
	if c.ip != nil {
		ip = c.ip.String()
	}
	return c.state, ip, c.lastErr
}

func (c *Client) setState(s State, lastErr string) {
	c.mu.Lock()
	c.state = s
	if lastErr != "" {
		c.lastErr = lastErr
	} else if s == StateConnected {
		c.lastErr = ""
	}
	c.mu.Unlock()
}

// Login 提交账号密码。返回 ErrNextAuthSMS / ErrNextAuthTOTP 时，
// 收到验证码后调 ContinueAuth；返回 nil 则可以直接 Start。
func (c *Client) Login(user, pass string) error {
	c.mu.Lock()
	c.user, c.pass = user, pass
	c.state = StateLoggingIn
	c.mu.Unlock()
	logf("info", "登录 %s（账号 %s）", c.Server, user)

	twfId, err := webLogin(c.Server, user, pass)
	if err != nil {
		if errors.Is(err, ErrNextAuthSMS) {
			c.mu.Lock()
			c.twfId, c.state, c.needs = twfId, StateNeedSMS, StateNeedSMS
			c.mu.Unlock()
			return ErrNextAuthSMS
		}
		if errors.Is(err, ErrNextAuthTOTP) {
			c.mu.Lock()
			c.twfId, c.state, c.needs = twfId, StateNeedTOTP, StateNeedTOTP
			c.mu.Unlock()
			return ErrNextAuthTOTP
		}
		c.setState(StateBroken, err.Error())
		return err
	}
	return c.loginByTwfId(twfId)
}

// ContinueAuth 提交二步验证码（短信或动态口令，按登录时的要求）。
func (c *Client) ContinueAuth(code string) error {
	c.mu.Lock()
	needs, twfId := c.needs, c.twfId
	c.mu.Unlock()
	if needs != StateNeedSMS && needs != StateNeedTOTP {
		return ErrNotPending
	}

	var err error
	var newTwf string
	if needs == StateNeedSMS {
		newTwf, err = authSms(c.Server, twfId, code)
	} else {
		newTwf, err = authTOTP(c.Server, twfId, code)
	}
	if err != nil {
		c.setState(StateBroken, err.Error())
		return err
	}
	return c.loginByTwfId(newTwf)
}

// loginByTwfId 换 ECAgent token、拼 48 字节 token、问内网 IP。
func (c *Client) loginByTwfId(twfId string) error {
	c.setState(StateConnecting, "")
	agent, err := ecAgentToken(c.Server, twfId)
	if err != nil {
		c.setState(StateBroken, err.Error())
		return err
	}
	c.mu.Lock()
	c.twfId = twfId
	c.token = (*[48]byte)([]byte(agent + twfId))
	c.mu.Unlock()

	ip, conn, err := queryIp(c.Server, c.token)
	if err != nil {
		c.setState(StateBroken, err.Error())
		return err
	}
	c.mu.Lock()
	c.ip = append(net.IP(nil), ip...)
	c.queryConn = conn
	c.needs = StateIdle
	c.mu.Unlock()
	logf("ok", "分配到内网 IP %s", net.IP(ip).String())
	c.setState(StateConnecting, "")
	return nil
}

// AssignedIP 返回登录后分配的内网 IP（未登录返回 nil）。
func (c *Client) AssignedIP() net.IP {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ip
}

// Start 建隧道并开 SOCKS5 服务，直到 ctx 取消或 Stop。
// 必须在 Login/ContinueAuth 成功之后调用。
func (c *Client) Start(parent context.Context) error {
	c.mu.Lock()
	token, ip := c.token, c.ip
	c.mu.Unlock()
	if token == nil || ip == nil {
		return ErrWrongState
	}

	ctx, cancel := context.WithCancel(parent)
	c.mu.Lock()
	c.cancel = cancel
	c.state = StateConnecting
	c.mu.Unlock()

	endpoint := &Endpoint{}
	ipStack := setupStack(ip, endpoint)
	ipRev := [4]byte{ip[3], ip[2], ip[1], ip[0]}

	// 收发两条流守护（断线自动重连）
	c.wg.Add(2)
	go func() {
		defer c.wg.Done()
		streamLoop(ctx, "收", func() error { return runRxStream(ctx, c.Server, token, &ipRev, endpoint, c.keep) })
	}()
	go func() {
		defer c.wg.Done()
		streamLoop(ctx, "发", func() error { return runTxStream(ctx, c.Server, token, &ipRev, endpoint, c.keep) })
	}()

	ln, err := net.Listen("tcp", c.SocksBind)
	if err != nil {
		cancel()
		c.keep.closeAll()
		c.setState(StateBroken, "SOCKS5 端口监听失败："+err.Error())
		return err
	}
	logf("ok", "SOCKS5 代理已就绪 %s（把浏览器代理指到这里即可访问校内资源）", c.SocksBind)
	c.setState(StateConnected, "")

	serveErr := make(chan error, 1)
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		serveErr <- serveSocks(ctx, ln, ipStack, ip)
	}()

	select {
	case <-ctx.Done():
		ln.Close()
		c.keep.closeAll()
		c.mu.Lock()
		if c.queryConn != nil {
			c.queryConn.Close()
		}
		c.mu.Unlock()
		c.wg.Wait()
		logf("info", "VPN 已停止")
		c.setState(StateIdle, "")
		return nil
	case err := <-serveErr:
		ln.Close()
		c.keep.closeAll()
		c.setState(StateBroken, err.Error())
		return err
	}
}

// Stop 主动断开。
func (c *Client) Stop() {
	c.mu.Lock()
	cancel := c.cancel
	c.mu.Unlock()
	if cancel != nil {
		logf("info", "正在断开 VPN ……")
		cancel()
	}
}
