// Command szunet 是深圳大学校园网的命令行工具。
//
// 它做两件事：
//
//  1. 自动登录。教学区的深澜（SRun）和宿舍区的 Dr.COM 都支持，自动判断你在哪个区。
//  2. 连不上的时候告诉你是哪一步出了问题。
//
// 同一份代码交叉编译出 Windows / macOS / Linux 三个平台的单文件程序，
// 不需要装运行时，下载下来直接就能跑。
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Alakazamc/szunet/internal/credential"
	"github.com/Alakazamc/szunet/internal/diagnose"
	"github.com/Alakazamc/szunet/internal/portal"
)

const version = "0.1.0"

// options 是所有子命令共用的参数。
type options struct {
	user      string
	password  string
	zone      string
	srunHost  string
	drcomHost string
	acID      string
	serverIP  string
	asJSON    bool
	verbose   bool
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "login":
		cmdLogin(args)
	case "logout":
		cmdLogout(args)
	case "status":
		cmdStatus(args)
	case "detect":
		cmdDetect(args)
	case "diag":
		cmdDiag(args)
	case "config":
		cmdConfig(args)
	case "version", "-v", "--version":
		fmt.Printf("szunet %s\n", version)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "不认识的命令: %s\n\n", cmd)
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Print(`szunet - 深圳大学校园网命令行工具

用法:
  szunet login      登录（自动判断你在教学区还是宿舍区）
  szunet logout     注销当前会话
  szunet status     看当前在哪个区、账号在不在线
  szunet detect     只探测网络区域
  szunet diag       连不上时跑这个，给出排查结论
  szunet config     管理保存的账号密码
  szunet version    看版本

常用参数:
  -u, --user        校园卡号（6 位）
  -p, --password    统一身份认证密码
  --zone            强制指定区域：auto（默认）/ teaching / dorm
  --ip              直接指定认证服务器 IP，绕过域名解析
  --ac-id           指定深澜的 ac_id（教学区，一般不用手动给）
  --json            以 JSON 形式输出，方便脚本调用
  --verbose         把服务端原始返回也打出来

先把账号存起来（推荐）:
  szunet config set -u 2023xxxx -p 你的密码

也可以临时用参数或环境变量:
  szunet login -u 2023xxxx -p 你的密码
  SZUNET_USERNAME=2023xxxx SZUNET_PASSWORD=你的密码 szunet login

说明: 本工具是第三方作品，与深圳大学无关。别和官方客户端同时用，会互相踢下线。
`)
}

func addCommonFlags(fs *flag.FlagSet, o *options) {
	fs.StringVar(&o.user, "u", "", "校园卡号（6 位）")
	fs.StringVar(&o.user, "user", "", "校园卡号（6 位）")
	fs.StringVar(&o.password, "p", "", "统一身份认证密码")
	fs.StringVar(&o.password, "password", "", "统一身份认证密码")
	fs.StringVar(&o.zone, "zone", "auto", "强制指定区域：auto / teaching / dorm")
	fs.StringVar(&o.srunHost, "host-teaching", portal.DefaultSrunHost, "教学区深澜门户地址")
	fs.StringVar(&o.drcomHost, "host-dorm", portal.DefaultDrcomHost, "宿舍区 Dr.COM 门户地址")
	fs.StringVar(&o.acID, "ac-id", "", "深澜的 ac_id（一般不用给）")
	fs.StringVar(&o.serverIP, "ip", "", "直接指定认证服务器 IP，绕过域名解析")
	fs.BoolVar(&o.asJSON, "json", false, "以 JSON 输出")
	fs.BoolVar(&o.verbose, "verbose", false, "打印服务端原始返回")
}

