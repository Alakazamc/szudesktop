//go:build darwin

// macOS 上校园网凭据和学校会话都进系统钥匙串，各占一个条目，
// 用户可以只清会话不动密码。
//
// 写入路径在 keychain_prompt.go：密码只从标准输入喂给 security，
// 不放进命令行参数——否则同机其他进程用 ps 就能看到（F21）。
package credential

import (
	"encoding/json"
	"fmt"
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
	return keychainSave(sessionService, sessionAccount, data)
}

func (s *darwinSessionStore) Load() (Session, error) {
	out, err := promptRead(sessionService, sessionAccount)
	if err != nil {
		if securityItemNotFound(err) {
			return Session{}, ErrSessionNotFound
		}
		// 钥匙串被锁、授权被拒、security 命令不存在都是真故障。
		// 说成「没保存过」会让用户以为登录状态丢了，重存一遍还是读不出来。
		return Session{}, fmt.Errorf("%w（%v）", ErrSessionStorageUnavailable, err)
	}
	var v Session
	if err := json.Unmarshal(out, &v); err != nil {
		return Session{}, fmt.Errorf("钥匙串里的登录状态格式不对: %w", err)
	}
	return v, nil
}

func (s *darwinSessionStore) Delete() error {
	_, err := runSecurity([]string{"delete-generic-password",
		"-a", sessionAccount,
		"-s", sessionService,
	}, nil)
	if err != nil {
		if securityItemNotFound(err) {
			return nil // 本来就没有，等于已经删掉
		}
		// 删不掉就不能报成功：用户以为清干净了，会话其实还留在钥匙串里。
		return fmt.Errorf("删除钥匙串里的登录状态失败: %v", err)
	}
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
	return keychainSave(keychainService, keychainAccount, data)
}

func (s *darwinStore) Load() (Credentials, error) {
	out, err := promptRead(keychainService, keychainAccount)
	if err != nil {
		if securityItemNotFound(err) {
			return Credentials{}, ErrNotFound
		}
		// 读不到不等于没保存。界面据此提示「暂时无法读取已保存的账号」，
		// 而不是显示成没存过、让用户重新填一遍密码。
		return Credentials{}, fmt.Errorf("读不到钥匙串里的校园网账号: %v", err)
	}

	var c Credentials
	if err := json.Unmarshal(out, &c); err != nil {
		return Credentials{}, fmt.Errorf("钥匙串里的凭据格式不对: %w", err)
	}
	return c, nil
}

func (s *darwinStore) Delete() error {
	_, err := runSecurity([]string{"delete-generic-password",
		"-a", keychainAccount,
		"-s", keychainService,
	}, nil)
	if err != nil {
		if securityItemNotFound(err) {
			return nil // 本来就没有，等于已经删掉
		}
		// 删不掉就不能报成功，否则用户以为「忘掉账号」生效了，凭据其实还在。
		return fmt.Errorf("删除钥匙串里的校园网账号失败: %v", err)
	}
	return nil
}

func (s *darwinStore) Describe() string {
	return "macOS 钥匙串（Keychain）"
}
