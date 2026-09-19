package credential

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Session 是用户从浏览器里交过来的登录会话（ehall 的 Cookie）。
//
// 为什么不自己走一遍统一身份认证拿会话？
// 因为那要复现账号密码登录、短信/企业微信二次验证、各种加密和风控，代价高且脆弱。
// 让用户在自己浏览器里正常登录，再把会话交给本机复用，是更稳也更透明的做法：
// 用户随时能在浏览器里退出登录让它失效，不需要把统一身份认证密码交给这个程序。
//
// 和 Credentials 一样落在系统安全存储里（Windows 用 DPAPI），
// 不写明文、不进源码、不进发布包、不进日志。
type Session struct {
	// Cookie 是原样粘过来的 Cookie 头，请求时直接带上。
	Cookie string `json:"cookie"`
	// Note 是给人看的来源说明，例如「ehall 统一身份认证」。
	Note string `json:"note,omitempty"`
}

// ErrSessionNotFound 表示还没保存过会话。
var ErrSessionNotFound = errors.New("还没有保存学校系统登录状态")

// SessionStore 是会话存储的统一接口。
type SessionStore interface {
	Save(s Session) error
	Load() (Session, error)
	Delete() error
	Describe() string
}

// DefaultSession 返回当前平台最合适的会话存储。
func DefaultSession() SessionStore {
	return platformSessionStore()
}

func sessionPath() (string, error) {
	d, err := dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "session.json"), nil
}

// fileSessionStore 是会话的文件兜底方案，权限只有本人可读。
type fileSessionStore struct {
	path string
	desc string
}

func (s *fileSessionStore) Save(v Session) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return errors.New("创建配置目录失败: " + err.Error())
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.path, data, 0o600); err != nil {
		return errors.New("写入登录状态失败: " + err.Error())
	}
	return nil
}

func (s *fileSessionStore) Load() (Session, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return Session{}, ErrSessionNotFound
		}
		return Session{}, err
	}
	var v Session
	if err := json.Unmarshal(data, &v); err != nil {
		return Session{}, errors.New("登录状态内容格式不对")
	}
	return v, nil
}

func (s *fileSessionStore) Delete() error {
	err := os.Remove(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (s *fileSessionStore) Describe() string { return s.desc }
