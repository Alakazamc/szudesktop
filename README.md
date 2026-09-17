# szuDesktop

> 深圳大学校园网客户端 —— 像素风桌面应用 + 命令行，一套 Go 内核，三端编译成单个可执行文件。

<p>
  <img alt="platform" src="https://img.shields.io/badge/platform-Windows%20%7C%20macOS%20%7C%20Linux-4a6fa5?style=flat-square">
  <img alt="go" src="https://img.shields.io/badge/Go-1.26%2B-00ADD8?style=flat-square&logo=go&logoColor=white">
  <img alt="license" src="https://img.shields.io/badge/license-MIT-2f7d32?style=flat-square">
  <img alt="release" src="https://img.shields.io/github/v/release/Alakazamc/szudesktop?include_prereleases&style=flat-square&label=release&color=c9a227">
  <img alt="runtime" src="https://img.shields.io/badge/runtime-%E6%97%A0%E4%BE%9D%E8%B5%96-6b7280?style=flat-square">
</p>

自动识别你在**教学区**还是**宿舍区**，走对应那套认证协议；连不上时告诉你卡在哪一步。

![szuDesktop 界面](docs/screenshot-desktop.png)

---

## 目录

- [它解决什么问题](#它解决什么问题)
- [下载与使用](#下载与使用)
- [功能状态](#功能状态)
- [命令行 szunet](#命令行-szunet)
- [设计取舍](#设计取舍)
- [账号密码存在哪](#账号密码存在哪)
- [自己构建](#自己构建)
- [项目结构](#项目结构)
- [已知限制](#已知限制)
- [参与贡献](#参与贡献)
- [免责声明](#免责声明)
- [致谢](#致谢)
- [许可](#许可)

---

## 它解决什么问题

深大校园网分成两个**互不通用**的区域，认证方式完全是两套东西：

| 区域 | 系统 | 协议特点 |
|---|---|---|
| 教学区 / 办公区 / 图书馆 | 深澜（SRun） | 2025 年寒假上线，密码过 HMAC-MD5，用户信息过 XXTEA 加密 + 自定义 Base64，另需 SHA1 校验和 |
| 宿舍区 / 教工区 | Dr.COM（ePortal） | 一个 GET 请求即可完成 |

由此带来两个麻烦：

1. **2024 年以前的教程和脚本在教学区全部失效** —— 它们都是为老的 Dr.COM 写的。
2. **报错信息五花八门没人解释** —— `ldap auth error`、`Rad:userid error1`、`登陆失败[05]`、
   `Unknow ac-type`…… 用户只能瞎试。

szuDesktop 同时对付这两件事：**一个按钮完成认证，一个按钮完成诊断。**

判区不看「门户能不能连上」，而看**协议指纹**（深澜看 `get_challenge` 能否握手、
宿舍区看 ePortal 登录接口是否存在）。原因是实测发现宿舍门户在教学区机器上首页同样返回 200，
只测连通性会在掉线时误判成宿舍区，然后拿 Dr.COM 协议去打一个 404 的接口。

---

## 下载与使用

### 下载

到 [Releases](../../releases) 页面下载 `szudesktop-<版本>-windows-amd64.zip`，
解压到任意位置，双击 `szudesktop.exe`。**绿色版，不用安装。**

### 首次使用

1. 打开程序（弹出无边框窗口，任务栏图标是荔枝）
2. 顶上点「**校园网**」标签，进登录页
3. 填校园卡号和统一身份认证密码；想免输就勾「记住账号密码」
4. 点「登录」

凭据存进 Windows 保险箱（DPAPI 加密），**不明文落盘**，换机器或换用户都解不开。
随时可以点「忘掉账号」。

> **连不上就先点「断线诊断」。** 它会列出区域判定、门户可达性、协议指纹和对应结论，
> 比一句「登录失败」有用得多。配套排查思路见
> [`docs/深圳大学校园网连不上排查指南.md`](docs/深圳大学校园网连不上排查指南.md)。

### 接到自己的路由器上？

**上一版（beta0.3）修好了「接路由器后登录不上」的问题。**

校园网每个接入点有独立编号（`ac_id`），你插哪个口就得报哪个号。
旧版本固定用 `1`，而路由器那条线路要求 `12`，编号报错直接被服务端拒绝。

现在按下面的顺序找编号，从上往下、哪个先成用哪个：

| 优先级 | 来源 | 可信度 |
|---|---|---|
| 1 | 你自己指定的（`--ac-id 12`） | 最高 |
| 2 | 这台机器在**这个网口**上上次认证成功用过的 | 高 |
| 3 | 未认证时被网络拦下，从它给的跳转地址里读出来的 | 权威 |
| 4 | 挨个试常见编号 | 低（界面上标「⚠️ 猜的」） |

**编号跟墙上那个网口走，不跟路由器走**：

- 同一台路由器插回**原来的口** → 编号不变
- 同一台路由器换个口 → 变了
- **换一台路由器**插原口 → 还是原来的编号

出现「认证失败：ac_id 用错了」时，用 `szunet detect` 看当前识别出的接入点，
必要时 `--ac-id` 手动指定。原理解析见
[`docs/blog/ac_id-接入点编号.md`](docs/blog/ac_id-接入点编号.md)。

### 界面说明

**本版（beta0.4）把概览页从翻页改成了二级菜单，登录失败涉及接入点编号时会自动展开高级选项。**

- **一屏放下，不滚动**：整页固定在一个窗口里，内容多的页面顶部有二级菜单
  （比如概览页的总览 / 服务 / 农田 / 统计 / 动态 / 荔宝），点哪块看哪块
- 认证报「接入点编号」相关错误时，登录页会自动展开高级选项让你手动填 `ac_id`；
  平时默认收起，普通用户不用管这个词
- 宠物、农田、收成架是游戏化外壳（星露谷那一路），不影响功能
- **不做后台自动重连**：要不要登录由你在登录页决定，程序不会在背后周期性发认证请求
- 关掉窗口不等于退出：程序还在后台。要彻底退，任务管理器结束 `szudesktop.exe`

---

## 功能状态

**当前版本：`beta0.4`** —— 公开测试版，校园网认证主链路可用，其余逐步接入。

| 功能 | 状态 |
|---|---|
| 教学区登录（深澜 SRun） | ✅ 已实现 |
| 宿舍区登录（Dr.COM ePortal） | ✅ 已实现，待真机复测 |
| **接入点编号（`ac_id`）自动发现 + 按网口缓存** | ✅ 已实现 |
| 自动判区（协议指纹）+ 手动指定 | ✅ 已实现 |
| 注销下线 / 断线诊断 / 账号存系统保险箱 | ✅ 已实现 |
| 校外 VPN（内置深信服 EasyConnect 协议） | ✅ 页面已接入，待校外网络实机验证 |
| 图书馆 / 一卡通 / 教务 / 服务等页面 | 🚧 开发中（目前是占位页） |
| 余额、流量、在线时长等真实数据 | 🚧 开发中（界面上标「演示」的数字是写死的示例） |
| macOS / Linux 客户端 | 🚧 开发中（先做 Windows） |

---

## 命令行 szunet

同一个内核还带一个命令行工具，适合脚本和没有图形界面的机器。

```bash
szunet detect                              # 看你在哪个区、网络通不通
szunet config set -u 2023xxxx -p 你的密码  # 存一次账号（进系统保险箱）
szunet login                               # 登录
szunet diag                                # 连不上时跑这个
```

### 命令

| 命令 | 作用 |
|---|---|
| `szunet login` / `logout` | 登录 / 注销 |
| `szunet status` | 看当前在哪个区、账号在不在线 |
| `szunet detect` | 只探测网络区域与接入点，不做认证 |
| `szunet diag` | 连不上时跑这个，给出排查结论 |
| `szunet config set/show/delete` | 管理保存的账号密码 |
| `szunet version` | 看版本 |

### 参数

| 参数 | 说明 |
|---|---|
| `-u, --user` / `-p, --password` | 校园卡号 / 统一身份认证密码 |
| `--zone` | 强制指定区域：`auto`（默认）/ `teaching` / `dorm` |
| `--ip` | 直接指定认证服务器 IP，绕过域名解析 |
| `--ac-id` | 指定深澜的 `ac_id`（教学区，一般不用手动给） |
| `--host-teaching` | 教学区深澜门户（默认 `https://net.szu.edu.cn`） |
| `--host-dorm` | 宿舍区 Dr.COM 门户（默认 `http://172.30.255.42`） |
| `--json` | 以 JSON 输出，方便脚本调用 |
| `--verbose` | 打印服务端原始返回，排错用 |

账号密码优先级：**命令行参数 > 环境变量**（`SZUNET_USERNAME` / `SZUNET_PASSWORD`）**> 已保存的凭据**。

### 当库用

`internal/portal` 里的两套协议客户端可以直接 import：

```go
c := portal.NewSrunClient(portal.DefaultSrunHost, user, pass)
res, err := c.Login()
```

---

## 设计取舍

社区已经有十来个深大校园网工具了，这个项目的取舍是：

| 取舍 | 理由 |
|---|---|
| **带图形界面的单文件** | 同类工具大多只有命令行，或要装 Python / Node 运行时。这个把界面编译进 exe，在本机回环地址起个小服务驱动内核，双击就能跑 |
| **三端原生单文件** | Go 写成，加密逻辑是纯 Go 实现，交叉编译出来直接跑 |
| **凭据进系统保险箱** | macOS 钥匙串 / Windows DPAPI / Linux Secret Service。很多同类工具把密码明文写在配置文件里 |
| **诊断优先** | 不是简单报「登录失败」，而是把区域判定、门户可达性、协议指纹、账号在线状态列出来并给出结论 |
| **认证请求强制直连** | 显式禁用系统代理，避免请求被 Clash / Mihomo 这类工具抓走 —— 这是校园网认证失败的高频原因，但很少有工具处理它 |
| **不猜、不缓存猜测** | `ac_id` 只把可信来源的成功结果写盘。猜出来的值绝不缓存，否则第一次猜错会被永久固化 |

---

## 账号密码存在哪

**不落明文盘。** 各平台用各平台自己的安全设施：

| 平台 | 存放方式 |
|---|---|
| Windows | DPAPI 加密后写文件，密钥由当前用户账户派生 |
| macOS | 钥匙串（Keychain），条目 `service=szunet` |
| Linux | Secret Service（gnome-keyring / KWallet）；机器上没有则退化成 `~/.szunet/credentials.json`，权限 600 |

---

## 自己构建

需要 **Go 1.26+**，以及 **Python 3**（仅用于桌面端构建脚本）。

```bash
git clone https://github.com/Alakazamc/szudesktop.git
cd szudesktop

# 命令行
go build -o szunet ./cmd/szunet

# 桌面端：同步页面 → 编译 → 塞图标和版本信息 → 校验
python desktop/build-windows.py

# 冒烟测试（起真服务、逐个打接口、检查页面引用的资源有没有 404）
python desktop/smoke_windows.py

# 打发布包
python desktop/make_release.py
```

界面的源文件是 `desktop/index.html`，**只改这一个**；
构建脚本会自动同步到 `desktop/assets/`（浏览器直接打开用）和
`desktop/internal/ui/assets/`（`go:embed` 能看见的那份）。

### 脚本里已经踩平的坑

- `go:embed` 不能引用上级目录，所以资源要复制到包内（`sync` 那一步）
- 同一个页面在 `file://` 和 `http://` 下对相对路径的解析基准不同，
  服务端要剥掉 `/assets` 前缀，否则本地打开正常、跑起来满屏破图
- Go 不支持在 Windows exe 里加资源，`desktop/add_resource.py` 直接改 PE，
  塞图标和版本信息，不引入 `windres` 之类的构建依赖

---

## 项目结构

```
cmd/szunet/           命令行入口
desktop/              桌面客户端
  index.html            界面源文件（唯一要改的那份）
  build-windows.py      构建：同步页面 → 编译 → 注入图标版本 → 校验
  smoke_windows.py      冒烟测试（40 项）
  add_resource.py       纯 Python 改 PE 加图标/版本信息
  internal/ui/          本地服务 + 界面接口（/api/*）
internal/crypto/      深澜协议用到的加密原语（HMAC-MD5、XXTEA 变体、自定义 Base64、SHA1）
internal/portal/      两套认证协议的客户端 + 区域探测 + ac_id 发现（srun.go / drcom.go）
internal/netpref/     按网络标识（网关 + 出口 IP）缓存 ac_id
internal/vpn/         深信服 EasyConnect 协议客户端（校外 VPN，开发中）
internal/credential/  凭据存储，按平台分文件
internal/diagnose/    诊断逻辑
design/               设计与学习笔记（点阵生成器、布局探针、VPN 协议笔记）
docs/                 排查指南 + 博客
```

---

## 已知限制

- **改完记得重新打开程序**：界面是编译进 exe 的，旧窗口不会自动更新。
- `--ip` 用于绕过域名解析时，HTTPS 证书校验仍按原域名进行，不会降级成跳过校验。
- 宿舍区 Dr.COM 的接口在不同楼栋、不同版本的 ePortal 上可能有细微差异。
  遇到解析不出来的返回，加 `--verbose` 看原始内容再提 issue。
- **接入点编号自动发现依赖网关拦截**：只有在「未认证」状态下才能问出编号。
  已经在线时拿不到，此时会退回缓存值或猜一个，属正常行为。
- 接口一旦被学校改动，认证就会失效 —— 协议细节集中在
  `internal/portal/srun.go` 和 `internal/portal/drcom.go`，改动只涉及这两个文件。
- 界面上的余额、流量、在线时长是演示数据（学校没有公开接口）。

---

## 参与贡献

欢迎 issue 和 PR。提交前请确保：

```bash
go build ./... && go vet ./... && gofmt -l .
go test ./...
python desktop/build-windows.py && python desktop/smoke_windows.py
```

- **改界面**：只改 `desktop/index.html`，别手改 `desktop/assets/` 和
  `desktop/internal/ui/assets/`（脚本会覆盖）
- **报认证问题**：请附 `szunet diag --verbose` 的输出（**注意先删掉账号密码**）
- **提 PR**：一个 PR 解决一件事，说明复现步骤和验证方式

---

## 免责声明

- 本项目是**第三方作品，与深圳大学无关**，学校不对它负责。
- **本项目与深信服科技股份有限公司无任何关联、合作或授权关系**，
  也不包含该公司的任何源代码。EasyConnect 及相关名称的一切权利归深信服所有。
- **仅供个人学习与研究使用**，不得用于商业用途。请勿转售、代充或据此牟利。
- 请使用**自己的账号**，不要共享、转借账号。VPN 功能仅限连接**本人所属学校**的资源。
- **不要和官方客户端同时使用**，两者互斥，会互相踢下线。
- 请遵守学校的校园网管理规定。因使用本工具产生的任何后果由使用者自行承担。
- 学校或厂商的系统升级都可能导致功能失效，本项目不作任何可用性保证。

---

## 致谢

深澜协议的加密实现参考了这些项目（均为 MIT 许可）：

- [Sleepstars/SZU-login](https://github.com/Sleepstars/SZU-login) — XXTEA（xEncode）实现与整体协议流程
- [vidar-team/srun-login](https://github.com/vidar-team/srun-login) — 上游实现

另外参考了这些项目的设计思路：

- [zu1k/srun](https://github.com/zu1k/srun) — 多网卡绑定、自动探测 IP
- [AatroxChen77/szu-net](https://github.com/AatroxChen77/szu-net) — 双区域策略引擎、断线保活思路
- [Sleepstars/SZU_Utils](https://github.com/Sleepstars/SZU_Utils) — 双区识别脚本

校外 VPN 的深信服协议部分为**依据公开协议行为分析独立实现**——协议流程由观察公开的
客户端与服务端交互行为整理而来，代码自行编写，**未复制任何厂商或第三方的源代码**。
协议流程整理与改造记录见 `design/vpn-notes.md`。

本项目与深信服科技股份有限公司**不存在任何关联、合作或授权关系**；
EasyConnect 及相关名称的一切权利归深信服所有。

界面素材与字体版权归原作者所有，本地原型用途。

---

## 许可

[MIT](LICENSE)。第三方代码声明见 LICENSE 文件末尾。
