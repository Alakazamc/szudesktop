package portal

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestAcIDFromURL 守住一个真踩过的坑。
//
// 背景：深澜认证里的 ac_id 是「你挂在哪个接入点」，跟着墙口/线路走。
// 同一台笔记本直连校园网常见 1，接上路由器换成 12，拿错就报
// "Unknow ac-type"。
//
// 我们的自动发现要靠外部给的 URL 来读这个值，所以这个解析函数
// 必须只认"确实是个数字"的情况 —— 宁可返回空串走兜底，
// 也不要从一个奇怪的地址里抠出一半数字拿去认证。
func TestAcIDFromURL(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "网关跳转地址（最常见的来源）",
			url:  "https://net.szu.edu.cn/srun_portal_pc?ac_id=12&theme=proyx",
			want: "12",
		},
		{
			name: "成功页地址",
			url:  "https://net.szu.edu.cn/srun_portal_success?ac_id=12&theme=proyx",
			want: "12",
		},
		{
			name: "ac_id 排在后面",
			url:  "http://1.1.1.1/?wlanuserip=10.0.0.1&ac_id=3&nas_id=7",
			want: "3",
		},
		{
			name: "没有 ac_id",
			url:  "https://www.msftconnecttest.com/redirect",
			want: "",
		},
		{
			name: "ac_id 不是数字，不能瞎认",
			url:  "https://net.szu.edu.cn/login?ac_id=abc",
			want: "",
		},
		{
			name: "别名 acid",
			url:  "https://portal.example.com/login?acid=12",
			want: "12",
		},
		{
			name: "ac_id 优先于别名",
			url:  "https://portal.example.com/login?acid=99&ac_id=12",
			want: "12",
		},
		{
			name: "空串",
			url:  "",
			want: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := acIDFromURL(c.url); got != c.want {
				t.Fatalf("acIDFromURL(%q) = %q，期望 %q", c.url, got, c.want)
			}
		})
	}
}

// TestAcIDFromInterceptPage 守住"网关不返回 302 而返回拦截页"的情况。
//
// 有些设备（尤其路由器自带的认证跳转）不用 Location 头，而是塞一个
// 带 meta refresh 或 JS 跳转的页面。如果只认 Location 头，
// 在这些设备上就彻底拿不到 ac_id 了 —— 而这是唯一可靠的来源。
func TestAcIDFromInterceptPage(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "meta refresh",
			body: `<html><head><meta http-equiv="refresh" content="0;url=https://net.szu.edu.cn/srun_portal_pc?ac_id=12&theme=proyx"></head></html>`,
			want: "12",
		},
		{
			name: "JS location.href",
			body: `<script>location.href = "https://net.szu.edu.cn/srun_portal_pc?ac_id=7";</script>`,
			want: "7",
		},
		{
			name: "JS location.replace",
			body: `<script>location.replace('https://net.szu.edu.cn/login?ac_id=3')</script>`,
			want: "3",
		},
		{
			name: "普通页面，没有跳转",
			body: `<html><body>欢迎使用校园网</body></html>`,
			want: "",
		},
		{
			name: "空页面",
			body: "",
			want: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := acIDFromInterceptPage(c.body); got != c.want {
				t.Fatalf("acIDFromInterceptPage(...) = %q，期望 %q", got, c.want)
			}
		})
	}
}

// TestAcIDPatternReadsPortalConfig 确认能从门户页内嵌配置里读出 acid。
//
// 实测：同一个页面，传 ac_id=1 页面里就是 acid = "1"，
// 传 ac_id=12 就是 acid = "12"。所以页面里的这个值就是认证要用的值。
func TestAcIDPatternReadsPortalConfig(t *testing.T) {
	cases := []struct {
		name string
		html string
		want string
	}{
		{
			name: "双引号",
			html: `CONFIG = { page: 'account', acid: "12", ip: "172.27.47.91" }`,
			want: "12",
		},
		{
			name: "单引号",
			html: `CONFIG = { acid: '1', nas: "" }`,
			want: "1",
		},
		{
			name: "无引号",
			html: `CONFIG = { acid: 7 }`,
			want: "7",
		},
		{
			name: "等号赋值",
			html: `var acid = "100";`,
			want: "100",
		},
		{
			name: "页面里完全没有",
			html: `<html><body>nothing here</body></html>`,
			want: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := acIDPattern.FindSubmatch([]byte(c.html))
			got := ""
			if m != nil {
				got = string(m[1])
			}
			if c.name == "页面里完全没有" {
				if m != nil {
					t.Fatalf("不该匹配到任何东西，却得到 %q", got)
				}
				return
			}
			if got != c.want {
				t.Fatalf("从 %q 里读出的 acid = %q，期望 %q", c.html, got, c.want)
			}
		})
	}
}

