// Package credential 负责保存和读取校园网账号密码。
//
// 设计原则：密码不落明文盘。三端各用系统自带的安全设施：
//
//   - macOS：钥匙串（Keychain）
//   - Windows：DPAPI（用当前用户账户加密，只有本机本用户解得开）
//   - Linux：Secret Service（gnome-keyring 之类）
//
// 任何一端拿不到系统安全设施时都**明确报错**（ErrStorageUnavailable），
// 不退回明文文件——见 store_unavailable.go。
//
// 这是刻意和很多同类脚本区分开的地方——把密码明文写进配置文件，
// 一旦配置文件被同步到网盘或者被人翻到，账号就漏了。
package credential

import (
	"errors"
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
	// 测试/便携场景可显式隔离配置目录，避免冒烟测试覆盖真实账号。
	// 正常双击客户端时不设置这个变量，仍使用 ~/.szunet。
	if d := os.Getenv("SZUNET_CONFIG_DIR"); d != "" {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".szunet"), nil
}

// dataPath 返回凭据文件的位置（Windows 用它存 DPAPI 加密后的内容）。
func dataPath() (string, error) {
	d, err := dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "credentials.json"), nil
}
