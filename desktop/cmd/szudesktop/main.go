// Command szudesktop 是 szuDesktop 的桌面客户端入口。
//
// 它做的事很简单：
//  1. 把页面（desktop/assets 里那套 HTML）编译进自己
//  2. 在本机回环地址上起一个小服务，让页面上的按钮能真的驱动校园网认证
//  3. 打开浏览器，按需自动登录一次
//
// 因为不碰窗口系统，三端共用一份代码：交叉编译出 Windows / macOS / Linux
// 三个单文件，双击就跑，不用装 SDK、不用装运行时。
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Alakazamc/szudesktop/desktop/internal/ui"
	"github.com/Alakazamc/szudesktop/internal/portal"
)

const version = "beta0.6.1"

func main() {
	fs := flag.NewFlagSet("szudesktop", flag.ExitOnError)

	addr := fs.String("addr", "127.0.0.1:0", "监听地址，默认随机端口")
	user := fs.String("u", "", "校园卡号（不填就用已保存的）")
	pass := fs.String("p", "", "统一身份认证密码")
	noAuto := fs.Bool("no-auto-login", false, "启动时不自动登录")
	noOpen := fs.Bool("no-open", false, "不自动打开浏览器")
	srunHost := fs.String("host-teaching", portal.DefaultSrunHost, "教学区深澜门户")
	drcomHost := fs.String("host-dorm", portal.DefaultDrcomHost, "宿舍区 Dr.COM 门户")
	campusBackend := fs.String("campus-backend", "", "未来校内后端的 HTTPS 地址（可选）")
	zone := fs.String("zone", "auto", "强制指定区域：auto / teaching / dorm")
	showVer := fs.Bool("version", false, "看版本")

	_ = fs.Parse(os.Args[1:])

	if *showVer {
		fmt.Printf("szuDesktop %s（内嵌 szunet 内核）\n", version)
		return
	}

	srv := ui.New(ui.Options{
		Addr:          *addr,
		User:          *user,
		Password:      *pass,
		AutoLogin:     !*noAuto,
		NoOpen:        *noOpen,
		SrunHost:      *srunHost,
		DrcomHost:     *drcomHost,
		CampusBackend: *campusBackend,
		Zone:          *zone,
	})

	if err := srv.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "启动失败: %v\n", err)
		if !*noOpen {
			startupError("启动失败：" + err.Error())
		}
		os.Exit(1)
	}
}
