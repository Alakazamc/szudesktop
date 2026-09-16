package portal

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSrunLoginEndToEndAgainstPortal(t *testing.T) {
	var sawLogin bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/get_challenge":
			if r.URL.Query().Get("username") != "123456" {
				t.Errorf("challenge username = %q", r.URL.Query().Get("username"))
			}
			_, _ = w.Write([]byte(`_({"challenge":"0123456789abcdef","client_ip":"10.20.30.40","error":"ok"})`))
		case "/srun_portal_pc":
			_, _ = w.Write([]byte(`window.portal = { acid: "12" };`))
		case "/cgi-bin/srun_portal":
			q := r.URL.Query()
			sawLogin = true
			if q.Get("action") != "login" || q.Get("username") != "123456" {
				t.Errorf("wrong login identity/action: %v", q)
			}
			if !strings.HasPrefix(q.Get("password"), "{MD5}") || len(q.Get("password")) != len("{MD5}")+32 {
				t.Errorf("password was not HMAC-MD5 encoded: %q", q.Get("password"))
			}
			if !strings.HasPrefix(q.Get("info"), "{SRBX1}") {
				t.Errorf("info was not SRBX1 encoded: %q", q.Get("info"))
			}
			if q.Get("ac_id") != "12" || q.Get("ip") != "10.20.30.40" || len(q.Get("chksum")) != 40 {
				t.Errorf("wrong SRun parameters: %v", q)
			}
			_, _ = w.Write([]byte(`_({"error":"ok","suc_msg":"login_ok","online_ip":"10.20.30.40"})`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	res, err := NewSrunClient(srv.URL, "123456", `p&<>"ass`).Login()
	if err != nil {
		t.Fatal(err)
	}
	if !sawLogin || !res.OK || res.Message != "认证成功" {
		t.Fatalf("unexpected result: saw=%v result=%+v", sawLogin, res)
	}
}

func TestDrcomLoginEndToEndAgainstPortal(t *testing.T) {
	var sawLogin bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/eportal/portal/login" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		sawLogin = true
		if q.Get("user_account") != ",0,123456" || q.Get("user_password") != `p&<>"ass` {
			t.Errorf("credentials were not encoded correctly: %v", q)
		}
		if q.Get("login_method") != "1" || q.Get("jsVersion") != "4.1.3" {
			t.Errorf("wrong Dr.COM parameters: %v", q)
		}
		_, _ = w.Write([]byte(`dr1003({"result":1,"msg":"认证成功","online_ip":"10.20.30.40"})`))
	}))
	defer srv.Close()

	res, err := NewDrcomClient(srv.URL, "123456", `p&<>"ass`).Login()
	if err != nil {
		t.Fatal(err)
	}
	if !sawLogin || !res.OK || res.Message != "认证成功" {
		t.Fatalf("unexpected result: saw=%v result=%+v", sawLogin, res)
	}
}
