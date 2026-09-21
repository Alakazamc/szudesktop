package credential

import (
	"errors"
	"os/exec"
	"strings"
)

// securityItemNotFound 判断 macOS `security` 命令的失败是不是「条目本来就不存在」。
//
// 为什么要单独判：找不到条目是正常的「还没保存过」，而钥匙串被锁、用户拒绝授权、
// security 命令不存在都是真故障。把真故障也说成「没保存」，用户会以为凭据丢了，
// 重新存一遍还是读不出来——和 Linux 上曾经犯过的错一样。
//
// 这个判断放在没有构建标签的文件里，是为了让 Linux CI 也能测到它；
// 只有真正调用钥匙串的那部分限定 darwin。
func securityItemNotFound(err error) bool {
	if err == nil {
		return false
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		// security 找不到条目时退出码是 44（errSecItemNotFound）。
		if exit.ExitCode() == 44 {
			return true
		}
		if keychainSaysMissing(string(exit.Stderr)) {
			return true
		}
	}
	return keychainSaysMissing(err.Error())
}

// keychainSaysMissing 认钥匙串自己报的「没有这一条」，
// 退出码被中间层包装掉时还能靠它兜住。
func keychainSaysMissing(s string) bool {
	s = strings.ToLower(s)
	return strings.Contains(s, "-25300") || strings.Contains(s, "could not be found")
}