// resolveCredentials 按「命令行参数 > 环境变量 > 已保存的凭据」的顺序取账号密码。
func resolveCredentials(o *options) (string, string, error) {
	user, pass := o.user, o.password

	if user == "" {
		user = os.Getenv("SZUNET_USERNAME")
	}
	if pass == "" {
		pass = os.Getenv("SZUNET_PASSWORD")
	}

	if user == "" || pass == "" {
		if c, err := credential.Default().Load(); err == nil {
			if user == "" {
				user = c.Username
			}
			if pass == "" {
				pass = c.Password
			}
		}
	}

	if user == "" || pass == "" {
		return "", "", fmt.Errorf(
			"没有可用的账号密码。先跑一次 `szunet config set` 存起来，" +
				"或者用 -u / -p 临时指定，也可以设环境变量 SZUNET_USERNAME 和 SZUNET_PASSWORD")
	}
	return user, pass, nil
}

// pickZone 决定用哪个区域的协议。
// 默认自动探测；用户在 --zone 里指定了就听用户的。
func pickZone(o *options) (portal.Zone, *portal.DetectResult) {
	det := portal.Detect()

	switch o.zone {
	case "", "auto":
		return det.Zone, det
	case "teaching", "srun":
		return portal.ZoneTeaching, det
	case "dorm", "dormitory", "drcom":
		return portal.ZoneDorm, det
	default:
		return det.Zone, det
	}
}

func cmdLogin(args []string) {
	fs := flag.NewFlagSet("login", flag.ExitOnError)
	var o options
	addCommonFlags(fs, &o)
	_ = fs.Parse(args)

	user, pass, err := resolveCredentials(&o)
	if err != nil {
		fail(err)
	}

	zone, det := pickZone(&o)

	switch zone {
	case portal.ZoneOnline:
		if o.asJSON {
			printJSON(map[string]any{"ok": true, "zone": zone, "message": "已经能上外网，不需要认证"})
			return
		}
		fmt.Println("当前已经能上外网，不需要认证")
		return

	case portal.ZoneTeaching:
		c := portal.NewSrunClient(o.srunHost, user, pass)
		if o.acID != "" {
			c.AcID = o.acID
		}
		if o.serverIP != "" {
			c.SetServerIP(o.serverIP)
		}
		res, err := c.Login()
		reportResult(res, err, zone, o.asJSON, o.verbose)

	case portal.ZoneDorm:
		c := portal.NewDrcomClient(o.drcomHost, user, pass)
		if o.serverIP != "" {
			c.SetServerIP(o.serverIP)
		}
		res, err := c.Login()
		reportResult(res, err, zone, o.asJSON, o.verbose)

	default:
		if det != nil {
			for _, n := range det.Notes {
				fmt.Fprintln(os.Stderr, "· "+n)
			}
		}
		fail(fmt.Errorf("判断不出你在哪个区。如果确定在校内，可以手动指定：" +
			"szunet login --zone dorm 或 --zone teaching"))
	}
}

func cmdLogout(args []string) {
	fs := flag.NewFlagSet("logout", flag.ExitOnError)
	var o options
	addCommonFlags(fs, &o)
	_ = fs.Parse(args)

	user, pass, err := resolveCredentials(&o)
	if err != nil {
		fail(err)
	}

	zone, _ := pickZone(&o)

	switch zone {
	case portal.ZoneTeaching:
		res, err := portal.NewSrunClient(o.srunHost, user, pass).Logout()
		reportResult(res, err, zone, o.asJSON, o.verbose)
	case portal.ZoneDorm:
		res, err := portal.NewDrcomClient(o.drcomHost, user, pass).Logout()
		reportResult(res, err, zone, o.asJSON, o.verbose)
	default:
		fail(fmt.Errorf("不在校园网内，没有可注销的会话"))
	}
}

