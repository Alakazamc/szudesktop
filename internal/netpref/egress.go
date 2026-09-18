//go:build !js

package netpref

import (
	"net"
	"os/exec"
	"runtime"
	"strings"
)

// LocalIP 返回本机在默认路由方向上使用的出口 IP。
// 不实际发包：UDP connect 只是让内核选一条路由，顺手把源地址告诉我们。
func LocalIP() string {
	c, err := net.Dial("udp", "223.5.5.5:80")
	if err != nil {
		return ""
	}
	defer c.Close()
	if a, ok := c.LocalAddr().(*net.UDPAddr); ok {
		return a.IP.String()
	}
	return ""
}

// Gateway 尽力取默认网关地址，取不到返回空串。
//
// 各平台命令不一样，这里只做"能取到就更好"的处理：取不到时
// NetKey 会自动退回到出口 IP，缓存照样能用。
func Gateway() string {
	switch runtime.GOOS {
	case "windows":
		return gatewayWindows()
	case "linux":
		return firstField(runOutput("ip", "route", "show", "default"))
	case "darwin":
		return firstField(runOutput("route", "-n", "get", "default"))
	}
	return ""
}

func gatewayWindows() string {
	out := runOutput("route", "print", "-4")
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		// 形如：0.0.0.0  0.0.0.0  172.27.40.1  172.27.47.91  45
		if len(f) >= 4 && f[0] == "0.0.0.0" && f[1] == "0.0.0.0" {
			if net.ParseIP(f[2]) != nil {
				return f[2]
			}
		}
	}
	return ""
}

// firstField 从命令输出里挑出第一个长得像 IP 的字段。
func firstField(out string) string {
	for _, f := range strings.Fields(out) {
		if ip := net.ParseIP(strings.Trim(f, ",\t")); ip != nil && ip.To4() != nil {
			return ip.String()
		}
	}
	return ""
}

func runOutput(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	hideProcess(cmd)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}

// Egress 返回用于网络缓存键的标识。
//
// 优先网关：它跟着"插哪个口/走哪条线路"变，正好和 ac_id 变化的节奏一致。
// 拿不到网关就退到出口 IP —— 校园网里换网段时出口 IP 一般也会变。
func Egress() string {
	return NetKey(Gateway(), LocalIP())
}
