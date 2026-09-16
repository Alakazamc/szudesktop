package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Alakazamc/szudesktop/internal/sysproxy"
	"github.com/Alakazamc/szudesktop/internal/vpn"
)

const (
	defaultVPNServer = "ssl.szu.edu.cn:443"
	defaultSocksPort = 7891
	maxVPNLogs       = 100
)

type vpnLog struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

// vpnManager 管一条 VPN 连接。generation 用来识别已经过期的后台协程：
// 旧连接晚一步退出时，不能把刚重连的新连接或新代理一起关掉。
type vpnManager struct {
	mu         sync.Mutex
	client     *vpn.Client
	server     string
	socksAddr  string
	busy       bool
	running    bool
	generation uint64
	cancel     context.CancelFunc
	logs       []vpnLog
}

func newVPNManager() *vpnManager {
	m := &vpnManager{server: defaultVPNServer, socksAddr: fmt.Sprintf("127.0.0.1:%d", defaultSocksPort)}
	vpn.SetLogger(m.appendLog)
	if sysproxy.HasBackup() {
		m.appendLog("warn", "检测到上次运行留下的系统代理备份；可点“关闭系统代理”恢复原设置")
	}
	return m
}

func (m *vpnManager) appendLog(level, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, vpnLog{Time: time.Now().Format("15:04:05"), Level: level, Message: message})
	if len(m.logs) > maxVPNLogs {
		m.logs = append([]vpnLog(nil), m.logs[len(m.logs)-maxVPNLogs:]...)
	}
}

type vpnStatusResp struct {
	State      string         `json:"state"`
	StateLabel string         `json:"state_label"`
	Connected  bool           `json:"connected"`
	Busy       bool           `json:"busy"`
	NeedsAuth  bool           `json:"needs_auth"`
	AuthType   string         `json:"auth_type,omitempty"`
	Server     string         `json:"server"`
	SocksAddr  string         `json:"socks_addr"`
	AssignedIP string         `json:"assigned_ip"`
	LastError  string         `json:"last_error"`
	Logs       []vpnLog       `json:"logs"`
	Proxy      sysproxy.State `json:"proxy"`
}

func vpnStateName(st vpn.State) string {
	switch st {
	case vpn.StateIdle:
		return "idle"
	case vpn.StateLoggingIn:
		return "logging_in"
	case vpn.StateNeedSMS:
		return "need_sms"
	case vpn.StateNeedTOTP:
		return "need_totp"
	case vpn.StateConnecting:
		return "connecting"
	case vpn.StateConnected:
		return "connected"
	default:
		return "broken"
	}
}

func (m *vpnManager) status() vpnStatusResp {
	m.mu.Lock()
	client, server, socksAddr := m.client, m.server, m.socksAddr
	busy := m.busy
	logs := append([]vpnLog(nil), m.logs...)
	m.mu.Unlock()

	st, ip, lastErr := vpn.StateIdle, "", ""
	if client != nil {
		st, ip, lastErr = client.Status()
	}
	out := vpnStatusResp{
		State: vpnStateName(st), StateLabel: st.Label(), Connected: st == vpn.StateConnected,
		Busy: busy, Server: server, SocksAddr: socksAddr, AssignedIP: ip,
		LastError: lastErr, Logs: logs, Proxy: sysproxy.Query(),
	}
	if st == vpn.StateNeedSMS {
		out.NeedsAuth, out.AuthType = true, "sms"
	} else if st == vpn.StateNeedTOTP {
		out.NeedsAuth, out.AuthType = true, "totp"
	}
	return out
}

func normalizeVPNServer(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultVPNServer, nil
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return "", errors.New("VPN 服务器地址不对")
	}
	if u.Path != "" && u.Path != "/" {
		return "", errors.New("VPN 服务器只填域名或 host:port，不要带页面路径")
	}
	port := u.Port()
	if port == "" {
		port = "443"
	}
	if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		return "", errors.New("VPN 服务器端口不对")
	}
	return net.JoinHostPort(u.Hostname(), port), nil
}

func socksAddress(port int) (string, error) {
	if port == 0 {
		port = defaultSocksPort
	}
	if port < 1024 || port > 65535 {
		return "", errors.New("SOCKS 端口要填 1024 到 65535")
	}
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), nil
}

