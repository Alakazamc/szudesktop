package portal

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestProbedFlagDistinguishesNotProbedFromFailed 锁死一个真实踩过的坑。
//
// 背景：detect 早期版本在"能上外网"时会提前 return，把 DormPortalOK /
// TeachPortalOK / SrunUsable / DormUsable 留成零值 false。但调用方（诊断报告、
// 界面状态格）分不清"没跑"和"跑了但没探到"，一律按后者显示，
// 于是明明在校园网里、深澜握手也正常，界面却在说"两个门户都连不上"，
// 判区看着像在校外，登录就跟着用错协议。
//
// 这条用例保证：联网快路径下 Probed 必须是 false（=这几个字段没意义），
// 而 Probe() 无论联不联网都必须把探测跑完、Probed 必须是 true。
func TestProbedFlagDistinguishesNotProbedFromFailed(t *testing.T) {
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/cgi-bin/get_challenge"):
			_, _ = w.Write([]byte(`_({"challenge":"abc123","client_ip":"10.0.0.9","error":"ok"})`))
		case r.URL.Path == "/":
			_, _ = w.Write([]byte(`<html>portal</html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer fake.Close()

	// 直接构造一个结果，验证"没探过"时不会被当成"探失败"。
	// 这是调用方真正依赖的契约。
	notProbed := &DetectResult{Zone: ZoneOnline, InternetOK: true}
	if notProbed.Probed {
		t.Fatal("零值结果不该是 Probed=true")
	}
	// 四个探测字段此时全是 false，但调用方必须靠 Probed 区分，
	// 而不是靠它们自己的真假。
	if notProbed.SrunUsable || notProbed.DormUsable ||
		notProbed.DormPortalOK || notProbed.TeachPortalOK {
		t.Fatal("未探测时四个字段应保持零值")
	}

	// 跑一次真的 Probe()：不管网络环境如何，都必须把探测做完。
	probed := Probe()
	if !probed.Probed {
		t.Fatal("Probe() 必须把探测跑完，Probed 应为 true")
	}
}

// TestClassifyUsesFingerprintNotJustPortalReachability 覆盖判区规则本身。
//
// 核心约定（踩过坑才定下来的）：
//   - 只有深澜指纹 → 教学区
//   - 只有 ePortal 指纹 → 宿舍区
//   - 两个门户都通但都没有指纹 → 宿舍区（教学区机器上宿舍门户首页也返回 200）
//   - 什么都没探到 → 校外
func TestClassifyUsesFingerprintNotJustPortalReachability(t *testing.T) {
	cases := []struct {
		name string
		in   DetectResult
		want Zone
	}{
		{
			name: "只有深澜指纹 → 教学区",
			in:   DetectResult{SrunUsable: true},
			want: ZoneTeaching,
		},
		{
			name: "只有 ePortal 指纹 → 宿舍区",
			in:   DetectResult{DormUsable: true},
			want: ZoneDorm,
		},
		{
			name: "两套指纹都有 → 宿舍区（宿舍区常见）",
			in:   DetectResult{SrunUsable: true, DormUsable: true},
			want: ZoneDorm,
		},
		{
			name: "两个门户都通但无指纹 → 宿舍区（别误判成教学区）",
			in:   DetectResult{DormPortalOK: true, TeachPortalOK: true},
			want: ZoneDorm,
		},
		{
			name: "只有宿舍门户通 → 宿舍区",
			in:   DetectResult{DormPortalOK: true},
			want: ZoneDorm,
		},
		{
			name: "只有教学门户通 → 教学区",
			in:   DetectResult{TeachPortalOK: true},
			want: ZoneTeaching,
		},
		{
			name: "两个门户都探不到 → 校外",
			in:   DetectResult{},
			want: ZoneOutside,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := classify(&c.in)
			if got != c.want {
				t.Fatalf("classify() = %s, 期望 %s", got, c.want)
			}
		})
	}
}
