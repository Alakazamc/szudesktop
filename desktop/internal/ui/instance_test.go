package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestInstanceLockAndStaleRecord(t *testing.T) {
	dir := t.TempDir()
	first, existing, err := acquireInstance(dir, false)
	if err != nil || existing {
		t.Fatalf("first instance: %v %v", existing, err)
	}
	release, owned, err := tryInstanceLock(filepath.Join(dir, "desktop-instance.lock"))
	if err != nil || owned {
		if release != nil {
			release()
		}
		t.Fatalf("second acquired lock: %v %v", owned, err)
	}
	first.close()
	if err := os.WriteFile(filepath.Join(dir, "desktop-instance.json"), []byte("stale"), 0600); err != nil {
		t.Fatal(err)
	}
	next, existing, err := acquireInstance(dir, false)
	if err != nil || existing {
		t.Fatalf("stale discovery blocks restart: %v %v", existing, err)
	}
	next.close()
}
func TestInstanceRejectsUntrustedEndpoints(t *testing.T) {
	for _, address := range []string{"https://127.0.0.1:80", "http://example.com:80", "http://127.0.0.1:80/path", "http://user@127.0.0.1:80", "http://127.0.0.1:80?q=x", "http://127.0.0.1:80#fragment"} {
		if activateInstance(instanceRecord{URL: address, Token: string(make([]byte, 64))}, false) == nil {
			t.Fatal("accepted", address)
		}
	}
}

func TestInstanceReuseReturnsVerifiedEndpoint(t *testing.T) {
	dir := t.TempDir()
	first, existing, err := acquireInstance(dir, false)
	if err != nil || existing {
		t.Fatalf("first instance: %v %v", existing, err)
	}
	defer first.close()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Token string `json:"token"`
			Open  bool   `json:"open"`
		}
		if r.URL.Path != "/api/instance" || r.Method != http.MethodPost || json.NewDecoder(r.Body).Decode(&request) != nil || request.Token != first.Token || request.Open {
			http.Error(w, "unexpected activation", http.StatusForbidden)
			return
		}
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	if err := first.publish(server.URL); err != nil {
		t.Fatal(err)
	}
	reused, existing, err := acquireInstance(dir, false)
	if err != nil || !existing {
		t.Fatalf("reuse: %v %v", existing, err)
	}
	if reused == nil || reused.URL != server.URL {
		t.Fatal("verified reused endpoint was not returned")
	}
	if reused.release != nil {
		t.Fatal("reused endpoint must not own the first instance lock")
	}
	if calls.Load() != 1 {
		t.Fatalf("activation calls: %d", calls.Load())
	}
}
