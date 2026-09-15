// Package vpn 实现深信服 EasyConnect SSL VPN 的第三方客户端。
//
// 协议依据开源项目 EasierConnect（lyc8503 原版，acd407 fork）逆向整理的流程移植，
// 仅用于用本人账号连接本人所属学校（深圳大学 ssl.szu.edu.cn / svpn.szu.edu.cn）的 VPN。
// 一切协议权利属深信服所有；学校服务端固件升级可能导致本包失效。
//
// 五步流程（详见 design/vpn-notes.md）：
//  1. Web 登录（RSA+CSRF）拿 TWFID，可能触发短信/TOTP 二步验证
//  2. 用 TWFID 明文 HTTP 探针换取 ECAgent token（藏在 TLS ServerHello SessionId 里）
//  3. token = ECAgent(31 字节+NUL) + TWFID(16 字节) = 48 字节
//  4. 用特制 ClientHello（SessionId 前 4 字节 "L3IP"）开 TLS 数据通道，QueryIp 拿内网 IP，
//     再开收（0x06）/发（0x05）两条块流传输裸 IPv4 包
//  5. gVisor 用户态 TCP/IP 栈终结 TCP，本地 SOCKS5 服务把流量导进隧道
package vpn

import "sync"

// State 是客户端状态机的状态。
type State int

const (
	StateIdle       State = iota // 未登录
	StateLoggingIn               // 正在登录
	StateNeedSMS                 // 等短信验证码
	StateNeedTOTP                // 等 TOTP 验证码
	StateConnecting              // 隧道握手中
	StateConnected               // 已连接（SOCKS5 服务中）
	StateBroken                  // 连上后断开（可重试 Start）
)

func (s State) Label() string {
	switch s {
	case StateIdle:
		return "未登录"
	case StateLoggingIn:
		return "登录中"
	case StateNeedSMS:
		return "等待短信验证码"
	case StateNeedTOTP:
		return "等待动态口令"
	case StateConnecting:
		return "隧道握手中"
	case StateConnected:
		return "已连接"
	default:
		return "已断开"
	}
}

// 错误：Login 返回这两个错误时，调用方收齐验证码后调 ContinueAuth。
var (
	ErrNextAuthSMS  = errStr("服务器要求短信验证码")
	ErrNextAuthTOTP = errStr("服务器要求动态口令（TOTP）")
	ErrNotPending   = errStr("当前没有等待中的二步验证")
	ErrWrongState   = errStr("状态不允许该操作")
)

type errStr string

func (e errStr) Error() string { return string(e) }

// logger 是全局日志回调。UI 设一个回调就能把过程显示到界面上；
// 不设则丢弃（CLI 场景由调用方自己接到 stdout）。
var (
	logMu sync.RWMutex
	logFn func(level, msg string)
)

// SetLogger 设置全局日志回调。level 为 "info"/"ok"/"warn"/"error"。
func SetLogger(fn func(level, msg string)) {
	logMu.Lock()
	logFn = fn
	logMu.Unlock()
}

func logf(level, format string, args ...any) {
	logMu.RLock()
	fn := logFn
	logMu.RUnlock()
	if fn != nil {
		fn(level, sprintf(format, args...))
	}
}
