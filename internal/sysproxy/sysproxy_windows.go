//go:build windows

package sysproxy

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// Windows 的系统代理设置在这个注册表键里（当前用户，不需要管理员权限）。
// 浏览器（Edge/Chrome）和绝大多数走系统设置的软件都读它。
const inetPath = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

// 改动之前的原值抄在这里。
//
// 为什么不放内存：程序可能被任务管理器强杀、被关机打断、或者崩了。
// 那种情况下内存里的备份跟着一起没了，用户的代理设置就永远停在我们改过的
// 样子——他还不知道是谁改的。存注册表的话，下次启动一看有备份没清掉，
// 就知道上回没善后，可以直接还原。
const backupPath = `Software\szuDesktop\ProxyBackup`

// InternetSetOptionW 的两个通知码：光改注册表不通知，已经开着的浏览器
// 不会重新读设置，得等重启才生效。这两个调用让改动立刻起作用。
const (
	optSettingsChanged = 39 // INTERNET_OPTION_SETTINGS_CHANGED
	optRefresh         = 37 // INTERNET_OPTION_REFRESH
)

var (
	wininet              = windows.NewLazySystemDLL("wininet.dll")
	procInternetSetOptio = wininet.NewProc("InternetSetOptionW")
)

// notifyChanged 告诉系统「代理设置变了，重新读一次」。
func notifyChanged() {
	// 失败也不当错误处理：设置已经落到注册表了，最坏情况是用户重开一下
	// 浏览器才生效，没必要为此让整个连接流程失败。
	_, _, _ = procInternetSetOptio.Call(0, optSettingsChanged, 0, 0)
	_, _, _ = procInternetSetOptio.Call(0, optRefresh, 0, 0)
}

// readInet 读当前的系统代理设置。
func readInet() (enabled bool, server, override string) {
	k, err := registry.OpenKey(registry.CURRENT_USER, inetPath, registry.QUERY_VALUE)
	if err != nil {
		return false, "", ""
	}
	defer k.Close()

	if v, _, err := k.GetIntegerValue("ProxyEnable"); err == nil {
		enabled = v != 0
	}
	server, _, _ = k.GetStringValue("ProxyServer")
	override, _, _ = k.GetStringValue("ProxyOverride")
	return
}

// saveBackup 把原值抄进我们自己的键。
func saveBackup(s Snapshot) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, backupPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("写代理备份失败: %w", err)
	}
	defer k.Close()

	var on uint32
	if s.Enabled {
		on = 1
	}
	if err := k.SetDWordValue("Enabled", on); err != nil {
		return err
	}
	if err := k.SetStringValue("Server", s.Server); err != nil {
		return err
	}
	return k.SetStringValue("Override", s.Override)
}

// loadBackup 取出备份。第二个返回值表示有没有备份。
func loadBackup() (Snapshot, bool) {
	k, err := registry.OpenKey(registry.CURRENT_USER, backupPath, registry.QUERY_VALUE)
	if err != nil {
		return Snapshot{}, false
	}
	defer k.Close()

	var s Snapshot
	v, _, err := k.GetIntegerValue("Enabled")
	if err != nil {
		return Snapshot{}, false
	}
	s.Enabled = v != 0
	s.Server, _, _ = k.GetStringValue("Server")
	s.Override, _, _ = k.GetStringValue("Override")
	s.Valid = true
	return s, true
}

func dropBackup() {
	// 删不掉不算致命：下次 Enable 会覆盖，Disable 也只是多还原一次同样的值。
	_ = registry.DeleteKey(registry.CURRENT_USER, backupPath)
}

// HasBackup 报告有没有「还没还原」的备份。
// 启动时为真，说明上一次运行没能正常善后（被强杀或崩了）。
func HasBackup() bool {
	_, ok := loadBackup()
	return ok
}

// Query 返回当前系统代理状况。
func Query() State {
	enabled, server, _ := readInet()
	_, managed := loadBackup()

	st := State{Supported: true, Enabled: enabled, Server: server, Managed: managed}
	switch {
	case managed && enabled:
		st.Note = "系统代理由 szuDesktop 接管中，断开 VPN 会自动还回原来的设置"
	case enabled:
		st.Note = "系统里本来就挂着代理：" + server
	default:
		st.Note = "系统代理没开，全部流量直连"
	}
	return st
}

// Enable 把系统代理指到本机的 SOCKS 端口。socksAddr 形如 127.0.0.1:7891。
//
// 注意 WinINET 这个 socks= 前缀的历史包袱：Chromium 读到它会按 SOCKS4
// 发握手，不是 SOCKS5。所以隧道那侧的 SOCKS 服务必须同时听得懂 SOCKS4，
// 否则这里设完看着成功、浏览器却一个页面都打不开。
func Enable(socksAddr string) error {
	if socksAddr == "" {
		return errors.New("没有给 SOCKS 地址")
	}

	// 先抄原值。已经有备份就别覆盖——那是我们自己设的代理，
	// 覆盖等于把用户真正的原始设置弄丢了。
	if _, ok := loadBackup(); !ok {
		enabled, server, override := readInet()
		if err := saveBackup(Snapshot{Enabled: enabled, Server: server, Override: override, Valid: true}); err != nil {
			return err
		}
	}

	k, err := registry.OpenKey(registry.CURRENT_USER, inetPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("打不开系统代理设置: %w", err)
	}
	defer k.Close()

	if err := k.SetStringValue("ProxyServer", "socks="+socksAddr); err != nil {
		return fmt.Errorf("写代理地址失败: %w", err)
	}
	// 本机地址和校内直连域名不走代理，否则访问 127.0.0.1 上的界面自己
	// 都要绕一圈，而且容易绕出死循环。
	if err := k.SetStringValue("ProxyOverride", "<local>;localhost;127.*"); err != nil {
		return fmt.Errorf("写例外列表失败: %w", err)
	}
	if err := k.SetDWordValue("ProxyEnable", 1); err != nil {
		return fmt.Errorf("打开代理开关失败: %w", err)
	}

	notifyChanged()
	return nil
}

// Disable 把系统代理还原成我们改之前的样子。没有备份就什么都不做。
func Disable() error {
	s, ok := loadBackup()
	if !ok {
		return nil // 没动过，没什么可还的
	}

	k, err := registry.OpenKey(registry.CURRENT_USER, inetPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("打不开系统代理设置: %w", err)
	}
	defer k.Close()

	// 逐字还原：原来有代理就把地址填回去，原来没有就把开关关掉。
	if err := k.SetStringValue("ProxyServer", s.Server); err != nil {
		return err
	}
	if err := k.SetStringValue("ProxyOverride", s.Override); err != nil {
		return err
	}
	var on uint32
	if s.Enabled {
		on = 1
	}
	if err := k.SetDWordValue("ProxyEnable", on); err != nil {
		return err
	}

	dropBackup()
	notifyChanged()
	return nil
}
