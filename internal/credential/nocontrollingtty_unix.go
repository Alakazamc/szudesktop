//go:build unix

package credential

import "syscall"

// noControllingTTY 让子进程脱离控制终端（新会话，没有 /dev/tty）。
//
// 为什么需要：security 的 -w 提示读密码走 readpassphrase(3)，它会优先打开
// /dev/tty；只有在打不开的时候才退回读标准输入。桌面程序本来就没有终端，
// 但命令行版是在终端里跑的——不脱离的话，子进程会去读用户键盘，
// 我们喂进去的管道就没人读，保存会卡在那儿等输入。
func noControllingTTY() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
