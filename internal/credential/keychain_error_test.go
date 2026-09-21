package credential

import (
	"errors"
	"os/exec"
	"runtime"
	"strconv"
	"testing"
)

// exitWith 跑一个真的以指定码退出的进程，用来造出真实的 *exec.ExitError。
// 不手工拼 ExitError：它的 ProcessState 为 nil 时 ExitCode() 会 panic，
// 那种测试测的是假对象，不是真实行为。
func exitWith(t *testing.T, code int, stderr string) error {
	t.Helper()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// cmd 里换行不是命令分隔符，必须用 &；文本避开括号，否则 cmd 会当成代码块解析。
		script := "exit " + strconv.Itoa(code)
		if stderr != "" {
			script = "echo " + stderr + " 1>&2 & " + script
		}
		cmd = exec.Command("cmd", "/c", script)
	} else {
		script := "exit " + strconv.Itoa(code)
		if stderr != "" {
			script = "echo '" + stderr + "' >&2; " + script
		}
		cmd = exec.Command("sh", "-c", script)
	}
	// 用 Output() 而不是 Run()：只有 Output() 会把 stderr 收进 ExitError.Stderr，
	// 生产代码里的 runSecurity 走的也是 Output()。
	_, err := cmd.Output()
	if err == nil {
		t.Fatalf("命令本该以 %d 退出，却成功了", code)
	}
	return err
}

func TestSecurityItemNotFound(t *testing.T) {
	t.Run("missing item exits 44", func(t *testing.T) {
		if !securityItemNotFound(exitWith(t, 44, "")) {
			t.Fatal("退出码 44 是「条目不存在」，必须认出来")
		}
	})

	t.Run("keychain message in stderr counts", func(t *testing.T) {
		err := exitWith(t, 1, "SecKeychainSearchCopyNext: The specified item could not be found in the keychain.")
		if !securityItemNotFound(err) {
			t.Fatalf("stderr 里已经写明条目不存在，却没认出来: %v", err)
		}
	})

	t.Run("wrapped error text still counts", func(t *testing.T) {
		if !securityItemNotFound(errors.New("security: exit status 1: errSecItemNotFound (-25300)")) {
			t.Fatal("退出码被包装掉时应靠 -25300 认出来")
		}
	})

	t.Run("other failures are not silently treated as missing", func(t *testing.T) {
		for name, err := range map[string]error{
			"nil":                nil,
			"exit 1":             exitWith(t, 1, ""),
			"locked keychain":    errors.New("User interaction is not allowed"),
			"security not found": exec.ErrNotFound,
		} {
			if securityItemNotFound(err) {
				t.Fatalf("%s 被当成了「没保存过」，用户会以为凭据丢了", name)
			}
		}
	})
}
