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
)

// darwinStore 用 macOS 钥匙串保存凭据。
// 相比写配置文件，钥匙串受系统统一保护，还能被用户自己的钥匙串访问控制管住。
type darwinStore struct{}

func platformStore() Store { return &darwinStore{} }

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
