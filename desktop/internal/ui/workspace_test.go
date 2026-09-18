package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func workspaceRequest(s *Server, method, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(method, "http://127.0.0.1/api/workspace", strings.NewReader(body))
	s.handleWorkspace(w, r)
	return w
}
func TestWorkspaceRevisionAndRestart(t *testing.T) {
	t.Setenv("SZUNET_CONFIG_DIR", t.TempDir())
	s := New(Options{})
	w := workspaceRequest(s, "GET", "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	body := `{"version":1,"revision":0,"data":{"marker":"garden"}}`
	w = workspaceRequest(s, "POST", body)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	second := New(Options{})
	w = workspaceRequest(second, "GET", "")
	var got workspaceSnapshot
	json.Unmarshal(w.Body.Bytes(), &got)
	if got.Revision != 1 || !strings.Contains(string(got.Data), "garden") {
		t.Fatal(w.Body.String())
	}
	w = workspaceRequest(second, "POST", body)
	if w.Code != http.StatusConflict {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, bad := range []string{`{}`, `{"version":1,"revision":1,"data":null}`, `{"version":1,"revision":1,"data":{}} {}`} {
		if w = workspaceRequest(s, "POST", bad); w.Code != 400 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}
func TestWorkspaceIndependentServersCannotOverwrite(t *testing.T) {
	t.Setenv("SZUNET_CONFIG_DIR", t.TempDir())
	servers := []*Server{New(Options{}), New(Options{})}
	codes := make(chan int, 2)
	var wg sync.WaitGroup
	for _, s := range servers {
		wg.Add(1)
		go func(s *Server) {
			defer wg.Done()
			codes <- workspaceRequest(s, "POST", `{"version":1,"revision":0,"data":{"garden":true}}`).Code
		}(s)
	}
	wg.Wait()
	close(codes)
	ok, conflict := 0, 0
	for code := range codes {
		if code == 200 {
			ok++
		}
		if code == 409 {
			conflict++
		}
	}
	if ok != 1 || conflict != 1 {
		t.Fatalf("ok %d conflict %d", ok, conflict)
	}
}
func TestWorkspaceCorruptionIsPreserved(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SZUNET_CONFIG_DIR", dir)
	p := filepath.Join(dir, "workspace-v1.json")
	os.WriteFile(p, []byte("broken"), 0600)
	w := workspaceRequest(New(Options{}), "POST", `{"version":1,"revision":0,"data":{}}`)
	if w.Code != 500 {
		t.Fatal(w.Code)
	}
	b, _ := os.ReadFile(p)
	if string(b) != "broken" {
		t.Fatal("corruption overwritten")
	}
}
func TestCredentialPrivacyByDefault(t *testing.T) {
	for _, url := range []string{"http://127.0.0.1/api/credential", "http://127.0.0.1/api/credential?reveal=0"} {
		if revealedUsername(httptest.NewRequest("GET", url, nil), "123456") != "" {
			t.Fatal("account revealed")
		}
	}
	if revealedUsername(httptest.NewRequest("GET", "http://127.0.0.1/api/credential?reveal=1", nil), "123456") != "123456" {
		t.Fatal("explicit reveal failed")
	}
}
func TestPartialCredentialsDoNotMix(t *testing.T) {
	s := New(Options{})
	if s.doLogin("123456", "", "auto", "").OK {
		t.Fatal("partial credentials accepted")
	}
	w := httptest.NewRecorder()
	s.handleLogout(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"username":"123456"}`)))
	if !strings.Contains(w.Body.String(), "同时填写") {
		t.Fatal(w.Body.String())
	}
}
