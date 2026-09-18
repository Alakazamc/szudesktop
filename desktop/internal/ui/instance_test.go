package ui

import (
	"os"
	"path/filepath"
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
