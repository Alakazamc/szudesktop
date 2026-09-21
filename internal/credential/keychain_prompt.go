package credential

// macOS 的 security 命令如果把密码写在命令行参数里，同一台机器上的其他进程
// 用 ps 就能看到它（F21）。这里统一改成把密码从标准输入喂进去，
// 命令行参数里绝不出现机密内容。
//
// Apple 的 man page 对这种写法的说明是：-w 放在命令最后时不带值，
// 程序会提示输入（"Put at end of command to be prompted (recommended)"）。
// 提示走的是 readpassphrase(3)，它在打不开 /dev/tty 时会退回读标准输入——
// 所以子进程要脱离控制终端（见 noControllingTTY），管道才喂得进去。
//
// 这段逻辑放在没有构建标签的文件里，是为了让 Linux CI 能用假命令验证
// 「密码不进 argv、只走标准输入」这条性质；真正操作钥匙串的只有 darwin 的文件。

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// securityBin 是 security 命令的位置，测试可以替换成假命令。
var securityBin = "security"

// runSecurity 执行 security 子命令。
//
// args 里不允许出现机密：调用方只能把机密放进 stdin。
// stdin 为 nil 表示这条命令不需要输入。
var runSecurity = func(args []string, stdin []byte) ([]byte, error) {
	cmd := exec.Command(securityBin, args...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	cmd.SysProcAttr = noControllingTTY()
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		// %w 不能省：securityItemNotFound 要顺着错误链找退出码 44（条目不存在）；
		// stderr 拼进消息里，退出码被包装掉时还能靠 -25300 认出来。
		return nil, fmt.Errorf("security %s 失败: %w%s", args[0], err, briefStderr(errBuf.String()))
	}
	return out.Bytes(), nil
}

// briefStderr 把命令报错压成一行短句，避免把整段用法说明塞进界面。
func briefStderr(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if len(s) > 200 {
		s = s[:200]
	}
	return "（" + s + "）"
}

// secretInput 把机密拼成「一行 + 换行」，喂给 security 的提示输入。
//
// 必须复制一份：直接 append 到调用方的切片上，容量够时会改掉它身后的数组。
func secretInput(secret []byte) []byte {
	buf := make([]byte, 0, len(secret)+1)
	buf = append(buf, secret...)
	buf = append(buf, '\n')
	return buf
}

func promptWrite(service, account string, secret []byte) error {
	// -w 必须放最后且不带值：这样 security 才会来要输入，而不是把它当参数收下。
	_, err := runSecurity([]string{"add-generic-password", "-a", account, "-s", service, "-U", "-w"}, secretInput(secret))
	return err
}

func promptRead(service, account string) ([]byte, error) {
	out, err := runSecurity([]string{"find-generic-password", "-a", account, "-s", service, "-w"}, nil)
	if err != nil {
		return nil, err
	}
	return bytes.TrimRight(out, "\r\n"), nil
}

// 自检结果只缓存成功：失败不缓存，用户解锁钥匙串后重试还能成功。
var (
	promptMu       sync.Mutex
	promptVerified bool
)

// ensurePromptWriteWorks 先用一次性条目确认这条路真的能用，再写真实凭据。
//
// 为什么不直接写真实条目试：万一密码没喂进去，钥匙串里留下的就是空值或错值，
// 用户下次登录会莫名其妙失败，而且原来存着的密码已经被覆盖掉了。
// 探测条目独立命名，失败也不会碰到用户的凭据。
func ensurePromptWriteWorks() error {
	promptMu.Lock()
	defer promptMu.Unlock()
	if promptVerified {
		return nil
	}

	stamp := strconv.FormatInt(time.Now().UnixNano(), 36)
	service, account := "szunet-selftest-"+stamp, "szunet-selftest"
	probe := []byte("probe-" + stamp)

	if err := promptWrite(service, account, probe); err != nil {
		return fmt.Errorf("这台机器上「密码经标准输入写入钥匙串」不可用，已取消保存: %w", err)
	}
	defer func() {
		_, _ = runSecurity([]string{"delete-generic-password", "-a", account, "-s", service}, nil)
	}()

	got, err := promptRead(service, account)
	if err != nil {
		return fmt.Errorf("自检条目写进去却读不回来，已取消保存: %w", err)
	}
	if !bytes.Equal(got, probe) {
		return errors.New("自检时写入的值与读回的不一致，已取消保存")
	}
	promptVerified = true
	return nil
}

// keychainSave 是 macOS 上凭据与会话共用的写入路径。
//
// 顺序是有意的：先自检机制、再写、最后读回校验。任何一步失败都如实报错，
// 不会退回「把密码放到命令行参数」那种会被 ps 看到的老做法。
func keychainSave(service, account string, secret []byte) error {
	if err := ensurePromptWriteWorks(); err != nil {
		return err
	}
	if err := promptWrite(service, account, secret); err != nil {
		return err
	}
	got, err := promptRead(service, account)
	if err != nil {
		return fmt.Errorf("保存后读回校验失败（写入可能没有生效）: %w", err)
	}
	if !bytes.Equal(got, secret) {
		return errors.New("保存后读回的内容与写入不一致，请重新保存一次；仍然失败请反馈")
	}
	return nil
}
