//go:build windows

package main

import (
	"flag"
	"io"
	"strings"
	"testing"

	"github.com/SzuDesktopTeam/szudesktop/internal/autostart"
)

// 开机自启登记的命令行，必须能被它指向的子命令真的解析。
//
// 历史教训：这里登记过 `login --auto`，而 login 从来没有 --auto 这个参数。
// szunet 的 flag 集用的是 ExitOnError，遇到未知开关会直接 os.Exit(2)，
// 于是「用命令行版开机自启」在开机时什么都没做就退出了 —— 一个静默失效的开关，
// 而且不会有任何报错浮到用户面前。
//
// 用 ContinueOnError 起一个和 login 同样配置的 flag 集来解析登记的参数，
// 让这类「注册了没人认识的参数」在测试阶段就暴露。
func TestAutostartCLIArgsAreAcceptedByLogin(t *testing.T) {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var o options
	addCommonFlags(fs, &o)

	args := strings.Fields(autostart.CLILoginArgs())
	if len(args) == 0 || args[0] != "login" {
		t.Fatalf("开机自启登记的参数必须从 login 子命令开始，实际是 %q", autostart.CLILoginArgs())
	}
	if err := fs.Parse(args[1:]); err != nil {
		t.Fatalf("开机自启登记的参数 login 不认：%v（参数：%q）", err, autostart.CLILoginArgs())
	}
}

// --auto 是给早期版本写下的注册表项留的兼容开关：它仍然必须被接受，
// 否则那些用户不改注册表就修不好。
func TestLoginStillAcceptsLegacyAutoFlag(t *testing.T) {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var o options
	addCommonFlags(fs, &o)
	if err := fs.Parse([]string{"--auto"}); err != nil {
		t.Fatalf("login 必须继续接受 --auto（旧开机自启项依赖它）：%v", err)
	}
}
