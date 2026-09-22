// 开机自启：把 szuDesktop 挂到「登录时运行」。
//
// Windows 上最省事、也最容易被用户自己检查和删除的做法是写注册表
// HKCU\Software\Microsoft\Windows\CurrentVersion\Run。
//
// 不用「计划任务」的原因：那需要管理员权限，而且用户想关掉的时候得去
// 任务计划程序里翻，不如注册表里一行清清楚楚。
package autostart

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValue   = "szuDesktop"
)

// 开机自启时的参数：不弹浏览器（开机就弹窗很烦），但自动登录。
// 用户可以随时双击程序手动开界面。
const autostartArgs = " --no-open"

// 命令行版开机自启的参数。
//
// 只放 szunet 真正认识的子命令：`szunet login` 本身就是非交互的——它按
// 「命令行参数 > 环境变量 > 已保存凭据」取账号，失败只反映在退出码上，
// 没有任何需要用户确认的提示，所以不需要额外的开关。
//
// 这里原来写的是 `login --auto`，而 login 从来没有 --auto 这个参数。szunet 的
// flag 集用的是 ExitOnError，遇到未知参数会直接 os.Exit(2)：于是「用命令行版
// 开机自启」这条路上程序什么都没做就退出了，用户看到的是一个静默失效的开关。
// 注册的命令行必须能被 szunet 真的接受，改这里时请对着 cmd/szunet 的 flag 核对。
const cliLoginArgs = " login"

// CLILoginArgs 返回命令行版开机自启登记的参数。
// 导出只为一件事：让 cmd/szunet 的测试能断言「登记的参数 login 一定认得」。
func CLILoginArgs() string { return cliLoginArgs }

// Status 读注册表里的登记情况。
//
// 读不到和没登记是两件事：没登记返回「未开启」，读不到返回状态未知，
// 否则界面会告诉用户「未开启」，用户再点一次开关，实际上什么都没修好。
func Status() State {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return State{Supported: true, Detail: "未开启"}
		}
		return unknown("打不开注册表启动项：" + err.Error())
	}
	defer k.Close()

	cmd, _, err := k.GetStringValue(runValue)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return State{Supported: true, Detail: "未开启"}
		}
		return unknown("读不到启动项内容：" + err.Error())
	}
	return State{Supported: true, Enabled: true, Detail: describe(cmd)}
}

// describe 把登记的命令行整理成一句人话，顺便指出记的路径还在不在。
func describe(cmd string) string {
	parts := strings.SplitN(cmd, `"`, 3)
	if len(parts) < 2 {
		return "已开启 → " + cmd
	}
	exe := parts[1]
	if _, err := os.Stat(exe); err != nil {
		return "已开启，但程序位置变了，路径已失效（重新打开一次开关即可修复）: " + exe
	}
	name := filepath.Base(exe)
	if strings.Contains(cmd, "--no-open") {
		return "已开启 → " + name + "（开机静默连网，不弹界面）"
	}
	return "已开启 → " + name + "（开机自动登录一次）"
}

// Enable 打开开机自启。preferCLI 为真时改登记命令行版。
func Enable(preferCLI bool) error {
	target, args, err := resolveTarget(preferCLI)
	if err != nil {
		return err
	}

	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("打不开注册表启动项: %w", err)
	}
	defer k.Close()

	cmd := `"` + target + `"` + args
	if err := k.SetStringValue(runValue, cmd); err != nil {
		return fmt.Errorf("写注册表失败: %w", err)
	}
	return nil
}

// Disable 关掉开机自启。本来就没登记不算失败。
func Disable() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		return err
	}
	defer k.Close()

	if err := k.DeleteValue(runValue); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return fmt.Errorf("删注册表项失败: %w", err)
	}
	return nil
}

// OpenSelfDir 打开程序所在目录，方便用户拖快捷方式。
func OpenSelfDir() error {
	self, err := selfPath()
	if err != nil {
		return err
	}
	return exec.Command("explorer", "/select,", self).Start()
}

// resolveTarget 决定开机启动哪个程序。
//
// 优先找同目录下的界面程序 szudesktop——它起的是常驻服务，
// 能一直盯着网络、掉线自动补登；命令行版只连一次。
//
// 文件名可能是 szudesktop.exe，也可能是 szudesktop-windows-amd64.exe
// （交叉编译的产物通常带平台后缀），所以按前缀找。
func resolveTarget(preferCLI bool) (string, string, error) {
	self, err := selfPath()
	if err != nil {
		return "", "", fmt.Errorf("找不到自己在哪里: %w", err)
	}
	dir := filepath.Dir(self)

	find := func(prefix string) string {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return ""
		}
		// 先精确匹配，再找前缀匹配里名字最短的（避免匹配到 .old 之类）
		var best string
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			n := strings.ToLower(e.Name())
			if !strings.HasPrefix(n, prefix) || !strings.HasSuffix(n, ".exe") {
				continue
			}
			if best == "" || len(n) < len(best) {
				best = e.Name()
			}
		}
		if best == "" {
			return ""
		}
		return filepath.Join(dir, best)
	}

	guiPath := find("szudesktop")
	cliPath := find("szunet")

	if preferCLI {
		if cliPath != "" {
			return cliPath, cliLoginArgs, nil
		}
		return self, cliLoginArgs, nil
	}
	if guiPath != "" {
		return guiPath, autostartArgs, nil
	}
	if cliPath != "" {
		return cliPath, cliLoginArgs, nil
	}
	// 两个都没找到（可能被改名了），就用自己
	return self, autostartArgs, nil
}
