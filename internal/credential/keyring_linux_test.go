//go:build linux

package credential

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// CI 的 ubuntu runner 通常没有可用的 Secret Service，正好用来验证真实的
// platformStore()：没有密钥环就拒绝保存，并且不往配置目录写任何东西。
// 本机 Windows 编译不到这个文件，由 Linux CI 执行。
func TestLinuxStoreRefusesPlaintextWithoutSecretService(t *testing.T) {
	if _, err := exec.LookPath("secret-tool"); err == nil {
		t.Skip("这台机器有 secret-tool，走不到拒绝保存那条路")
	}
	dir := t.TempDir()
	t.Setenv("SZUNET_CONFIG_DIR", dir)

	s := platformStore()
	err := s.Save(Credentials{Username: "2026123456", Password: "hunter2"})
	if !errors.Is(err, ErrStorageUnavailable) {
		t.Fatalf("没有密钥环时必须拒绝保存，得到 %v", err)
	}

	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("密码被写进了配置目录：%v", names)
	}

	if !strings.Contains(s.Describe(), "不可用") {
		t.Fatalf("Describe 会显示给用户，必须如实说明存储不可用，得到 %q", s.Describe())
	}
	if _, loadErr := s.Load(); !errors.Is(loadErr, ErrStorageUnavailable) {
		t.Fatalf("读不出来的原因必须如实说明，得到 %v", loadErr)
	}
}
