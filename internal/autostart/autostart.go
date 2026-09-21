// Package autostart 管理「登录系统时自动运行 szuDesktop」。
//
// 为什么从 cmd/szunet 抽出来：桌面设置页也要显示和切换这个开关，
// 两边各写一份注册表操作迟早会走偏，而且命令行版能改、界面版看不到状态，
// 用户会以为没生效。
package autostart

import (
	"errors"
	"os"
	"path/filepath"
)

// ErrUnsupported 表示这个平台还没实现开机自启。
// 给明确错误，而不是沉默地什么都不做。
var ErrUnsupported = errors.New("开机自启目前只做了 Windows 版")

// State 是开机自启的当前状态。
type State struct {
	Supported bool   `json:"supported"` // 这个平台实现了没有
	Enabled   bool   `json:"enabled"`   // 已经登记了没有
	Detail    string `json:"detail"`    // 一句人话，界面直接显示
	Error     string `json:"error"`     // 读取失败的原因；读取失败不等于未开启
}

// unknown 表示状态读不出来。项目红线是「读不到 ≠ 没有」，
// 所以这种情况不能显示成「未开启」，否则用户会再登记一次。
func unknown(reason string) State {
	return State{
		Supported: true,
		Detail:    "状态未知：" + reason,
		Error:     reason,
	}
}

// selfPath 取自己的真实路径。走软链接启动时必须解析，
// 否则注册表里记的是个失效的链接。
func selfPath() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		p = resolved
	}
	return p, nil
}
