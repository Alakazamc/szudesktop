package credential

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"time"
)

// Kept platform-independent so failure behavior can be tested on Windows too.
type secretSessionStore struct {
	run func(string, []byte) ([]byte, error)
}

func runSecretSessionCommand(op string, input []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	args := []string{op}
	if op == "store" {
		args = append(args, "--label=szunet 学校系统登录状态")
	}
	args = append(args, "service", "szunet", "kind", "session")
	cmd := exec.CommandContext(ctx, "secret-tool", args...)
	cmd.Stdin = bytes.NewReader(input)
	data, err := cmd.Output()
	// secret-tool uses exit 1 without stderr when no matching item exists.
	if err != nil && (op == "lookup" || op == "clear") {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 && len(bytes.TrimSpace(exitErr.Stderr)) == 0 {
			return nil, nil
		}
	}
	return data, err
}
func (s *secretSessionStore) Save(v Session) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err = s.run("store", data); err != nil {
		return ErrSessionStorageUnavailable
	}
	return nil
}
func (s *secretSessionStore) Load() (Session, error) {
	data, err := s.run("lookup", nil)
	if err != nil {
		return Session{}, ErrSessionStorageUnavailable
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return Session{}, ErrSessionNotFound
	}
	var v Session
	if json.Unmarshal(data, &v) != nil {
		return Session{}, errors.New("安全存储中的登录状态无法解析")
	}
	return v, nil
}
func (s *secretSessionStore) Delete() error {
	if _, err := s.run("clear", nil); err != nil {
		return ErrSessionStorageUnavailable
	}
	return nil
}
func (s *secretSessionStore) Describe() string {
	return "Linux Secret Service（失败时不保存明文）"
}
