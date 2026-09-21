package ui

import (
	"github.com/SzuDesktopTeam/szudesktop/internal/credential"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

type guardTestStore struct {
	value  credential.Credentials
	writes int
}

func (s *guardTestStore) Save(c credential.Credentials) error   { s.value = c; s.writes++; return nil }
func (s *guardTestStore) Load() (credential.Credentials, error) { return s.value, nil }
func (s *guardTestStore) Delete() error                         { s.writes++; return nil }
func (s *guardTestStore) Describe() string                      { return "test memory" }

func TestCredentialRouteProtectsStoredAccount(t *testing.T) {
	cases := []struct {
		name, method, host, origin, site, contentType string
		want                                          int
		writes                                        int
	}{
		{"same origin save", "POST", "127.0.0.1:1234", "http://127.0.0.1:1234", "same-origin", "application/json", 200, 1},
		{"local client save", "POST", "127.0.0.1:1234", "", "", "application/json; charset=utf-8", 200, 1},
		{"cross origin JSON", "POST", "127.0.0.1:1234", "https://example.com", "", "application/json", 403, 0},
		{"cross origin form", "POST", "127.0.0.1:1234", "https://example.com", "cross-site", "text/plain", 403, 0},
		{"opaque origin", "POST", "127.0.0.1:1234", "null", "", "application/json", 403, 0},
		{"another local port", "POST", "127.0.0.1:1234", "http://127.0.0.1:5678", "same-site", "application/json", 403, 0},
		{"DNS rebinding", "POST", "example.com:1234", "http://example.com:1234", "same-origin", "application/json", 403, 0},
		{"plain body no origin", "POST", "127.0.0.1:1234", "", "", "text/plain", 415, 0},
		{"unsupported method", "PUT", "127.0.0.1:1234", "", "", "application/json", 405, 0},
		{"cross origin delete", "DELETE", "127.0.0.1:1234", "https://example.com", "", "", 403, 0},
		{"cross origin read", "GET", "127.0.0.1:1234", "https://example.com", "", "", 403, 0},
		{"local delete", "DELETE", "127.0.0.1:1234", "", "", "", 200, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &guardTestStore{value: credential.Credentials{Username: "original", Password: "original"}}
			s := &Server{store: store}
			mux := http.NewServeMux()
			s.routes(mux, fstest.MapFS{})
			req := httptest.NewRequest(tc.method, "http://"+tc.host+"/api/credential", strings.NewReader(`{"username":"replacement","password":"not-real"}`))
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("Sec-Fetch-Site", tc.site)
			req.Header.Set("Content-Type", tc.contentType)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			if w.Code != tc.want || store.writes != tc.writes {
				t.Fatalf("status=%d writes=%d, want %d/%d", w.Code, store.writes, tc.want, tc.writes)
			}
			if tc.writes == 0 && store.value.Username != "original" {
				t.Fatal("rejected request changed credentials")
			}
		})
	}
}

func TestMutatingAPIsRejectGET(t *testing.T) {
	s := &Server{}
	mux := http.NewServeMux()
	s.routes(mux, fstest.MapFS{})
	for _, path := range []string{"/api/login", "/api/logout", "/api/vpn/connect", "/api/vpn/auth", "/api/vpn/disconnect", "/api/vpn/proxy"} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", "http://127.0.0.1:1234"+path, nil))
		if w.Code != 405 {
			t.Fatalf("%s returned %d", path, w.Code)
		}
	}
}
