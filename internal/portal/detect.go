package portal

import (
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// 外网连通性探测地址。正常联网时返回 204 No Content；
// 校园网没认证时，请求会被网关抢走，拿回来的不是 204。
const connectivityProbe = "http://connect.rom.miui.com/generate_204"

// DetectResult 是一次区域探测的完整结果。
// 除了结论，探测过程也一起带出来——排查时这些细节比结论更有用。
type DetectResult struct {
	Zone          Zone
	InternetOK    bool // 能不能上外网
	DormPortalOK  bool // 宿舍区门户（172.30.255.42）通不通
	TeachPortalOK bool // 教学区门户（net.szu.edu.cn）通不通
	SrunDNSOK     bool // 教学区门户的域名能不能解析出来
	Notes         []string
}

// Detect 判断设备当前在哪张网。
//
// 判断顺序是实测出来的，不是拍脑袋定的：
//   - 已经联网时，外网探测会直接通过
//   - 没认证的宿舍区，宿舍门户和教学门户**都能**连上，但上不了外网
//   - 没认证的教学区，只有教学门户能连上
//
// 所以先看外网通不通，通了就不用折腾了；不通再看哪个门户能连上。
func Detect() *DetectResult {
	r := &DetectResult{}

	r.SrunDNSOK = dnsResolvable("net.szu.edu.cn")

	r.InternetOK = internetReachable()
	if r.InternetOK {
		r.Zone = ZoneOnline
		r.Notes = append(r.Notes, "能正常访问外网，当前不需要认证")
		return r
	}
	r.Notes = append(r.Notes, "上不了外网，接下来判断你在哪个区")

	r.DormPortalOK = reachable(DefaultDrcomHost + "/")
	r.TeachPortalOK = reachable(DefaultSrunHost + "/")

	switch {
	case r.DormPortalOK && r.TeachPortalOK:
		// 这是宿舍区未认证时最常见的情况，容易误判成教学区，所以放第一个判断。
		r.Zone = ZoneDorm
		r.Notes = append(r.Notes, "两个门户都能连上 → 判定宿舍区（宿舍区未认证时两个门户都通）")
	case r.DormPortalOK:
		r.Zone = ZoneDorm
		r.Notes = append(r.Notes, "只有宿舍门户能连上 → 判定宿舍区")
	case r.TeachPortalOK:
		r.Zone = ZoneTeaching
		r.Notes = append(r.Notes, "只有教学门户能连上 → 判定教学区")
	default:
		r.Zone = ZoneOutside
		r.Notes = append(r.Notes, "两个门户都连不上 → 不在校园网内，或者校园网本身故障")
	}

	if !r.SrunDNSOK {
		r.Notes = append(r.Notes,
			"注意：net.szu.edu.cn 这个域名解析不出来。如果开着代理或 DoH，它可能把域名解析抢走了，"+
				"可以先关掉代理，或者用 --ip 直接指定服务器 IP")
	}

	return r
}

// internetReachable 检查是否真的能上外网。
func internetReachable() bool {
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			Proxy:           nil,
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	resp, err := client.Get(connectivityProbe)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))

	// 只有干净的 204 才算真的通。被网关劫持时状态码通常不是 204。
	return resp.StatusCode == http.StatusNoContent
}

// reachable 只关心"连不连得上"，不管对方返回什么。
//
// 不跟随跳转是故意的：认证门户对未登录的请求一律 302 到登录页，
// 跟随跳转反而会绕远路，甚至被系统代理截胡。
func reachable(rawURL string) bool {
	client := &http.Client{
		Timeout: 4 * time.Second,
		Transport: &http.Transport{
			Proxy:           nil,
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Get(rawURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	return true
}

// dnsResolvable 检查一个域名能不能解析出地址。
func dnsResolvable(host string) bool {
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimPrefix(host, "https://")
	if i := strings.IndexAny(host, "/:"); i >= 0 {
		host = host[:i]
	}
	_, err := net.LookupHost(host)
	return err == nil
}
