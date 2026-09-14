// Package credential 负责保存和读取校园网账号密码。
//
// 设计原则：密码不落明文盘。三端各用系统自带的安全设施：
//
//   - macOS：钥匙串（Keychain）
//   - Windows：DPAPI（用当前用户账户加密，只有本机本用户解得开）
//   - Linux：Secret Service（gnome-keyring 之类），机器上没有就退化成权限受限的文件
//
// 这是刻意和很多同类脚本区分开的地方——把密码明文写进配置文件，
// 一旦配置文件被同步到网盘或者被人翻到，账号就漏了。
package credential

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Credentials 是要保存的账号信息。
type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// ErrNotFound 表示还没保存过凭据。
var ErrNotFound = errors.New("还没有保存过账号密码")

// Store 是凭据存储的统一接口。
type Store interface {
	Save(c Credentials) error
	Load() (Credentials, error)
	Delete() error
	Describe() string // 说明当前用的是哪种存储方式，给用户看的
}

// Default 返回当前平台最合适的凭据存储。
func Default() Store {
	return platformStore()
}

// dir 返回配置目录 ~/.szunet。
func dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".szunet"), nil
}

// dataPath 返回凭据文件在兜底方案下的位置。
func dataPath() (string, error) {
	d, err := dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "credentials.json"), nil
}

// fileStore 是把凭据写在文件里的兜底方案，文件权限设为只有本人可读。
type fileStore struct {
	path string
	desc string
}

func (s *fileStore) Save(c Credentials) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.path, data, 0o600); err != nil {
		return fmt.Errorf("写入凭据文件失败: %w", err)
	}
	return nil
}

func (s *fileStore) Load() (Credentials, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return Credentials{}, ErrNotFound
		}
		return Credentials{}, err
	}
	var c Credentials
	if err := json.Unmarshal(data, &c); err != nil {
		return Credentials{}, fmt.Errorf("凭据文件格式不对: %w", err)
	}
	return c, nil
}

func (s *fileStore) Delete() error {
	err := os.Remove(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (s *fileStore) Describe() string { return s.desc }
