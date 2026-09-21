//go:build !unix

package credential

import "syscall"

// noControllingTTY 在 Windows 上没有意义：security 命令只存在于 macOS，
// 这里的实现只是让那几个文件在所有平台都能编译。
func noControllingTTY() *syscall.SysProcAttr {
	return nil
}
