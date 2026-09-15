package portal

import "testing"

// TestZoneFingerprint 在真机网络里验证「协议指纹」判区。
//
// 这个测试依赖当前网络，所以在非校园网环境会直接跳过，不算失败。
// 它的价值是：人在学校时跑一下，就能确认判区用的是不是真信号。
func TestZoneFingerprint(t *testing.T) {
	srun := srunUsable()
	drcom := drcomUsable()
	t.Logf("深澜握手(教学区信号)=%v  ePortal 登录接口(宿舍区信号)=%v", srun, drcom)

	if !srun && !drcom {
		t.Skip("两套认证接口都没回应（不在校园网，或都已认证到外网通的状态），跳过")
	}

	switch {
	case srun && !drcom:
		t.Log("判定：教学区（深澜）")
	case drcom && !srun:
		t.Log("判定：宿舍区（Dr.COM）")
	default:
		t.Log("两套都有回应：按宿舍区处理，若登录报协议错误请用 --zone teaching 指定")
	}
}
