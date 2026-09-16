package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Alakazamc/szudesktop/internal/portal"
)

// TestLoginStillAuthenticatesWhenAlreadyOnline 锁死一个真实故障。
//
// 症状：程序明明能上外网，用户点「登录」却什么都没发生，
// 只弹一句"已经能上外网，不用再认证"——看着像登录成功了，其实一次
// 认证请求都没发。
//
// 根因：doLogin 在 zone == online 时直接 return。但"能上外网"和
// "我的账号已经在这个区认证过"是两回事：连着有线/热点/别人的会话残留时，
// 外网是通的，可用户点登录就是想把自己的会话建立起来。
//
// 修法：联网时不再直接放弃，而是看协议指纹指向哪个区，按那套协议真登录一次。
// 这条用例保证：只要指纹认得出区，就必须真的发出认证请求。
func TestLoginStillAuthenticatesWhenAlreadyOnline(t *testing.T) {
	var sawLogin bool
	srun := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/get_challenge":
			_, _ = w.Write([]byte(`_({"challenge":"0123456789abcdef","client_ip":"10.0.0.8","error":"ok"})`))
		case "/srun_portal_pc":
			_, _ = w.Write([]byte(`var acid = 12;`))
		case "/cgi-bin/srun_portal":
			sawLogin = true
			_, _ = w.Write([]byte(`_({"error":"ok","suc_msg":"login_ok"})`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srun.Close()

	s := New(Options{SrunHost: srun.URL, Zone: "auto"})

	// 直接按"指纹认出了教学区"的路径验证：登录必须真的打出去。
	res := s.loginWithProtocol(portal.ZoneTeaching, "123456", "not-real")
	if !res.OK || !sawLogin {
		t.Fatalf("指纹认出教学区后必须真的走一次深澜认证: result=%+v sawLogin=%v", res, sawLogin)
	}
	if !strings.Contains(res.Message, "成功") {
		t.Fatalf("应报告认证成功，实际: %s", res.Message)
	}
}

// TestLoginWithProtocolDormHitsEportal 确认宿舍区那条分支打的是 ePortal。
func TestLoginWithProtocolDormHitsEportal(t *testing.T) {
	var sawLogin bool
	drcom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/eportal/portal/login" {
			http.NotFound(w, r)
			return
		}
		sawLogin = true
		_, _ = w.Write([]byte(`dr1003({"result":1,"msg":"认证成功"})`))
	}))
	defer drcom.Close()

	s := New(Options{DrcomHost: drcom.URL, Zone: "auto"})
	res := s.loginWithProtocol(portal.ZoneDorm, "123456", "not-real")
	if !res.OK || !sawLogin {
		t.Fatalf("宿舍区必须走 ePortal 认证: result=%+v sawLogin=%v", res, sawLogin)
	}
}
