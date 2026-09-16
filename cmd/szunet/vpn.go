// vpn 子命令：校外访问校园网的三条官方通道。
//
// VPN 连接本身依赖学校指定的客户端（EasyConnect / SecureLink），
// 第三方工具不应该也不需要去重新实现它们的私有协议。
// 所以这个命令做的是「引导」：把通道摆清楚、把入口打开、把状态说清。
package main

import (
	"flag"
	"fmt"
	"os/exec"
	"runtime"
)

// vpnChannel 描述一条校外访问通道。
type vpnChannel struct {
	name   string // 通道名
	url    string // 入口地址（可打开/下载）
	kind   string // web（浏览器直接用）/ client（要先装客户端）
	detail string // 怎么登录、注意什么
}

var vpnChannels = []vpnChannel{
	{
		name:   "WebVPN（网页版）",
		url:    "https://webvpn.szu.edu.cn/",
		kind:   "web",
		detail: "浏览器打开，用统一身份认证登录，登录后在页面里直接点校内系统。不用装任何客户端，想临时查个校内资源选这个最省事。",
	},
	{
		name:   "SSL VPN（EasyConnect 客户端）",
		url:    "https://ssl.szu.edu.cn/",
		kind:   "client",
		detail: "先在 ssl.szu.edu.cn 下载 EasyConnect 客户端（浏览器提示证书风险时点「继续前往」是正常的，学校这个站没换正式证书）。装好后用统一身份认证登录，登录后整台电脑都在校园网里。",
	},
	{
		name:   "零信任 SecureLink（新通道）",
		url:    "https://www.wangsu.com/app/securelink/",
		kind:   "client",
		detail: "网宿的零信任客户端。登录时企业标识填 szu（小写），选「第三方登录 → CAS」跳统一身份认证。官方说明：装它之前要先卸载电脑上其他 VPN 软件，防止冲突。",
	},
}

func cmdVPN(args []string) {
	fs := flag.NewFlagSet("vpn", flag.ExitOnError)
	open := fs.Int("open", 0, "用默认浏览器打开第 N 条通道（1=WebVPN 2=EasyConnect 3=SecureLink），不填只列出")
	asJSON := fs.Bool("json", false, "以 JSON 输出")
	_ = fs.Parse(args)

	if *asJSON {
		list := make([]map[string]any, 0, len(vpnChannels))
		for i, c := range vpnChannels {
			list = append(list, map[string]any{
				"n":      i + 1,
				"name":   c.name,
				"url":    c.url,
				"kind":   c.kind,
				"detail": c.detail,
			})
		}
		printJSON(map[string]any{"channels": list})
		return
	}

	fmt.Println("校外访问校园网，学校现在有三条通道（都要统一身份认证）：")
	fmt.Println()
	for i, c := range vpnChannels {
		kind := "网页"
		if c.kind == "client" {
			kind = "要装客户端"
		}
		fmt.Printf("%d. %s  [%s]\n", i+1, c.name, kind)
		fmt.Printf("   入口: %s\n", c.url)
		fmt.Printf("   说明: %s\n", c.detail)
		fmt.Println()
	}

	if *open > 0 {
		if *open > len(vpnChannels) {
			fail(fmt.Errorf("只有 %d 条通道", len(vpnChannels)))
		}
		if err := openBrowser(vpnChannels[*open-1].url); err != nil {
			fail(fmt.Errorf("打不开浏览器: %v（直接把上面的入口复制到浏览器也行）", err))
		}
		fmt.Printf("已在默认浏览器打开: %s\n", vpnChannels[*open-1].url)
	} else {
		fmt.Println("用 -open 1 可以直接在默认浏览器打开第 1 条（WebVPN）。")
	}

	fmt.Println()
	fmt.Println("提示: 校外 VPN 和校园网认证是两回事——VPN 是「在校外装作在校内」，")
	fmt.Println("      szunet login 管「人在校内但还没认证」。两个问题别混着排查。")
}

// openBrowser 用系统默认浏览器打开链接，三端各自的路子。
func openBrowser(url string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default: // linux 等
		return exec.Command("xdg-open", url).Start()
	}
}
