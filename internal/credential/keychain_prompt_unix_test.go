//go:build unix

package credential

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 这个测试用真的子进程跑一遍完整链路：把 securityBin 指向一个假脚本，
// 验证密码确实经标准输入送达、命令行参数里确实没有它，并且读回校验也在跑。
//
// macOS 上 security 的提示输入行为（readpassphrase 打不开 /dev/tty 时退回读标准输入）
// 无法在这里验证，只能靠这条链路 + 真机自检，见 docs/STATUS.md。
func TestKeychainSaveThroughRealCommandKeepsSecretOutOfArgv(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "keychain")
	argvLog := filepath.Join(dir, "argv.log")
	script := filepath.Join(dir, "security")

	body := `#!/bin/sh
printf '%s\n' "$*" >> ` + argvLog + `
case "$1" in
  add-generic-password)
    # 真实 security 会问两遍：密码，然后是确认。两行不一致就报
    # "passwords don't match"、退出码 44，条目根本建不起来。
    IFS= read -r first || true
    IFS= read -r second || true
    if [ -z "$first" ] || [ "$first" != "$second" ]; then
      echo "passwords don't match" >&2
      exit 44
    fi
    printf '%s' "$first" > ` + store + `
    ;;
  find-generic-password)
    if [ -f ` + store + ` ]; then cat ` + store + `; echo; else echo "could not be found (-25300)" >&2; exit 44; fi
    ;;
  delete-generic-password)
    rm -f ` + store + `
    ;;
esac
exit 0
`
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil {
		t.Fatalf("写假命令失败: %v", err)
	}

	savedBin := securityBin
	securityBin = script
	promptMu.Lock()
	promptVerified = false
	promptMu.Unlock()
	t.Cleanup(func() {
		securityBin = savedBin
		promptMu.Lock()
		promptVerified = false
		promptMu.Unlock()
	})

	secret := []byte(`{"username":"000000","password":"test-only-secret"}`)
	if err := keychainSave("szunet-unix-test", "szunet-unix-test", secret); err != nil {
		t.Fatalf("保存失败: %v", err)
	}

	logs, err := os.ReadFile(argvLog)
	if err != nil {
		t.Fatalf("假命令没有被调用: %v", err)
	}
	if strings.Contains(string(logs), "test-only-secret") {
		t.Fatalf("密码出现在子进程命令行参数里，同机 ps 可见:\n%s", logs)
	}
	if !strings.Contains(string(logs), "find-generic-password") {
		t.Fatal("没有读回校验，写成功不代表写对了")
	}

	got, err := os.ReadFile(store)
	if err != nil {
		t.Fatalf("假钥匙串里没有内容: %v", err)
	}
	if string(got) != string(secret) {
		t.Fatalf("标准输入的内容没被完整收到，存下来的是 %q", got)
	}
}
