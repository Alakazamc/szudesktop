package portal

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeSrunPortal 起一个只应答认证相关四个接口的假门户。
// loginResp 决定登录接口回什么，onlineResp 决定在线查询回什么。
func fakeSrunPortal(t *testing.T, loginResp, onlineResp string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/get_challenge":
			_, _ = w.Write([]byte(`_({"challenge":"0123456789abcdef","client_ip":"10.20.30.40","error":"ok"})`))
		case "/srun_portal_pc":
			_, _ = w.Write([]byte(`window.portal = { acid: "12" };`))
		case "/cgi-bin/srun_portal":
			_, _ = w.Write([]byte(loginResp))
		case "/cgi-bin/rad_user_info":
			_, _ = w.Write([]byte(onlineResp))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestSrunIPAlreadyOnlineIsNotReportedAsSuccess 锁死一个真把人绕晕的坑。
//
// 深澜在两种情况下都给 error:"ok"：
//
//	already_online    —— 自己这个账号在线，会话有效、能上网
//	ip_already_online —— 这个出口已经挂着会话，服务端在**校验账号密码和
//	                     ac_id 之前**就短路返回了
//
// 老实现把后者也当成功，一边返回 OK:true 一边提示「本次登录没有生效」——
// 同一句话里既说成功又说没生效，用户看到只能懵。
func TestSrunIPAlreadyOnlineIsNotReportedAsSuccess(t *testing.T) {
	srv := fakeSrunPortal(t,
		`_({"error":"ok","suc_msg":"ip_already_online_error","online_ip":"10.20.30.40"})`,
		`_({"error":"ok","online_ip":"10.20.30.40","online_device_total":"1",`+
			`"online_device_detail":"{\"344898392\":{\"class_name\":\"Macintosh\",\"os_name\":\"Mac OS\",\"ip\":\"10.20.30.40\"}}"})`,
	)

	c := NewSrunClient(srv.URL, "123456", "pw")
	c.AcID = "12" // 手动来源，这样「该不该写缓存」才测得出来

	var resolved []string
	c.OnAcIDResolved = func(id string) { resolved = append(resolved, id) }

	res, err := c.Login()
	if err != nil {
		t.Fatal(err)
	}

	if res.OK {
		t.Errorf("出口被占用、本次没生效，不该报成功：%+v", res)
	}
	if !strings.Contains(res.Message, "没有生效") {
		t.Errorf("提示要说清本次没生效，实际 %q", res.Message)
	}
	// 用户最想知道的其实是「那我到底能不能上网」，提示得能回答这个
	if !strings.Contains(res.Message, "能上网") {
		t.Errorf("提示该告诉用户怎么自己判断，实际 %q", res.Message)
	}
	// 带上在线设备信息，这是这种情况唯一能给用户的具体线索
	if !strings.Contains(res.Message, "1 台设备在线") || !strings.Contains(res.Message, "Mac OS") {
		t.Errorf("提示该带上在线设备信息，实际 %q", res.Message)
	}
	// 服务端是在校验 ac_id 之前短路的，这个编号压根没被验证过，不能缓存
	if len(resolved) != 0 || c.lastAcID != "" {
		t.Errorf("ac_id 没被验证过就不该写缓存，实际 resolved=%v lastAcID=%q", resolved, c.lastAcID)
	}
}

// TestSrunAlreadyOnlineStaysSuccessAndCachesAcID 是上一条的对照面：
// 账号自己在线，会话有效、能上网，这就该算成功；而且这次认证确实被服务端
// 接受过，ac_id 可以放心缓存。
func TestSrunAlreadyOnlineStaysSuccessAndCachesAcID(t *testing.T) {
	srv := fakeSrunPortal(t,
		`_({"error":"ok","suc_msg":"already_online","online_ip":"10.20.30.40"})`,
		`_({"error":"ok"})`,
	)

	c := NewSrunClient(srv.URL, "123456", "pw")
	c.AcID = "12"

	var resolved []string
	c.OnAcIDResolved = func(id string) { resolved = append(resolved, id) }

	res, err := c.Login()
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Errorf("账号自己在线应该算成功，实际 %+v", res)
	}
	if !strings.Contains(res.Message, "本来就在线") {
		t.Errorf("提示该说清是自己在线，实际 %q", res.Message)
	}
	if len(resolved) != 1 || resolved[0] != "12" {
		t.Errorf("真成功的 ac_id 应该写进缓存，实际 %v", resolved)
	}
}

