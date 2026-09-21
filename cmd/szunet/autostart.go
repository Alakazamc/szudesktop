package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Alakazamc/szudesktop/internal/autostart"
)

// autostart 子命令：管理开机自动登录。
//
// 想解决的问题很直白——每天开机第一件事是连校园网，那就别让人再点一次。
// 打开后，登录 Windows 时会静默把 szuDesktop 拉起来（不弹界面），
// 它在后台连一次网；用户想用界面的时候再双击程序就行。
//
// 实际操作注册表的代码在 internal/autostart，桌面设置页用的是同一份。
func cmdAutostart(args []string) {
	fs := flag.NewFlagSet("autostart", flag.ExitOnError)
	on := fs.Bool("on", false, "打开开机自启")
	off := fs.Bool("off", false, "关掉开机自启")
	show := fs.Bool("status", false, "看当前状态")
	openDir := fs.Bool("open-dir", false, "在资源管理器里定位程序位置")
	targetCLI := fs.Bool("target-cli", false, "开机跑命令行版而不是界面版（默认界面版，能持续保持在线）")

	if len(args) == 0 {
		fmt.Println("用法:")
		fmt.Println("  szunet autostart           看当前状态")
		fmt.Println("  szunet autostart -on       打开：开机自动登录")
		fmt.Println("  szunet autostart -off      关掉")
		fmt.Println("  szunet autostart -open-dir 在文件夹里定位程序")
		fmt.Println()
		fmt.Println("说明: 默认开机自启同目录下的 szudesktop（界面程序），它会在后台静默连一次网，")
		fmt.Println("      并持续盯着网络状态、掉线自动补登，但不弹窗口。想手动用界面就双击它。")
		fmt.Println("      加 -target-cli 改成只跑命令行版（只登录一次，不保持）。")
		fmt.Println()
		fmt.Printf("当前状态: %s\n", autostart.Status().Detail)
		return
	}
	_ = fs.Parse(args)

	switch {
	case *on:
		if err := autostart.Enable(*targetCLI); err != nil {
			fail(err)
		}
		fmt.Println("已打开开机自启。下次登录 Windows 会自动连接校园网。")
		fmt.Printf("  注册表位置: HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Run\n")
		fmt.Printf("  当前状态  : %s\n", autostart.Status().Detail)
		fmt.Println("  提示: 开机后是静默运行，不会弹界面；想主动看界面就双击 szudesktop。")

	case *off:
		if err := autostart.Disable(); err != nil {
			fail(err)
		}
		fmt.Println("已关掉开机自启。")

	case *openDir:
		if err := autostart.OpenSelfDir(); err != nil {
			fail(err)
		}
		fmt.Println("已在文件夹里定位到程序。")

	case *show:
		fmt.Printf("当前状态: %s\n", autostart.Status().Detail)

	default:
		fmt.Fprintf(os.Stderr, "不认识的参数。跑 `szunet autostart` 看用法。\n")
		os.Exit(2)
	}
}
