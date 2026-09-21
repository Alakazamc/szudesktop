# 校园网认证一直失败，元凶是一个我从没在意过的参数

先说结论：**`ac_id` 是「你插在哪个网络接口上」的编号，不是随便填 1 就能用的默认值。**

我们的校园网工具箱（深大 szuDesktop）之前一直认证失败，尤其是接了自己路由器的时候。查了一圈，问题就出在这个参数上——代码里写死了 `ac_id=1`，而宿舍区那条线路要的是 `12`。

这篇文章记录一下怎么定位的、为什么它不能写死、以及最后的解决办法。

---

## 一、症状：官方客户端也连不上

最开始的线索是：**接自己路由器的时候，官方那个「上网认证客户端」也起不来**，必须手动打开一个网页才能登录：

```
https://net.szu.edu.cn/srun_portal_success?ac_id=12&theme=proyx
```

注意那个 `ac_id=12`。

我们自己的工具在这个网络下同样连不上。这就排除了「客户端软件写错了」的可能——**官方和我们一起挂，说明问题在协议参数，不在代码质量**。

那时候最自然的怀疑是：这个线路把 `ac_id` 换成了 12，而我们的代码还在用 1。

---

## 二、实测：`ac_id` 确实参与签名，不是装饰

代码里原来有一行注释写着：

> URL 的 ac_id 只是入口编号，不是认证参数。

**这句话是错的。** 实测证据：

| 请求地址 | 门户页面里的 JS 配置 |
|---|---|
| `/srun_portal_pc?ac_id=1&theme=proyx` | `acid : "1"` |
| `/srun_portal_pc?ac_id=12&theme=proyx` | `acid : "12"` |

门户会把你传进去的编号原样回填到页面配置里。而这个 `acid` 会**参与登录时的 chksum 校验和加密串生成**——也就是说，编号传错了，算出来的签名就是错的，服务端直接拒绝。

不是「入口错一点也照样能用」，是**数学上不对**。

---

## 三、踩坑：为什么不能从页面上猜

知道 `ac_id` 会变之后，第一个念头是「那我把所有可能的编号试一遍，哪个能返回登录页就用哪个」。

于是我写了段代码，挨个请求 `ac_id=1/2/3/4/5/10/12`，看哪个能拿到正常页面。

结果很打击人：

```
ac_id=1   → 8333 字节
ac_id=2   → 8333 字节
ac_id=3   → 8334 字节
ac_id=4   → 8333 字节
ac_id=5   → 8333 字节
ac_id=10  → 8333 字节
ac_id=12  → 8333 字节
```

**全都返回完整登录页，体积几乎一样，差 1 个字节。** 页面里翻遍了也找不到任何字段能说明「你现在实际挂在哪个接入点」。

换句话说：**门户页面只能告诉你「这个编号存在」，不能告诉你「你就在这个编号上」。** 试一遍全都能用，等于全是瞎猜。

（这部分代码我留在项目里当兜底了，但明确标了 `guess`，而且**不写缓存**——见后文。）

---

## 四、正解：让网关自己告诉你

那 `ac_id` 到底从哪来？

去过酒店、机场的朋友可能见过「连接 Wi-Fi 后自动弹出登录页」——那个弹窗不是魔法，是网关干的：你请求外网，网关发现你没认证，把你的请求**半路拦下来**，返回一个 302 跳转到认证页。这个跳转地址里就带着它想让你用的参数，**包括 `ac_id`**。

所以正确做法是：**未认证的时候主动去访问一个外网站点，然后读那个 302 跳转里的 `ac_id`。**