// TestSrunStatusReadsOnlineDevices 确认在线设备信息能读出来。
// 真实返回里这个字段是「JSON 字符串里再套一层 JSON」，很容易读丢。
func TestSrunStatusReadsOnlineDevices(t *testing.T) {
	srv := fakeSrunPortal(t, "",
		`_({"error":"ok","online_ip":"10.20.30.40","online_device_total":"2",`+
			`"online_device_detail":"{\"344898392\":{\"class_name\":\"Macintosh\",\"os_name\":\"Mac OS\",\"ip\":\"10.20.30.40\"},\"344898393\":{\"class_name\":\"Windows\",\"os_name\":\"Windows 11\",\"ip\":\"10.20.30.41\"}}"})`)

	st, err := NewSrunClient(srv.URL, "123456", "pw").Status()
	if err != nil {
		t.Fatal(err)
	}
	if !st.Online || st.IP != "10.20.30.40" {
		t.Fatalf("在线状态读错了：%+v", st)
	}
	if st.DeviceTotal != 2 {
		t.Errorf("设备数 = %d，应为 2", st.DeviceTotal)
	}
	if len(st.Devices) != 2 {
		t.Fatalf("设备列表 = %v，应有 2 条", st.Devices)
	}
	// 按 rad_online_id 排序，同样的输入必须给出同样的顺序
	want := []string{"Mac OS · 10.20.30.40", "Windows 11 · 10.20.30.41"}
	for i := range want {
		if st.Devices[i] != want[i] {
			t.Errorf("设备描述[%d] = %q，应为 %q", i, st.Devices[i], want[i])
		}
	}
}

// TestSrunOnlineFieldsTolerateGarbage 确认这些补充字段读坏了也不影响主流程。
// 它们只是提示里的装饰，不该让「账号在不在线」这个核心判断一起失败。
func TestSrunOnlineFieldsTolerateGarbage(t *testing.T) {
	for _, detail := range []string{"", "   ", "这不是 json", `["a","b"]`, "{坏"} {
		if got := parseOnlineDevices(detail); got != nil {
			t.Errorf("parseOnlineDevices(%q) = %v，应为 nil", detail, got)
		}
	}

	if got := parseOnlineDeviceCount("不是数字"); got != 0 {
		t.Errorf("坏数字该读成 0，实际 %d", got)
	}
	if got := parseOnlineDeviceCount("-3"); got != 0 {
		t.Errorf("负数该读成 0，实际 %d", got)
	}
}

// TestSrunStatusJudgesOnlineByErrorFieldOnly 记一条实测事实：
// rad_user_info 的返回里**没有 user_name**，所以「在不在线」只能看 error 字段。
// 谁要是图省事改成靠用户名判断在线，这条会红。
func TestSrunStatusJudgesOnlineByErrorFieldOnly(t *testing.T) {
	srv := fakeSrunPortal(t, "",
		`_({"error":"ok","online_ip":"10.20.30.40","online_device_total":"1"})`)

	st, err := NewSrunClient(srv.URL, "123456", "pw").Status()
	if err != nil {
		t.Fatal(err)
	}
	if !st.Online {
		t.Error("error=ok 就该判为在线，不能因为没有 user_name 就说判断不了")
	}
	if st.Username != "" {
		t.Errorf("真实返回里没有 user_name，实际读到 %q", st.Username)
	}
}
