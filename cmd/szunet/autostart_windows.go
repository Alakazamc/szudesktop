// 开机自启：把 szuDesktop 挂到「登录时运行」。
//
// Windows 上最省事、也最容易被用户自己检查和删除的做法是写注册表
// HKCU\Software\Microsoft\Windows\CurrentVersion\Run。
//
// 不用「计划任务」的原因：那需要管理员权限，而且用户想关掉的时候得去
// 任务计划程序里翻，不如注册表里一行清清楚楚。
package main

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
	exeSuffix  = ".exe"
)

// 开机自启时的参数：不弹浏览器（开机就弹窗很烦），但自动登录。
// 用户可以随时双击程序手动开界面。
const autostartArgs = " --no-open"

// resolveAutostartTarget 决定开机启动哪个程序。
//
// 优先找同目录下的界面程序 szudesktop——它起的是常驻服务，
// 能一直盯着网络、掉线自动补登；命令行版只连一次。
//
// 文件名可能是 szudesktop.exe，也可能是 szudesktop-windows-amd64.exe
// （交叉编译的产物通常带平台后缀），所以按前缀找。
func resolveAutostartTarget(preferCLI bool) (string, string, error) {
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
			return cliPath, " login --auto", nil
		}
		return self, " login --auto", nil
	}
	if guiPath != "" {
		return guiPath, autostartArgs, nil
	}
	if cliPath != "" {
		return cliPath, " login --auto", nil
	}
	// 两个都没找到（可能被改名了），就用自己
	return self, autostartArgs, nil
}

func selfPath() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", err
	}
	// 走软链接启动时取真实路径，否则注册表里记的会是个失效的链接
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		p = resolved
	}
	return p, nil
}

// enableAutostart 打开开机自启。preferCLI 为真时改注册命令行版。
func enableAutostart(preferCLI bool) error {
	target, args, err := resolveAutostartTarget(preferCLI)
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

// disableAutostart 关掉开机自启。
func disableAutostart() error {
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

// autostartStatus 返回 (是否已开启, 注册表里记的命令行)。
func autostartStatus() (bool, string) {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false, ""
	}
	defer k.Close()

	v, _, err := k.GetStringValue(runValue)
	if err != nil {
		return false, ""
	}
	return true, v
}

// describeAutostart 把状态整理成一句人话，顺便指出记的路径还在不在。
func describeAutostart() string {
	on, cmd := autostartStatus()
	if !on {
		return "未开启"
	}
	parts := strings.SplitN(cmd, `"`, 3)
	if len(parts) < 2 {
		return "已开启 → " + cmd
	}
	exe := parts[1]
	if _, err := os.Stat(exe); err != nil {
		return "已开启，但程序位置变了，路径已失效（重跑一次 -on 即可修复）: " + exe
	}
	name := filepath.Base(exe)
	if strings.Contains(cmd, "--no-open") {
		return "已开启 → " + name + "（开机静默连网，不弹界面）"
	}
	return "已开启 → " + name + "（开机自动登录一次）"
}

// openSelfDir 打开程序所在目录，方便用户拖快捷方式。
func openSelfDir() error {
	self, err := selfPath()
	if err != nil {
		return err
	}
	return exec.Command("explorer", "/select,", self).Start()
}
