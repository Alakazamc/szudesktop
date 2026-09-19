//go:build linux

package credential

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// linuxStore 优先使用 Secret Service（gnome-keyring、KWallet 等提供的
// 标准密码存储接口），机器上没有这套服务时退化成权限受限的文件。
//
// 为什么不像 macOS 那样直接调系统 API：Linux 上没有统一的钥匙串实现，
// 走 secret-tool 这个命令行入口是最省事、覆盖最广的办法。
type linuxStore struct {
	fallback  *fileStore
	useSecret bool
}

func platformStore() Store {
	path, err := dataPath()
	if err != nil {
		return &fileStore{path: "credentials.json", desc: "文件（无法确定用户目录）"}
	}

	s := &linuxStore{
		fallback: &fileStore{
			path: path,
			desc: "文件（~/.szunet/credentials.json，权限 600）",
		},
	}
	if _, err := exec.LookPath("secret-tool"); err == nil {
		s.useSecret = true
	}
	return s
}

// platformSessionStore 的会话优先走 Secret Service，没有就落权限受限的文件。
// 和凭据分开存，用户可以只清会话、保留校园网密码。
func platformSessionStore() SessionStore {
	path, err := sessionPath()
	if err != nil {
		return &fileSessionStore{path: "session.json", desc: "文件（无法确定用户目录）"}
	}
	s := &linuxSessionStore{
		fallback: &fileSessionStore{
			path: path,
			desc: "文件（~/.szunet/session.json，权限 600）",
		},
	}
	if _, err := exec.LookPath("secret-tool"); err == nil {
		s.useSecret = true
	}
	return s
}

type linuxSessionStore struct {
	fallback  *fileSessionStore
	useSecret bool
}

func (s *linuxSessionStore) Save(v Session) error {
	if !s.useSecret {
		return s.fallback.Save(v)
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	cmd := exec.Command("secret-tool", "store",
		"--label=szunet 学校系统登录状态",
		"service", "szunet",
		"kind", "session",
	)
	cmd.Stdin = strings.NewReader(string(data))
	if out, err := cmd.CombinedOutput(); err != nil {
		if ferr := s.fallback.Save(v); ferr == nil {
			s.useSecret = false
			return nil
		}
		return fmt.Errorf("写入密钥环失败: %v（%s）", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (s *linuxSessionStore) Load() (Session, error) {
	if !s.useSecret {
		return s.fallback.Load()
	}
	out, err := exec.Command("secret-tool", "lookup",
		"service", "szunet",
		"kind", "session",
	).Output()
	if err != nil {
		if v, ferr := s.fallback.Load(); ferr == nil {
			return v, nil
		}
		return Session{}, ErrSessionNotFound
	}
	var v Session
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(out))), &v); err != nil {
		return Session{}, fmt.Errorf("密钥环里的登录状态格式不对: %w", err)
	}
	return v, nil
}

func (s *linuxSessionStore) Delete() error {
	_ = exec.Command("secret-tool", "clear",
		"service", "szunet",
		"kind", "session",
	).Run()
	return s.fallback.Delete()
}

func (s *linuxSessionStore) Describe() string {
	if s.useSecret {
		return "Linux Secret Service（gnome-keyring / KWallet）"
	}
	return s.fallback.Describe()
}

func (s *linuxStore) Save(c Credentials) error {
	if !s.useSecret {
		return s.fallback.Save(c)
	}

	data, err := json.Marshal(c)
	if err != nil {
		return err
	}

	// 密码从标准输入读，不出现在命令行参数里，避免被 ps 看到。
	cmd := exec.Command("secret-tool", "store",
		"--label=szunet 校园网凭据",
		"service", "szunet",
		"account", "szunet",
	)
	cmd.Stdin = strings.NewReader(string(data))

	if out, err := cmd.CombinedOutput(); err != nil {
		// 钥匙串服务没跑起来时不要直接失败，退回文件方式，保证工具还能用。
		if ferr := s.fallback.Save(c); ferr == nil {
			s.useSecret = false
			return nil
		}
		return fmt.Errorf("写入密钥环失败: %v（%s）", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (s *linuxStore) Load() (Credentials, error) {
	if !s.useSecret {
		return s.fallback.Load()
	}

	out, err := exec.Command("secret-tool", "lookup",
		"service", "szunet",
		"account", "szunet",
	).Output()
	if err != nil {
		// 密钥环里没有，再试试文件兜底。
		if c, ferr := s.fallback.Load(); ferr == nil {
			return c, nil
		}
		return Credentials{}, ErrNotFound
	}

	var c Credentials
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(out))), &c); err != nil {
		return Credentials{}, fmt.Errorf("密钥环里的凭据格式不对: %w", err)
	}
	return c, nil
}

func (s *linuxStore) Delete() error {
	_ = exec.Command("secret-tool", "clear",
		"service", "szunet",
		"account", "szunet",
	).Run()
	return s.fallback.Delete()
}

func (s *linuxStore) Describe() string {
	if s.useSecret {
		return "Linux Secret Service（gnome-keyring / KWallet）"
	}
	return s.fallback.Describe()
}
