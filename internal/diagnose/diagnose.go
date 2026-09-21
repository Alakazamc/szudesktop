// Package diagnose 把区域探测、在线状态和建议整合成一份诊断报告。
//
// 这个包存在的意义：大部分"连不上"的问题，答案不在登录脚本里，
// 而在"我在哪个区、账号什么状态、卡在哪一步"。
// 同类工具基本都只做登录，不回答这些问题。
package diagnose

import (
	"fmt"

	"github.com/SzuDesktopTeam/szudesktop/internal/portal"
)

// Report 是一次诊断的完整结果。
type Report struct {
	Detect    *portal.DetectResult
	Online    *portal.OnlineStatus // 没查或查不到时为 nil
	OnlineErr error
	Advices   []string
}

// Run 执行一次诊断。
// username / password 为空时跳过在线状态查询，只做网络侧探测。
//
// 这里用 portal.Probe() 而不是 portal.Detect()：诊断要回答的是
// "万一下一秒掉线，程序会认为我在哪个区"，这个答案在已经联网时
// 只有把探测跑完才知道。用 Detect() 会因为提前返回而给出假的"探不到"。
func Run(username, password, srunHost, drcomHost string) *Report {
	r := &Report{}
	r.Detect = portal.Probe()

	if r.Detect.Zone == portal.ZoneOnline {
		r.Advices = append(r.Advices, "当前能正常上外网。如果只是想上网，不用做任何事")
		// 已经在线时不会去认证，但掉线重登走的正是这套判区，
		// 所以把预判结论单独报出来，让人现在就能确认。
		r.Advices = append(r.Advices, fingerprintAdvice(r.Detect))
		return r
	}

	// 有凭据的话，顺便查一下账号在这个区是不是已经在线。
	if username != "" && password != "" {
		switch r.Detect.Zone {
		case portal.ZoneTeaching:
			st, err := portal.NewSrunClient(srunHost, username, password).Status()
			if err != nil {
				r.OnlineErr = err
			} else {
				r.Online = st
			}
		case portal.ZoneDorm:
			st, err := portal.NewDrcomClient(drcomHost, username, password).Status()
			if err != nil {
				r.OnlineErr = err
			} else {
				r.Online = st
			}
		}
	}

	r.Advices = advices(r)
	return r
}

// fingerprintAdvice 把协议指纹的结论说成人话。
//
// 已经在线时区域探测会短路，判区结果看不见，而掉线重登恰恰要用它，
// 所以单独做一条说明，方便在线状态下也能验判区对不对。
func fingerprintAdvice(d *portal.DetectResult) string {
	if d == nil {
		return ""
	}
	var zone string
	switch {
	case d.SrunUsable && !d.DormUsable:
		zone = "教学区（深澜）"
	case d.DormUsable && !d.SrunUsable:
		zone = "宿舍区（Dr.COM）"
	case d.SrunUsable && d.DormUsable:
		zone = "两套都有回应，按宿舍区处理；不对就用 --zone teaching"
	default:
		zone = "两套接口都没回应，判不出来"
	}
	return fmt.Sprintf("协议指纹：深澜握手=%s、ePortal 登录接口=%s → 真掉线时按「%s」的协议登录",
		boolCN(d.SrunUsable), boolCN(d.DormUsable), zone)
}

func boolCN(v bool) string {
	if v {
		return "是"
	}
	return "否"
}

// advices 根据探测结果生成排查建议。
// 这些建议对应的是最常见的几种"连不上"，按出现频率排。
func advices(r *Report) []string {
	var out []string

	if r.Online != nil && r.Online.Online {
		out = append(out, "账号在这个区域已经在线了。如果还是上不了网，"+
			"大概率是代理、域名解析或者系统网络设置的问题，和认证本身无关")
	}
	if r.OnlineErr != nil {
		out = append(out, "查在线状态时出错，多半是网络还没通，可以先忽略这一条")
	}

	switch r.Detect.Zone {
	case portal.ZoneTeaching:
		out = append(out, "你在教学区，走深澜（SRun）认证。账号是 6 位校园卡号，"+
			"密码是统一身份认证密码")
		out = append(out, "教学办公区的上网权限是宿舍区套餐免费附带的，不用另外买")
		out = append(out, "如果报 ldap auth error 是密码错；报 Rad:userid error 是账号错")

	case portal.ZoneDorm:
		out = append(out, "你在宿舍区，走 Dr.COM 网页认证。先去自助服务确认套餐没到期")
		out = append(out, "宿舍区一个账号只能同时登录 1 台电脑 + 2 台移动设备。"+
			"被自己的其他设备挤下线，是\"莫名断网\"最常见的原因")
		out = append(out, "如果提示「尚未办理校内上网套餐」，检查校园卡「通用代扣费账户」余额是否够扣")

	case portal.ZoneOutside:
		out = append(out, "两个认证门户都连不上。先确认是不是在校外；"+
			"如果人在校内，可能是墙上端口或交换机故障，直接报修比反复重试有用")
	}

	if !r.Detect.SrunDNSOK {
		out = append(out, "net.szu.edu.cn 这个域名解析不出来。开着代理或 DoH 时很常见，"+
			"先关掉代理再试，或者用 --ip 直接指定服务器地址")
	}

	return out
}
