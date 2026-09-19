package credential

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsSessionEncryptedRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "session.json")
	s := &windowsSessionStore{path: p}
	if err := s.Save(Session{Cookie: "session-security-test-only"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("session-security-test-only")) || bytes.Contains(data, []byte("cookie")) {
		t.Fatal("plaintext session on disk")
	}
	v, err := s.Load()
	if err != nil || v.Cookie != "session-security-test-only" {
		t.Fatal("encrypted round trip failed")
	}
	if err = s.Delete(); err != nil {
		t.Fatal(err)
	}
}
