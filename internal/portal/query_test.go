package portal

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestQueryOnlineCoversOnlineZone 锁死这次修的核心：
// 能上外网（ZoneOnline）时也必须去问在线状态。
//
// 以前命令行和桌面界面都在这个分支上直接跳过，用户拿到的只有一句"没查到"，
// 于是以为自己掉线了，跑去反复点登录，然后被 ip_already_online 挡回来。
// 可"能上网"恰恰是已登录的最强证据，这种时候最不该装聋作哑。
func TestQueryOnlineCoversOnlineZone(t *testing.T) {
	srv := fakeSrunPortal(t, "",
		`_({"error":"ok","online_ip":"10.20.30.40","online_device_total":"1"})`)

	st, err := QueryOnline(ZoneOnline, srv.URL, srv.URL, "123456", "pw")
	if err != nil {
		t.Fatal(err)
	}
	if st == nil {
		t.Fatal("联网时必须查到在线状态，不能返回空")
	}
	if !st.Online || st.IP != "10.20.30.40" {
		t.Errorf("结果不对：%+v", st)
	}
}

// TestQueryOnlineZoneOnlineFallsBackToDorm 联网时 Detect() 提前返回、没跑协议
// 指纹，所以不知道当初是哪套协议认证的：深澜那边没结果，要接着问宿舍区那套。
func TestQueryOnlineZoneOnlineFallsBackToDorm(t *testing.T) {
	mux := http.NewServeMux()
	// 深澜：够得着，但当前不在线
	mux.HandleFunc("/cgi-bin/rad_user_info", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`_({"error":"not_online"})`))
	})
	// 宿舍区：在线
	mux.HandleFunc("/eportal/portal/rad_user_info", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`dr1003({"result":1,"online_ip":"10.20.30.41"})`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	st, err := QueryOnline(ZoneOnline, srv.URL, srv.URL, "123456", "pw")
	if err != nil {
		t.Fatal(err)
	}
	if st == nil || !st.Online || st.IP != "10.20.30.41" {
		t.Fatalf("深澜没结果时该采信宿舍区那套，实际 %+v", st)
	}
}

// TestQueryOnlineOutsideAsksNothing 人在校外（或判不出区）就别问了，
// 返回空让调用方跳过显示。地址故意给了个连不上的，真发请求就会失败。
func TestQueryOnlineOutsideAsksNothing(t *testing.T) {
	for _, zone := range []Zone{ZoneOutside, ZoneUnknown} {
		t.Run(string(zone), func(t *testing.T) {
			st, err := QueryOnline(zone, "http://127.0.0.1:1", "http://127.0.0.1:1", "123456", "pw")
			if err != nil || st != nil {
				t.Errorf("%s 不该发起查询，实际 st=%+v err=%v", zone, st, err)
			}
		})
	}
}
