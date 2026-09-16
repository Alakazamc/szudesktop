// Package sysproxy 一键把系统代理指到本机 SOCKS5，断开时原样还回去。
//
// VPN 隧道本身只提供一个本地 SOCKS5 端口。想让浏览器和大部分软件真的走进
// 隧道，还得让系统知道「有个代理在这儿」。这个包干的就是这件事。
//
// 为什么要备份原值：很多人本来就挂着别的代理（公司代理、抓包工具、
// 学术镜像）。如果断开 VPN 时简单地把代理一关了事，等于顺手废掉了人家
// 原来的设置，而且用户根本不知道是谁改的。所以开之前先把原值抄下来，
// 断开时逐字还回去。
//
// 备份存在注册表里而不是内存里，因为程序可能在连接中被强杀（任务管理器、
// 关机、崩溃）。存在注册表里，下次启动还能把上一次没来得及恢复的设置捞回来。
package sysproxy

// Snapshot 是改动之前的系统代理设置，用来原样恢复。
type Snapshot struct {
	Enabled  bool   // 原来有没有开代理
	Server   string // 原来的代理地址（WinINET 的 ProxyServer 格式）
	Override string // 原来的不代理列表（ProxyOverride）
	Valid    bool   // 这份快照是不是真读出来的（false 表示没有可恢复的东西）
}

// State 是当前系统代理的状况，给界面显示用。
type State struct {
	Supported bool   `json:"supported"` // 这个平台支不支持一键设置
	Enabled   bool   `json:"enabled"`   // 系统代理当前是否开着
	Server    string `json:"server"`    // 当前代理地址
	Managed   bool   `json:"managed"`   // 是不是我们设的（有备份在案）
	Note      string `json:"note"`      // 一句人话说明
}
