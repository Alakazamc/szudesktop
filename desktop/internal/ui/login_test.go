package ui

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDoLoginUsesRequestedTeachingZone(t *testing.T) {
	var sawChallenge, sawLogin bool
	portal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/get_challenge":
			sawChallenge = true
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
	defer portal.Close()

	s := New(Options{SrunHost: portal.URL, Zone: "auto"})
	res := s.doLogin("123456", "not-real", "teaching")
	if !res.OK || !sawChallenge || !sawLogin {
		t.Fatalf("forced teaching login did not complete: result=%+v challenge=%v login=%v", res, sawChallenge, sawLogin)
	}
}

func TestDoLoginUsesRequestedDormZone(t *testing.T) {
	var sawLogin bool
	portal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/eportal/portal/login" {
			http.NotFound(w, r)
			return
		}
		sawLogin = true
		_, _ = w.Write([]byte(`dr1003({"result":1,"msg":"认证成功"})`))
	}))
	defer portal.Close()

	s := New(Options{DrcomHost: portal.URL, Zone: "auto"})
	res := s.doLogin("123456", "not-real", "dorm")
	if !res.OK || !sawLogin {
		t.Fatalf("forced dorm login did not complete: result=%+v login=%v", res, sawLogin)
	}
}
