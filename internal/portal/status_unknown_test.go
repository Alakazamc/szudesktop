package portal

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDrcomStatusRequiresValidState(t *testing.T) {
	for _, tc := range []struct {
		name, body    string
		known, online bool
	}{
		{"online number", `dr1003({"result":1})`, true, true},
		{"online string", `dr1003({"result":"1"})`, true, true},
		{"offline number", `dr1003({"result":0})`, true, false},
		{"offline string", `dr1003({"result":"0"})`, true, false},
		{"malformed JSON", `dr1003({bad})`, false, false},
		{"missing result", `dr1003({"msg":"server error"})`, false, false},
		{"null result", `dr1003({"result":null})`, false, false},
		{"unexpected result", `dr1003({"result":2})`, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/eportal/portal/rad_user_info" {
					t.Errorf("unexpected request: %s", r.URL.Path)
				}
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			st, err := NewDrcomClient(srv.URL, "", "").Status()
			if !tc.known {
				if err == nil || st != nil {
					t.Fatalf("invalid state became offline: status=%+v err=%v", st, err)
				}
				return
			}
			if err != nil || st == nil || st.Online != tc.online {
				t.Fatalf("status=%+v err=%v", st, err)
			}
		})
	}
}

func TestSrunStatusMissingErrorIsUnknown(t *testing.T) {
	for _, body := range []string{`_({})`, `_({"error":null})`, `_({"error":""})`, `_({"error":"  "})`} {
		t.Run(body, func(t *testing.T) {
			srv := fakeSrunPortal(t, "", body)
			st, err := NewSrunClient(srv.URL, "", "").Status()
			if err == nil || st != nil {
				t.Fatalf("missing error became offline: status=%+v err=%v", st, err)
			}
		})
	}
}

func TestQueryOnlineDoesNotHideFailedProtocol(t *testing.T) {
	for _, tc := range []struct {
		name, srun, drcom string
		known, online     bool
	}{
		{"dorm failed", `_({"error":"not_online"})`, `dr1003({})`, false, false},
		{"teaching failed", `_({})`, `dr1003({"result":0})`, false, false},
		{"both failed", `_({})`, `dr1003({})`, false, false},
		{"both offline", `_({"error":"not_online"})`, `dr1003({"result":0})`, true, false},
		{"dorm online after teaching failure", `_({})`, `dr1003({"result":1})`, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/cgi-bin/rad_user_info":
					_, _ = w.Write([]byte(tc.srun))
				case "/eportal/portal/rad_user_info":
					_, _ = w.Write([]byte(tc.drcom))
				default:
					t.Errorf("unexpected request: %s", r.URL.Path)
				}
			}))
			defer srv.Close()
			st, err := QueryOnline(ZoneOnline, srv.URL, srv.URL, "", "")
			if !tc.known {
				if err == nil || st != nil {
					t.Fatalf("failed query became offline: status=%+v err=%v", st, err)
				}
				return
			}
			if err != nil || st == nil || st.Online != tc.online {
				t.Fatalf("status=%+v err=%v", st, err)
			}
		})
	}
}
