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

	"github.com/SzuDesktopTeam/szudesktop/internal/credential"
	"github.com/SzuDesktopTeam/szudesktop/internal/diagnose"
	"github.com/SzuDesktopTeam/szudesktop/internal/netpref"
	"github.com/SzuDesktopTeam/szudesktop/internal/portal"
	"github.com/SzuDesktopTeam/szudesktop/internal/version"
)

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
	auto      bool
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
	case "vpn":
		cmdVPN(args)
	case "autostart":
		cmdAutostart(args)
	case "config":
		cmdConfig(args)
	case "version", "-v", "--version":
		fmt.Printf("szunet %s\n", version.Current)
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
  szunet vpn        校外访问校园网的三条通道（WebVPN / EasyConnect / 零信任）
  szunet autostart  开机自动登录（Windows：写注册表启动项）
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
	// 兼容开关，没有实际作用：login 本来就是非交互的（只用已保存的凭据，
	// 失败只反映在退出码上）。早期版本的开机自启登记的是 `szunet login --auto`，
	// 而那时 login 并不认识 --auto，flag 遇到未知参数会 os.Exit(2)，
	// 于是那些已经写进用户注册表的启动项一直在静默失败。留着这个开关，
	// 是为了让这些旧启动项不改注册表也能恢复正常。新登记不再带它。
	fs.BoolVar(&o.auto, "auto", false, "兼容旧的「开机自启」登记项，无实际作用")
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