// TestAcIDSourceGuessIsNotCached 守住一个设计决定：猜出来的 ac_id 不许缓存。
//
// 为什么重要：所有门户编号（1/2/3/5/10/12）返回的页面几乎一样大
// （实测 8333 / 8334 字节），页面里也没有任何字段说明"你现在在哪个接入点"。
// 所以从页面这里只能确认"这个编号存在"，**不能确认你就在这个接入点上**。
//
// 如果这种猜出来的值被写进缓存，下次在别的网络里就会被当成定论复用，
// 错得更隐蔽 —— 用户会看到 Unknow ac-type，却不知道为什么"上次还好好的"。
func TestAcIDSourceGuessIsNotCached(t *testing.T) {
	var cached []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/get_challenge":
			_, _ = w.Write([]byte(`_({"challenge":"0123456789abcdef","client_ip":"10.20.30.40","error":"ok"})`))
		case "/srun_portal_pc":
			// 门户页回填我们传进去的编号 —— 但这只证明编号存在。
			_, _ = w.Write([]byte(`CONFIG = { acid: "` + r.URL.Query().Get("ac_id") + `" };`))
		case "/cgi-bin/srun_portal":
			_, _ = w.Write([]byte(`_({"error":"ok","suc_msg":"login_ok"})`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewSrunClient(srv.URL, "123456", "pw")
	c.OnAcIDResolved = func(id string) { cached = append(cached, id) }

	res, err := c.Login()
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("登录应该成功，却是 %+v", res)
	}
	if res.AcIDSource != string(AcIDSourceGuess) {
		t.Fatalf("这个场景应该是猜出来的值，实际来源 %q", res.AcIDSource)
	}
	if len(cached) != 0 {
		t.Fatalf("猜出来的 ac_id 不该被缓存，却写入了 %v", cached)
	}
}

// TestAcIDSourceManualIsTrusted 确认用户手填的值会被当成定论缓存。
//
// 和上一条正好对照：用户说了算，成功之后要记住。
func TestAcIDSourceManualIsTrusted(t *testing.T) {
	var cached []string
	var sawAcID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/get_challenge":
			_, _ = w.Write([]byte(`_({"challenge":"0123456789abcdef","client_ip":"10.20.30.40","error":"ok"})`))
		case "/cgi-bin/srun_portal":
			sawAcID = r.URL.Query().Get("ac_id")
			_, _ = w.Write([]byte(`_({"error":"ok","suc_msg":"login_ok"})`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewSrunClient(srv.URL, "123456", "pw")
	c.AcID = "12"
	c.OnAcIDResolved = func(id string) { cached = append(cached, id) }

	res, err := c.Login()
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("登录应该成功，却是 %+v", res)
	}
	if sawAcID != "12" {
		t.Fatalf("应该用手填的 12 去认证，实际用了 %q", sawAcID)
	}
	if res.AcIDSource != string(AcIDSourceManual) {
		t.Fatalf("来源应该是 manual，实际 %q", res.AcIDSource)
	}
	if len(cached) != 1 || cached[0] != "12" {
		t.Fatalf("手填且成功的值应该被缓存，实际 %v", cached)
	}
}

// TestRedirectProbeIgnoresNonRedirectResponses 守住一个真踩过的坑。
//
// 背景：探针列表最早把「认证门户自己」也算进去了。门户对未登录请求
// 必然 302 到它自己的登录页，而那个地址里带的是默认的 ac_id=1。
//
// 后果很阴：已经在线、网关根本没拦的时候，这里也会"读出"一个 1，
// 还被标记成"网关跳转来的、可信"。用户换到路由器（真正需要 12）后
// 就会拿着这个假可信的 1 去认证，报 Unknow ac-type。
//
// 所以判据必须收紧：只有真的发生 3xx 跳转，才算网关拦了我们。
func TestRedirectProbeIgnoresNonRedirectResponses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 200 + 一个把自己伪装成跳转页的 body。
		// 这种响应说明"没被拦"，绝不能从里面读 ac_id。
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<meta http-equiv="refresh" content="0;url=https://net.szu.edu.cn/login?ac_id=1">`))
	}))
	defer srv.Close()

	c := NewSrunClient(srv.URL, "123456", "pw")
	if got := c.acIDFromLocation(srv.URL+"/x", ""); got != "" {
		t.Fatalf("没有 Location 头时不该读出值，实际 %q", got)
	}

	// 直接验"非 3xx 一律跳过"这条规则。
	if got := c.discoverAcIDFromRedirect(); got != "" {
		t.Fatalf("探针返回 200 时不该认为被网关拦了，却读出 %q", got)
	}
}

// TestRedirectProbeReadsRealGatewayRedirect 确认真被拦时能读出 ac_id。
func TestRedirectProbeReadsRealGatewayRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 模拟校园网网关：任意外网请求都被 302 到认证页，带 ac_id=12。
		w.Header().Set("Location", "https://net.szu.edu.cn/srun_portal_pc?ac_id=12&theme=proyx")
		w.WriteHeader(http.StatusFound)
	}))
	defer srv.Close()

	c := NewSrunClient(srv.URL, "123456", "pw")
	got := c.acIDFromLocation(srv.URL+"/probe",
		"https://net.szu.edu.cn/srun_portal_pc?ac_id=12&theme=proyx")
	if got != "12" {
		t.Fatalf("应该从跳转里读出 12，实际 %q", got)
	}

	// 相对跳转也要能补全后读出来。
	got = c.acIDFromLocation("https://probe.example.com/x", "/login?ac_id=7")
	if got != "7" {
		t.Fatalf("相对跳转应该也能读出 7，实际 %q", got)
	}
}
