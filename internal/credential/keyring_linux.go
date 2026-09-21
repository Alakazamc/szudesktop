//go:build linux

package credential

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// linuxStore 用 Secret Service（gnome-keyring、KWallet 等提供的标准密码存储
// 接口）保存校园网凭据。
//
// 为什么不像 macOS 那样直接调系统 API：Linux 上没有统一的钥匙串实现，
// 走 secret-tool 这个命令行入口是最省事、覆盖最广的办法。
//
// 机器上没有 secret-tool、或者密钥环服务没跑起来时，**不退回明文文件**，
// 一律返回错误（见 store_unavailable.go）。明文密码会被备份、云同步和
// 误提交带走，而用户看到的却是「保存成功」，那比保存失败糟得多。
type linuxStore struct{}

const secretToolMissing = "本机没有 Secret Service，找不到 secret-tool 命令"

func platformStore() Store {
	if _, err := exec.LookPath("secret-tool"); err != nil {
		return newUnavailableStore(secretToolMissing)
	}
	return &linuxStore{}
}

// School sessions require Secret Service. Never fall back to plaintext.
func platformSessionStore() SessionStore {
	if _, err := exec.LookPath("secret-tool"); err != nil {
		return &unavailableSessionStore{}
	}
	return &secretSessionStore{run: runSecretSessionCommand}
}

func (s *linuxStore) Save(c Credentials) error {
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
		// 无头环境里没有 D-Bus 会话时也会走到这里。宁可失败并让用户知道，
		// 也不偷偷写一个明文文件。
		return fmt.Errorf("%w：写入密钥环失败: %v（%s）", ErrStorageUnavailable, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (s *linuxStore) Load() (Credentials, error) {
	out, err := exec.Command("secret-tool", "lookup",
		"service", "szunet",
		"account", "szunet",
	).Output()
	if err != nil {
		// 已知局限：这里分不清「密钥环里没这一条」和「密钥环被锁 / 服务没起来」，
		// 两种都当成没保存过。macOS 那边（R06）已经按退出码区分，Linux 还没有
		// 可靠依据，不猜。
		return Credentials{}, ErrNotFound
	}

	var c Credentials
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(out))), &c); err != nil {
		return Credentials{}, fmt.Errorf("密钥环里的凭据格式不对: %w", err)
	}
	return c, nil
}

func (s *linuxStore) Delete() error {
	out, err := exec.Command("secret-tool", "clear",
		"service", "szunet",
		"account", "szunet",
	).CombinedOutput()
	if err != nil {
		// 删除失败不报成功：用户以为「忘掉账号」了，凭据其实还在。
		return fmt.Errorf("清除密钥环里的凭据失败: %v（%s）", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (s *linuxStore) Describe() string {
	return "Linux Secret Service（gnome-keyring / KWallet）"
}