func cmdStatus(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	var o options
	addCommonFlags(fs, &o)
	_ = fs.Parse(args)

	user, pass, credErr := resolveCredentials(&o)
	zone, det := pickZone(&o)

	out := map[string]any{
		"zone":            zone,
		"zone_label":      zone.Label(),
		"internet_ok":     det.InternetOK,
		"dorm_portal_ok":  det.DormPortalOK,
		"teaching_portal": det.TeachPortalOK,
	}

	var status *portal.OnlineStatus
	if credErr == nil && zone != portal.ZoneOnline && zone != portal.ZoneOutside {
		switch zone {
		case portal.ZoneTeaching:
			status, _ = portal.NewSrunClient(o.srunHost, user, pass).Status()
		case portal.ZoneDorm:
			status, _ = portal.NewDrcomClient(o.drcomHost, user, pass).Status()
		}
	}
	if status != nil {
		out["online"] = status.Online
		out["online_ip"] = status.IP
	}

	if o.asJSON {
		printJSON(out)
		return
	}

	fmt.Printf("网络区域: %s\n", zone.Label())
	fmt.Printf("外网连通: %s\n", yesNo(det.InternetOK))
	if status != nil {
		fmt.Printf("账号状态: %s\n", onlineText(status.Online))
		if status.IP != "" {
			fmt.Printf("在线 IP : %s\n", status.IP)
		}
	} else if credErr != nil {
		fmt.Printf("账号状态: 没查到（%v）\n", credErr)
	} else {
		fmt.Printf("账号状态: 没查到\n")
	}
}

func cmdDetect(args []string) {
	fs := flag.NewFlagSet("detect", flag.ExitOnError)
	var o options
	addCommonFlags(fs, &o)
	_ = fs.Parse(args)

	det := portal.Detect()

	if o.asJSON {
		printJSON(map[string]any{
			"zone":            det.Zone,
			"zone_label":      det.Zone.Label(),
			"internet_ok":     det.InternetOK,
			"dorm_portal_ok":  det.DormPortalOK,
			"teaching_portal": det.TeachPortalOK,
			"srun_dns_ok":     det.SrunDNSOK,
			"notes":           det.Notes,
		})
		return
	}

	fmt.Printf("网络区域: %s\n", det.Zone.Label())
	fmt.Printf("外网连通: %s\n", yesNo(det.InternetOK))
	fmt.Printf("宿舍门户: %s\n", yesNo(det.DormPortalOK))
	fmt.Printf("教学门户: %s\n", yesNo(det.TeachPortalOK))
	fmt.Printf("域名解析: %s\n", yesNo(det.SrunDNSOK))
	if len(det.Notes) > 0 {
		fmt.Println()
		for _, n := range det.Notes {
			fmt.Println("· " + n)
		}
	}
}

func cmdDiag(args []string) {
	fs := flag.NewFlagSet("diag", flag.ExitOnError)
	var o options
	addCommonFlags(fs, &o)
	_ = fs.Parse(args)

	user, pass, _ := resolveCredentials(&o)
	rep := diagnose.Run(user, pass, o.srunHost, o.drcomHost)

	if o.asJSON {
		out := map[string]any{
			"zone":            rep.Detect.Zone,
			"zone_label":      rep.Detect.Zone.Label(),
			"internet_ok":     rep.Detect.InternetOK,
			"dorm_portal_ok":  rep.Detect.DormPortalOK,
			"teaching_portal": rep.Detect.TeachPortalOK,
			"srun_dns_ok":     rep.Detect.SrunDNSOK,
			"notes":           rep.Detect.Notes,
			"advices":         rep.Advices,
		}
		if rep.Online != nil {
			out["online"] = rep.Online.Online
			out["online_ip"] = rep.Online.IP
		}
		printJSON(out)
		return
	}

	fmt.Println("=== 网络区域 ===")
	fmt.Printf("  判定结果: %s\n", rep.Detect.Zone.Label())
	fmt.Printf("  外网连通: %s\n", yesNo(rep.Detect.InternetOK))
	fmt.Printf("  宿舍门户: %s\n", yesNo(rep.Detect.DormPortalOK))
	fmt.Printf("  教学门户: %s\n", yesNo(rep.Detect.TeachPortalOK))
	fmt.Printf("  域名解析: %s\n", yesNo(rep.Detect.SrunDNSOK))

	if rep.Online != nil {
		fmt.Println()
		fmt.Println("=== 账号状态 ===")
		fmt.Printf("  是否在线: %s\n", onlineText(rep.Online.Online))
	}

	if len(rep.Advices) > 0 {
		fmt.Println()
		fmt.Println("=== 排查建议 ===")
		for i, a := range rep.Advices {
			fmt.Printf("  %d. %s\n", i+1, a)
		}
	}

	if o.verbose {
		fmt.Println()
		fmt.Println("=== 探测细节 ===")
		for _, n := range rep.Detect.Notes {
			fmt.Println("  · " + n)
		}
	}
}