// authenticationZone keeps explicit user choices ahead of automatic probing.
func authenticationZone(o *options, det *portal.DetectResult) portal.Zone {
	switch o.zone {
	case "teaching", "srun":
		return portal.ZoneTeaching
	case "dorm", "dormitory", "drcom":
		return portal.ZoneDorm
	default:
		return det.AuthenticationZone()
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

	det := portal.Probe()
	zone := authenticationZone(&o, det)

	switch zone {
	case portal.ZoneTeaching, portal.ZoneDorm:
		res, err := loginByZone(zone, &o, user, pass)
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

// loginByZone 按区域挑对应协议发一次认证请求。
func loginByZone(zone portal.Zone, o *options, user, pass string) (*portal.Result, error) {
	switch zone {
	case portal.ZoneDorm:
		c := portal.NewDrcomClient(o.drcomHost, user, pass)
		if o.serverIP != "" {
			c.SetServerIP(o.serverIP)
		}
		return c.Login()
	default:
		c := portal.NewSrunClient(o.srunHost, user, pass)
		if o.acID != "" {
			c.AcID = o.acID
		}
		if o.serverIP != "" {
			c.SetServerIP(o.serverIP)
		}
		attachAcIDCache(c)
		return c.Login()
	}
}

// attachAcIDCache 让客户端复用上次这张网成功的 ac_id，成功后写回缓存。
//
// ac_id 跟着"插哪个墙口 / 走哪条线路"变，所以缓存键用出口标识（网关优先），
// 而不是写死一个值。换网后缓存命中不了，客户端会自动重新发现。
//
// 只缓存"可信来源"的结果：猜出来的值不写盘（见 portal.AcIDSource），
// 否则会把一次侥幸固化下来，下次在别的网络里继续用错值。
func attachAcIDCache(c *portal.SrunClient) {
	prefs := netpref.Load()
	key := netpref.Egress()
	if id := prefs.AcIDFor(key); id != "" {
		c.SetLastAcID(id)
	}
	c.OnAcIDResolved = func(id string) {
		prefs.SetAcID(key, id)
		_ = prefs.Save()
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

	zone := authenticationZone(&o, portal.Probe())

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

// queryOnline 是命令行查询在线状态的入口，做成变量方便测试替换。
var queryOnline = portal.QueryOnline

// statusOnline 查出当前出口的在线状态，命令行与桌面端 /api/status 用的是同一个门户查询。
//
// 有没有账号都要查：门户是按出口 IP 回答的，跟本机存没存账号无关，
// 而用户问的恰恰是「我这个出口到底认证了没」。
func statusOnline(zone portal.Zone, o *options, user, pass string) (*portal.OnlineStatus, error) {
	return queryOnline(zone, o.srunHost, o.drcomHost, user, pass)
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

	// 联网时也照查（QueryOnline 的注释里写了为什么），有没有账号都要查：
	// 门户按出口 IP 回答，跟本机存没存账号无关。以前这里写的是
	// if credErr == nil { 查 }，没存账号的人永远只看到「没查到」，
	// 会以为自己掉线——F20 在桌面端修过同一个毛病，CLI 这条漏了
	// （2026-09-21 在校园网里实测发现：桌面端报「已在线」，CLI 报「没查到」）。
	status, statusErr := statusOnline(zone, &o, user, pass)
	out["online_known"] = status != nil
	if status != nil {
		out["online"] = status.Online
		out["online_ip"] = status.IP
		out["online_devices"] = status.DeviceTotal
	}

	if o.asJSON {
		printJSON(out)
		return
	}

	fmt.Printf("网络区域: %s\n", zone.Label())
	fmt.Printf("外网连通: %s\n", yesNo(det.InternetOK))
	switch {
	case status != nil:
		fmt.Printf("账号状态: %s\n", onlineText(status.Online))
		if status.IP != "" {
			fmt.Printf("在线 IP : %s\n", status.IP)
		}
		if len(status.Devices) > 0 {
			fmt.Printf("在线设备: %s\n", strings.Join(status.Devices, "、"))
		}
	case statusErr != nil:
		fmt.Printf("账号状态: 没查到（%v）\n", statusErr)
	default:
		fmt.Printf("账号状态: 没查到\n")
	}
	// 没存账号时补一句怎么登录；但状态本身照报，不能把「没查到」当结论。
	if credErr != nil && status != nil && !status.Online {
		fmt.Println()
		fmt.Println("提示: 本机没有保存账号。要登录先跑 `szunet config set`，或用 -u / -p 临时指定。")
	}
}

func cmdDetect(args []string) {
	fs := flag.NewFlagSet("detect", flag.ExitOnError)
	var o options
	addCommonFlags(fs, &o)
	_ = fs.Parse(args)

	// detect 用 Probe()：哪怕现在能上外网，也把门户和指纹跑完，
	// 这样输出里不会出现"没跑过"被误读成"探不到"的假 false。
	det := portal.Probe()

	// 顺带把 ac_id 也算出来给用户看。
	//
	// 这东西跟着"插哪个墙口 / 走哪条线路"变，是校内认证最常见的
	// 失败原因（报 Unknow ac-type），但界面上以前完全看不到它，
	// 排查时只能靠猜。这里显式打出来，并说明它可不可信。
	user, pass, _ := resolveCredentials(&o)
	acID, acIDSource := detectAcID(&o, user, pass)

	if o.asJSON {
		printJSON(map[string]any{
			"zone":            det.Zone,
			"zone_label":      det.Zone.Label(),
			"internet_ok":     det.InternetOK,
			"probed":          det.Probed,
			"dorm_portal_ok":  det.DormPortalOK,
			"teaching_portal": det.TeachPortalOK,
			"srun_usable":     det.SrunUsable,
			"dorm_usable":     det.DormUsable,
			"srun_dns_ok":     det.SrunDNSOK,
			"ac_id":           acID,
			"ac_id_source":    string(acIDSource),
			"ac_id_trusted":   acIDSource != portal.AcIDSourceGuess,
			"notes":           det.Notes,
		})
		return
	}

	fmt.Printf("网络区域: %s\n", det.Zone.Label())
	fmt.Printf("外网连通: %s\n", yesNo(det.InternetOK))
	fmt.Printf("宿舍门户: %s\n", yesNo(det.DormPortalOK))
	fmt.Printf("教学门户: %s\n", yesNo(det.TeachPortalOK))
	fmt.Printf("深澜指纹: %s\n", yesNo(det.SrunUsable))
	fmt.Printf("ePortal指纹: %s\n", yesNo(det.DormUsable))
	fmt.Printf("域名解析: %s\n", yesNo(det.SrunDNSOK))
	fmt.Printf("接入点  : %s\n", describeAcID(acID, acIDSource))

	if len(det.Notes) > 0 {
		fmt.Println()
		for _, n := range det.Notes {
			fmt.Println("· " + n)
		}
	}
}

// detectAcID 算一次接入点编号，并说明它是否可信。
func detectAcID(o *options, user, pass string) (string, portal.AcIDSource) {
	c := portal.NewSrunClient(o.srunHost, user, pass)
	if o.acID != "" {
		c.AcID = o.acID
	}
	if o.serverIP != "" {
		c.SetServerIP(o.serverIP)
	}
	attachAcIDCache(c)
	return c.ResolveAcIDWithSource()
}

// describeAcID 把接入点编号和它的可信度讲成人话。
func describeAcID(acID string, source portal.AcIDSource) string {
	switch source {
	case portal.AcIDSourceManual:
		return acID + "（你手动指定的）"
	case portal.AcIDSourceCache:
		return acID + "（这张网上次认证成功用的）"
	case portal.AcIDSourceRedirect:
		return acID + "（网关跳转里读出来的，可信）"
	default:
		return acID + "（⚠️ 猜的，不一定对。掉线时点登录，让网关自己告诉我们才准）"
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
			"probed":          rep.Detect.Probed,
			"dorm_portal_ok":  rep.Detect.DormPortalOK,
			"teaching_portal": rep.Detect.TeachPortalOK,
			"srun_usable":     rep.Detect.SrunUsable,
			"dorm_usable":     rep.Detect.DormUsable,
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
	fmt.Printf("  深澜指纹: %s\n", yesNo(rep.Detect.SrunUsable))
	fmt.Printf("  ePortal指纹: %s\n", yesNo(rep.Detect.DormUsable))
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
