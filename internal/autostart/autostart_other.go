//go:build !windows

// 非 Windows 平台还没做开机自启（macOS 要写 LaunchAgent，Linux 要写 systemd user unit）。
// 这里给出明确的提示，而不是沉默地什么都不做。
package autostart

// Status 报告这个平台还没实现，不假装「未开启」。
func Status() State {
	return State{Detail: "未开启（这个平台还没做）"}
}

func Enable(bool) error { return ErrUnsupported }

func Disable() error { return ErrUnsupported }

func OpenSelfDir() error { return ErrUnsupported }
