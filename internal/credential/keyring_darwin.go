//go:build darwin

package credential

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// 钥匙串条目的标识。账号信息整体以 JSON 形式存在密码字段里，
// 钥匙串本身是加密存储的，不需要我们再套一层。
const (
	keychainService = "szunet"
	keychainAccount = "szunet"
	// 会话存成独立条目，和账号密码分开，用户可以只清会话不清密码。
	sessionService = "szunet-session"
	sessionAccount = "szunet-session"
)

// darwinStore 用 macOS 钥匙串保存凭据。
// 相比写配置文件，钥匙串受系统统一保护，还能被用户自己的钥匙串访问控制管住。
type darwinStore struct{}

func platformStore() Store { return &darwinStore{} }

// platformSessionStore 的会话同样进钥匙串，单独一个条目。
func platformSessionStore() SessionStore { return &darwinSessionStore{} }

type darwinSessionStore struct{}

func (s *darwinSessionStore) Save(v Session) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	cmd := exec.Command("security", "add-generic-password",
		"-a", sessionAccount,
		"-s", sessionService,
		"-w", string(data),
		"-U",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("写入钥匙串失败: %v（%s）", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (s *darwinSessionStore) Load() (Session, error) {
	cmd := exec.Command("security", "find-generic-password",
		"-a", sessionAccount,
		"-s", sessionService,
		"-w",
	)
	out, err := cmd.Output()
	if err != nil {
		return Session{}, ErrSessionNotFound
	}
	var v Session
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(out))), &v); err != nil {
		return Session{}, fmt.Errorf("钥匙串里的登录状态格式不对: %w", err)
	}
	return v, nil
}

func (s *darwinSessionStore) Delete() error {
	_ = exec.Command("security", "delete-generic-password",
		"-a", sessionAccount,
		"-s", sessionService,
	).Run()
	return nil
}

func (s *darwinSessionStore) Describe() string {
	return "macOS 钥匙串（Keychain）"
}

func (s *darwinStore) Save(c Credentials) error {
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}

	// -U 表示条目已存在就更新，避免重复添加时报错。
	cmd := exec.Command("security", "add-generic-password",
		"-a", keychainAccount,
		"-s", keychainService,
		"-w", string(data),
		"-U",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("写入钥匙串失败: %v（%s）", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (s *darwinStore) Load() (Credentials, error) {
	cmd := exec.Command("security", "find-generic-password",
		"-a", keychainAccount,
		"-s", keychainService,
		"-w",
	)
	out, err := cmd.Output()
	if err != nil {
		return Credentials{}, ErrNotFound
	}

	var c Credentials
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(out))), &c); err != nil {
		return Credentials{}, fmt.Errorf("钥匙串里的凭据格式不对: %w", err)
	}
	return c, nil
}

func (s *darwinStore) Delete() error {
	// 本来就不存在也算删除成功，所以忽略返回值。
	_ = exec.Command("security", "delete-generic-password",
		"-a", keychainAccount,
		"-s", keychainService,
	).Run()
	return nil
}

func (s *darwinStore) Describe() string {
	return "macOS 钥匙串（Keychain）"
}