func cmdConfig(args []string) {
	if len(args) < 1 {
		fmt.Println("用法:")
		fmt.Println("  szunet config set -u 2023xxxx -p 你的密码   保存账号密码")
		fmt.Println("  szunet config show                        看当前存在哪、存的什么账号")
		fmt.Println("  szunet config delete                      删掉保存的账号密码")
		os.Exit(2)
	}

	store := credential.Default()

	switch args[0] {
	case "set":
		fs := flag.NewFlagSet("config set", flag.ExitOnError)
		var o options
		addCommonFlags(fs, &o)
		_ = fs.Parse(args[1:])

		user := o.user
		if user == "" {
			fmt.Print("校园卡号（6 位）: ")
			reader := bufio.NewReader(os.Stdin)
			line, _ := reader.ReadString('\n')
			user = strings.TrimSpace(line)
		}

		pass := o.password
		if pass == "" {
			fmt.Print("统一身份认证密码（输入时会显示在屏幕上）: ")
			reader := bufio.NewReader(os.Stdin)
			line, _ := reader.ReadString('\n')
			pass = strings.TrimSpace(line)
		}

		if user == "" || pass == "" {
			fail(fmt.Errorf("账号和密码都不能为空"))
		}

		if err := store.Save(credential.Credentials{Username: user, Password: pass}); err != nil {
			fail(err)
		}
		fmt.Printf("已保存，存放方式: %s\n", store.Describe())

	case "show":
		c, err := store.Load()
		if err != nil {
			fail(err)
		}
		fmt.Printf("存放方式: %s\n", store.Describe())
		fmt.Printf("账号    : %s\n", c.Username)
		fmt.Println("密码    : 已保存（不显示）")

	case "delete":
		if err := store.Delete(); err != nil {
			fail(err)
		}
		fmt.Println("已删除保存的账号密码")

	default:
		fmt.Fprintf(os.Stderr, "不认识的子命令: %s\n", args[0])
		os.Exit(2)
	}
}

// reportResult 统一处理一次认证操作的输出。
func reportResult(res *portal.Result, err error, zone portal.Zone, asJSON, verbose bool) {
	if err != nil {
		fail(err)
	}

	if asJSON {
		out := map[string]any{
			"ok":      res.OK,
			"zone":    zone,
			"message": res.Message,
		}
		if verbose {
			out["raw"] = res.Raw
		}
		printJSON(out)
	} else {
		if res.OK {
			fmt.Printf("[成功] %s（%s）\n", res.Message, zone.Label())
		} else {
			fmt.Printf("[失败] %s\n", res.Message)
		}
		if verbose && res.Raw != "" {
			fmt.Printf("服务端原始返回: %s\n", res.Raw)
		}
	}

	if !res.OK {
		os.Exit(1)
	}
}

func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "出错: %v\n", err)
	os.Exit(1)
}

func yesNo(b bool) string {
	if b {
		return "是"
	}
	return "否"
}

func onlineText(b bool) string {
	if b {
		return "已在线"
	}
	return "不在线"
}