```go
func (c *SrunClient) discoverAcIDFromRedirect() string {
	probes := []string{
		connectivityProbe,                                   // 未认证时必定被拦
		"http://www.msftconnecttest.com/redirect",           // Windows 自带探测
		"http://connectivitycheck.gstatic.com/generate_204", // 安卓/Chrome
		"http://captive.apple.com/hotspot-detect.html",      // 苹果
	}

	for _, p := range probes {
		resp, err := c.http.Get(p)
		if err != nil {
			continue
		}
		loc := resp.Header.Get("Location")
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		_ = resp.Body.Close()

		// 只认 3xx 跳转。200 说明没被拦，那就是正常上网，不是校园网闸门。
		if resp.StatusCode/100 != 3 {
			continue
		}

		if id := c.acIDFromLocation(p, loc); id != "" {
			return id
		}
		// 有些网关不返回 Location 头，而是塞一个带 meta refresh /
		// JS 跳转的拦截页。这种也一起捞，否则在那些设备上就彻底瞎了。
		if id := acIDFromInterceptPage(string(body)); id != "" {
			return id
		}
	}
	return ""
}
```

这段有几个点值得说：

- **探针必须是真正的外网站点。** 我一开始把「认证门户自己」也放进了探针列表，结果门户必然 302 到它自己的登录页，而那个地址里带着默认的 `ac_id=1`。于是**已经在线、网关根本没拦**的时候，这里也会「成功」读出一个 1，还被当成可信值。这是个极具欺骗性的假信号，我差点就信了。
- **只认 3xx。** 加了 `resp.StatusCode/100 != 3` 这一条之后，假信号立刻消失。返回 200 说明你根本没被拦，那就是正常上网状态。
- **兼容没有 `Location` 头的网关。** 有些设备返回的是个带 `<meta http-equiv="refresh">` 或 JS `location.href = ...` 的拦截页，也一并解析。

顺带兼容了几种参数别名（`ac_id` / `acid` / `wlanacname` / `nasid`）——不同网关改写 URL 用的名字不完全一样，名字对不上就会漏掉唯一可靠的线索，代价太大。

---

## 五、设计：给每个值标上「可信度」

发现方式有好几种，可靠性差很多。如果都当成一样的值，就会出事。所以我用一个 `AcIDSource` 把来源显式标出来：

```go
const (
	// Manual 用户显式指定。最高优先级，也是唯一"绝对可信"的。
	AcIDSourceManual AcIDSource = "manual"

	// Cache 这张网上次认证成功用过的，跟着成功结果来的。
	AcIDSourceCache AcIDSource = "cache"

	// Redirect 未认证时被校园网网关拦下，从跳转地址里读出来的。
	AcIDSourceRedirect AcIDSource = "redirect"

	// Guess 只从门户页面试探出来的，只能说明"这个编号存在"，
	// 不能说明你就在这个接入点上。不该缓存。
	AcIDSourceGuess AcIDSource = "guess"
)
```

取值优先级从高到低：

```go
func (c *SrunClient) resolveAcIDWithSource() (string, AcIDSource) {
	if c.AcID != "" {
		return c.AcID, AcIDSourceManual        // 1. 用户说了算
	}
	if c.lastAcID != "" {
		return c.lastAcID, AcIDSourceCache     // 2. 这张网上次成功过
	}
	if id := c.discoverAcIDFromRedirect(); id != "" {
		return id, AcIDSourceRedirect          // 3. 网关跳转，权威
	}
	if id := c.discoverAcIDFromPortal(); id != "" {
		return id, AcIDSourceGuess             // 4. 挨个试，猜的
	}
	return "1", AcIDSourceGuess                // 5. 最后兜底，也是猜的
}
```

**关键规则：只有可信来源的成功结果才写缓存。**

```go
if acIDSource != AcIDSourceGuess {
	c.rememberAcID(acID)
}
```

这条看着不起眼，但它防止了一个很恶心的情况：如果猜出来的值也被缓存，那**第一次猜错之后，以后每次都会用这个错值**，而且用户完全不知道为什么——他只会觉得「这软件越来越烂了」。

宁可每次都重新探测，也不要缓存一个没验证过的猜测。

---

## 六、缓存按什么键存

缓存键也得选对。答案是：**默认网关优先，出口 IP 兜底**。

