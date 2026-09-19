package credential

import (
	"errors"
	"os"
	"testing"
)

func TestSessionStorageFailureNeverWritesPlaintext(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, s := range []SessionStore{&unavailableSessionStore{}, &secretSessionStore{run: func(string, []byte) ([]byte, error) { return []byte("sensitive-output"), errors.New("backend failed") }}} {
		if !errors.Is(s.Save(Session{Cookie: "test-only-secret"}), ErrSessionStorageUnavailable) {
			t.Fatal("save must fail closed")
		}
		if _, err := s.Load(); !errors.Is(err, ErrSessionStorageUnavailable) {
			t.Fatal("load error must stay visible")
		}
		if err := s.Delete(); !errors.Is(err, ErrSessionStorageUnavailable) {
			t.Fatal("delete error must stay visible")
		}
	}
	entries, err := os.ReadDir(".")
	if err != nil || len(entries) != 0 {
		t.Fatal("failure wrote a file")
	}
}
func TestSecretSessionUsesStdinAndIndependentKey(t *testing.T) {
	var stored []byte
	s := &secretSessionStore{run: func(op string, in []byte) ([]byte, error) {
		switch op {
		case "store":
			stored = append([]byte{}, in...)
		case "lookup":
			return stored, nil
		case "clear":
			stored = nil
		default:
			t.Fatal(op)
		}
		return nil, nil
	}}
	if err := s.Save(Session{Cookie: "test-only"}); err != nil {
		t.Fatal(err)
	}
	v, err := s.Load()
	if err != nil || v.Cookie != "test-only" {
		t.Fatal("round trip")
	}
	if err = s.Delete(); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Load(); !errors.Is(err, ErrSessionNotFound) {
		t.Fatal("empty lookup")
	}
}
