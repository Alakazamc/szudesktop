package ui

import (
	"encoding/json"
	"github.com/SzuDesktopTeam/szudesktop/internal/portal"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOnlineWithoutCampusDoesNotAuthenticate(t *testing.T) {
	s := New(Options{})
	s.probe = func() *portal.DetectResult {
		return &portal.DetectResult{Zone: portal.ZoneOnline, InternetOK: true, Probed: true}
	}
	res := s.doLogin("123456", "not-real", "auto", "")
	if res.OK {
		t.Fatal("Internet connectivity was reported as successful authentication")
	}
	if !strings.Contains(res.Message, "尚未验证") {
		t.Fatalf("missing explanation: %s", res.Message)
	}
}

func TestOnlineAutoLoginAndLogoutUseProtocol(t *testing.T) {
	for _, zone := range []portal.Zone{portal.ZoneTeaching, portal.ZoneDorm} {
		t.Run(string(zone), func(t *testing.T) {
			t.Setenv("SZUNET_CONFIG_DIR", t.TempDir())
			logins, logouts := 0, 0
			fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/cgi-bin/get_challenge":
					w.Write([]byte(`_({"challenge":"0123456789abcdef","client_ip":"10.0.0.8","error":"ok"})`))
				case "/srun_portal_pc":
					w.Write([]byte(`var acid = 12;`))
				case "/cgi-bin/srun_portal":
					if r.URL.Query().Get("action") == "logout" {
						logouts++
					} else {
						logins++
					}
					w.Write([]byte(`_({"error":"ok","suc_msg":"login_ok"})`))
				case "/eportal/portal/login":
					logins++
					w.Write([]byte(`dr1003({"result":1,"msg":"认证成功"})`))
				case "/eportal/portal/logout":
					logouts++
					w.Write([]byte(`dr1003({"result":1,"msg":"注销成功"})`))
				default:
					http.NotFound(w, r)
				}
			}))
			defer fake.Close()
			s := New(Options{SrunHost: fake.URL, DrcomHost: fake.URL})
			s.probe = func() *portal.DetectResult {
				return &portal.DetectResult{Zone: portal.ZoneOnline, InternetOK: true, Probed: true, SrunUsable: zone == portal.ZoneTeaching, DormUsable: zone == portal.ZoneDorm}
			}
			if res := s.doLogin("123456", "not-real", "auto", "12"); !res.OK {
				t.Fatalf("login failed: %+v", res)
			}
			req := httptest.NewRequest("POST", "/api/logout", strings.NewReader(`{"username":"123456","password":"not-real","zone":"auto"}`))
			w := httptest.NewRecorder()
			s.handleLogout(w, req)
			var out loginResp
			if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
				t.Fatal(err)
			}
			if !out.OK || logins != 1 || logouts != 1 {
				t.Fatalf("result=%+v login=%d logout=%d", out, logins, logouts)
			}
		})
	}
}