type vpnConnectReq struct {
	Server      string `json:"server"`
	SocksPort   int    `json:"socks_port"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	EnableProxy bool   `json:"enable_proxy"`
}

func (m *vpnManager) connect(req vpnConnectReq) (string, error) {
	server, err := normalizeVPNServer(req.Server)
	if err != nil {
		return "", err
	}
	socksAddr, err := socksAddress(req.SocksPort)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(req.Username) == "" || req.Password == "" {
		return "", errors.New("账号和密码都不能空")
	}

	m.mu.Lock()
	if m.busy || m.running {
		m.mu.Unlock()
		return "", errors.New("VPN 已在连接或运行，请先断开")
	}
	m.generation++
	gen := m.generation
	client := vpn.New(server, socksAddr)
	m.client, m.server, m.socksAddr = client, server, socksAddr
	m.busy = true
	m.logs = nil
	m.mu.Unlock()

	m.appendLog("info", "开始建立校外 VPN 连接")
	if err := client.Login(strings.TrimSpace(req.Username), req.Password); err != nil {
		m.mu.Lock()
		if m.generation == gen {
			m.busy = false
		}
		m.mu.Unlock()
		if errors.Is(err, vpn.ErrNextAuthSMS) {
			return "服务器要求短信验证码", nil
		}
		if errors.Is(err, vpn.ErrNextAuthTOTP) {
			return "服务器要求动态口令", nil
		}
		return "", err
	}
	return m.startTunnel(gen, client, req.EnableProxy)
}

func (m *vpnManager) startTunnel(gen uint64, client *vpn.Client, enableProxy bool) (string, error) {
	ctx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	if m.generation != gen || m.client != client {
		m.mu.Unlock()
		cancel()
		return "", errors.New("连接已被新的操作替换")
	}
	m.busy, m.running, m.cancel = false, true, cancel
	m.mu.Unlock()

	go func() {
		err := client.Start(ctx)
		m.mu.Lock()
		current := m.generation == gen && m.client == client
		if current {
			m.running, m.busy, m.cancel = false, false, nil
		}
		m.mu.Unlock()
		if err != nil {
			m.appendLog("error", "VPN 隧道已退出："+err.Error())
		}
		if current {
			if err := sysproxy.Disable(); err != nil {
				m.appendLog("error", "恢复系统代理失败："+err.Error())
			}
		}
	}()

	if enableProxy {
		// Start 在后台监听端口，短暂等它进入 connected，最多 3 秒。
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			m.mu.Lock()
			current := m.generation == gen && m.client == client
			m.mu.Unlock()
			if !current {
				return "", errors.New("连接已取消")
			}
			st, _, lastErr := client.Status()
			if st == vpn.StateConnected {
				if err := sysproxy.Enable(client.SocksBind); err != nil {
					cancel()
					client.Stop()
					return "", fmt.Errorf("VPN 已连上，但设置系统代理失败: %w", err)
				}
				return "VPN 已连接，系统代理已打开", nil
			}
			if st == vpn.StateBroken {
				return "", errors.New(lastErr)
			}
			time.Sleep(50 * time.Millisecond)
		}
		return "VPN 隧道正在启动；连接完成后可手动打开系统代理", nil
	}
	return "VPN 登录成功，隧道正在启动", nil
}

func (m *vpnManager) continueAuth(code string, enableProxy bool) (string, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", errors.New("验证码不能空")
	}
	m.mu.Lock()
	client, gen := m.client, m.generation
	if client == nil || m.busy || m.running {
		m.mu.Unlock()
		return "", errors.New("当前没有等待中的二步验证")
	}
	m.busy = true
	m.mu.Unlock()

	if err := client.ContinueAuth(code); err != nil {
		m.mu.Lock()
		if m.generation == gen {
			m.busy = false
		}
		m.mu.Unlock()
		return "", err
	}
	return m.startTunnel(gen, client, enableProxy)
}

func (m *vpnManager) disconnect() error {
	m.mu.Lock()
	m.generation++
	client := m.client
	m.busy, m.running = false, false
	m.mu.Unlock()
	if client != nil {
		client.Stop()
	}
	if err := sysproxy.Disable(); err != nil {
		return err
	}
	m.appendLog("info", "VPN 已断开，系统代理已恢复")
	return nil
}

func decodeJSON(r *http.Request, dst any) error {
	if r.Body == nil {
		return nil
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return errors.New("请求格式不对")
	}
	return nil
}

func writeAPIError(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(map[string]any{"ok": false, "message": err.Error()})
}

func requirePost(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodPost {
		return true
	}
	writeAPIError(w, http.StatusMethodNotAllowed, errors.New("这个操作只接受 POST"))
	return false
}

func (s *Server) handleVPNStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, s.vpn.status())
}

func (s *Server) handleVPNConnect(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	var req vpnConnectReq
	if err := decodeJSON(r, &req); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	if req.Username == "" || req.Password == "" {
		user, pass, err := s.creds()
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err)
			return
		}
		if req.Username == "" {
			req.Username = user
		}
		if req.Password == "" {
			req.Password = pass
		}
	}
	message, err := s.vpn.connect(req)
	if err != nil {
		writeAPIError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "message": message, "status": s.vpn.status()})
}

type vpnAuthReq struct {
	Code        string `json:"code"`
	EnableProxy bool   `json:"enable_proxy"`
}

func (s *Server) handleVPNAuth(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	var req vpnAuthReq
	if err := decodeJSON(r, &req); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	message, err := s.vpn.continueAuth(req.Code, req.EnableProxy)
	if err != nil {
		writeAPIError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "message": message, "status": s.vpn.status()})
}

func (s *Server) handleVPNDisconnect(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	if err := s.vpn.disconnect(); err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "message": "VPN 已断开", "status": s.vpn.status()})
}

type vpnProxyReq struct {
	Enabled bool `json:"enabled"`
}

func (s *Server) handleVPNProxy(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	var req vpnProxyReq
	if err := decodeJSON(r, &req); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	if req.Enabled {
		st := s.vpn.status()
		if !st.Connected {
			writeAPIError(w, http.StatusConflict, errors.New("VPN 还没连接，不能打开系统代理"))
			return
		}
		if err := sysproxy.Enable(st.SocksAddr); err != nil {
			writeAPIError(w, http.StatusInternalServerError, err)
			return
		}
	} else if err := sysproxy.Disable(); err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "proxy": sysproxy.Query()})
}