```go
// Gateway 跟着"插哪个口"变，正好和 ac_id 的变化同步。
func Egress() string { return NetKey(Gateway(), LocalIP()) }
```

为什么用网关而不是 MAC 地址或路由器型号？因为**`ac_id` 跟着物理接入位置走，不跟着设备走**：

- 同一台路由器，拔下来插回**原来那个墙口** → `ac_id` 还是 `12`
- 同一台路由器，换个墙口 → 变了
- 换一台路由器，插原墙口 → **还是 `12`**

所以这个缓存是绑在「你现在插在哪个网络口上」的，不是绑在设备上的。网关地址正好和这件事同步变化。

---

## 七、所以「换个路由器会变吗」

回到最初那个问题。答案是：

> **换路由器本身不会变。变的是你插在哪个墙口 / 走哪条线路。**

`ac_id` 是网管给**每个接入点**分的编号。你换路由器，只要还是插原来那个口，学校那边看到的还是同一个接入点，编号当然不变。真正会让它变的是换个墙口、换条线路、或者像这次一样——从直接插墙口改成经过路由器。

至于用户要不要管这件事？**不用。** 现在这套逻辑会在未认证时自己问网关，问不到就用上次成功的，都拿不到才猜一个。用户唯一需要做的，就是在它猜的时候别太当真——所以界面上标了「⚠️ 猜的，不一定对」。

---

## 八、留一个坑给自己

有一套测试专门守着这块逻辑（`internal/portal/acid_test.go`）：

```go
func TestAcIDSourceGuessIsNotCached(t *testing.T)        // 猜出来的不写缓存
func TestAcIDSourceManualIsTrusted(t *testing.T)         // 手填的被信任
func TestRedirectProbeIgnoresNonRedirectResponses(t *testing.T)  // 200 不当跳转
func TestAcIDFromInterceptPage(t *testing.T)             // meta/JS 跳转都认
```

我特意做了负向验证：把 `defaultAcIDCandidates` 改回只试 `{"1"}`，测试确实报红（`ac_id:[1]`）。**测试能抓住这个 bug，才算真的有效**，否则就是一排看着舒服的绿色。

---

## 九、还没做到的

诚实说一句：写这篇文章时，桌面端**还没有让用户手动填 `ac_id` 的界面入口**。后端支持了（命令行有 `--ac-id` 参数，程序内部也有 `Options.AcID`），但界面上没暴露。

> **后续更正（2026-09-21）**：这一条已经做到了。桌面端「校园网」页的登录表单里有「高级设置 · 仅在接入点识别失败时修改」，展开就是「接入点编号」输入框（`desktop/assets/garden/app.mjs:53` 的 `<input id="acid" name="ac_id">`），提交时随 `/api/login` 一起送出（同文件 `:84`）。下面那件没验证成的事仍然没验证。

还有一件事没验证成：本机当前网络已经认证在线，出口 IP 在线时服务端会**在校验 `ac_id` 之前就返回 `ip_already_online_error`**（IP 级短路），所以我在自己机器上拿不到真实的网关跳转，没法端到端复现「未认证 + 需 `ac_id=12`」那一步。这个得等下次真的在路由器未认证状态下测。

---

## 小结

| 问题 | 答案 |
|---|---|
| `ac_id` 是什么 | 接入点编号，参与认证签名 |
| 能写死吗 | 不能，不同线路不一样 |
| 能从门户页面猜到吗 | 不能，所有编号都返回几乎相同的页面 |
| 唯一可靠来源 | 未认证时被网关 302 拦截，读跳转地址 |
| 换路由器会变吗 | 不会。换墙口/换线路才会 |
| 猜出来的值要缓存吗 | **绝对不要** |

题目不算大，但这几个坑挺典型：**一个看起来像「配置常量」的参数，其实是运行时环境信息。** 把它写死，就等于假设所有用户都在同一个接入点上——而这在一个有几百个接入点的校园网里，显然不成立。
