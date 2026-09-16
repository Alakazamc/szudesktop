//go:build !windows

package sysproxy

import "errors"

// 非 Windows 平台先不做一键设置。
//
// macOS 要调 networksetup 且得按每个网络服务（Wi-Fi / 以太网）分别设，
// Linux 更是各桌面环境一套（GNOME 走 gsettings、KDE 走配置文件，
// 命令行程序则只认 http_proxy 环境变量）。这些都得在真机上验证才靠得住，
// 眼下 Windows 是首要目标，这里先老实说不支持，让界面把手动配置的
// 地址显示给用户，而不是假装设好了。

func HasBackup() bool { return false }

func Query() State {
	return State{
		Supported: false,
		Note:      "这个系统还不支持一键设置代理，请手动把浏览器的 SOCKS5 代理填成下面的地址",
	}
}

func Enable(socksAddr string) error {
	return errors.New("当前系统还不支持一键设置代理，请手动配置 SOCKS5：" + socksAddr)
}

func Disable() error { return nil }
