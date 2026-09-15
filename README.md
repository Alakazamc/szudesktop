# szuNet

深圳大学校园网工具箱：命令行 + 桌面客户端 + 校外 VPN。

自动判断你在**教学区**还是**宿舍区**，走对应的那套认证协议；连不上的时候，
告诉你卡在哪一步；人在校外时，用内置的深信服协议客户端连回学校 VPN。

Windows / macOS / Linux 三端通用，编译产物是**单个可执行文件**，
不依赖 Python、Node 之类的运行时——下载下来就能跑。

---

## 它解决什么问题

深大的校园网分成两个互不通用的区域，认证方式完全是两套东西：

| 区域 | 系统 | 协议特点 |
|---|---|---|
| 教学区 / 办公区 / 图书馆 | 深澜（SRun） | 2025 年寒假上线，密码要过 HMAC-MD5、用户信息要过 XXTEA 加密和自定义 Base64，另外还要算 SHA1 校验和 |
| 宿舍区 / 教工区 | Dr.COM（ePortal） | 一个 GET 请求就完事 |

由此带来两个麻烦：

1. **2024 年以前的教程和脚本在教学区全部失效**，因为它们都是为老的 Dr.COM 写的。
2. 连不上的时候，报错信息五花八门（`ldap auth error`、`Rad:userid error1`、
   `登陆失败[05]`……），但没人告诉你这些分别意味着什么。

szuNet 同时对付这两件事：一个命令完成认证，一个命令完成诊断。

## 和同类工具有什么不一样

社区已经有十来个深大校园网工具了，这个项目的取舍是：

- **三端原生单文件**。不少同类工具只做 Linux（面向路由器或服务器），
  有的用 Python 写、要求目标机器装好 Python 和 Node.js 才能跑加密逻辑。
  szuNet 用 Go 写成，加密逻辑是纯 Go 实现，交叉编译出来直接跑。
- **凭据进系统保险箱**。密码不落明文盘：macOS 用钥匙串，Windows 用 DPAPI
  （换机器、换用户都解不开），Linux 用 Secret Service。很多同类工具把密码
  明文写在配置文件里。
- **诊断优先**。`szunet diag` 不是简单地报"登录失败"，而是把区域判定、
  门户可达性、域名解析、账号在线状态列出来，并给出对应的结论。
  配套的排查思路见 [`docs/深圳大学校园网连不上排查指南.md`](docs/深圳大学校园网连不上排查指南.md)。
- **认证请求强制直连**。显式禁用系统代理，避免请求被 Clash / Mihomo 这类工具
  抓走——这是校园网认证失败的一个高频原因，但很少有工具处理它。

## 快速开始

先看看你在哪个区、网络通不通：

```bash
szunet detect
```

把账号存起来（推荐，只存一次）：

```bash
szunet config set -u 2023xxxx -p 你的密码
```

登录：

```bash
szunet login
```

连不上？跑诊断：

```bash
szunet diag
```

## 安装

### 直接下载

到 [Releases](../../releases) 页面，按你的系统下载对应的文件：

| 系统 | 文件 |
|---|---|
| Windows | `szunet-windows-amd64.exe` |
| macOS（Apple 芯片） | `szunet-darwin-arm64` |
| macOS（Intel） | `szunet-darwin-amd64` |
| Linux | `szunet-linux-amd64` |

下载后给它执行权限即可，不需要安装。

```bash
chmod +x szunet-darwin-arm64
./szunet-darwin-arm64 detect
```

### 自己编译

需要 Go 1.21 或更高版本。

```bash
git clone https://github.com/Alakazamc/szuNet.git
cd szuNet
go build -o szunet ./cmd/szunet
```

一次性编译出三端产物：

```bash
make cross
```

## 命令

| 命令 | 作用 |
|---|---|
| `szunet login` | 登录。自动判断区域，走对应协议 |
| `szunet logout` | 注销当前会话 |
| `szunet status` | 看当前在哪个区、账号在不在线 |
| `szunet detect` | 只探测网络区域，不做认证 |
| `szunet diag` | 连不上时跑这个，给出排查结论 |
| `szunet config set/show/delete` | 管理保存的账号密码 |
| `szunet version` | 看版本 |

## 参数

| 参数 | 说明 |
|---|---|
| `-u, --user` | 校园卡号（6 位） |
| `-p, --password` | 统一身份认证密码 |
| `--zone` | 强制指定区域：`auto`（默认）/ `teaching` / `dorm` |
| `--ip` | 直接指定认证服务器 IP，绕过域名解析 |
| `--ac-id` | 指定深澜的 `ac_id`（教学区，一般不用手动给） |
| `--host-teaching` | 教学区深澜门户地址（默认 `https://net.szu.edu.cn`） |
| `--host-dorm` | 宿舍区 Dr.COM 门户地址（默认 `http://172.30.255.42`） |
| `--json` | 以 JSON 输出，方便脚本调用 |
| `--verbose` | 打印服务端原始返回，排错用 |

账号密码的优先级：命令行参数 > 环境变量（`SZUNET_USERNAME` / `SZUNET_PASSWORD`）
> 已保存的凭据。

### 可以当库用

`internal/portal` 里的两套协议客户端也可以直接 import：

```go
c := portal.NewSrunClient(portal.DefaultSrunHost, user, pass)
res, err := c.Login()
```

## 账号密码存在哪

**不落明文盘。** 各平台用各平台自己的安全设施：

| 平台 | 存放方式 |
|---|---|
| macOS | 钥匙串（Keychain），条目 `service=szunet` |
| Windows | DPAPI 加密后写文件，密钥由当前用户账户派生 |
| Linux | Secret Service（gnome-keyring / KWallet）；机器上没有则退化成 `~/.szunet/credentials.json`，权限 600 |

也可以完全不用保存功能，改用环境变量或每次从命令行传入。

删掉保存的凭据：

```bash
szunet config delete
```

## 桌面客户端 szuDesktop（Windows 首版）

同仓库里还有一个图形界面的客户端，给不想敲命令行的人用。

它做的事：把网页界面**编译进一个 exe**，在本机回环地址起个小服务让界面上的按钮
能真的驱动校园网认证。所以它不碰窗口系统、不装运行时，双击就能跑，三端共用一份代码。

| | |
|---|---|
| 下载 | [Releases](../../releases) 里的 `szudesktop-<版本>-windows-amd64.zip` |
| 内含 | `szudesktop.exe`（单文件约 8.6 MB）、`README-快速开始.txt`、`LICENSE` |
| 界面 | 像素风（星露谷物语那一路），后端是内嵌的 szunet 内核 |

### 桌面客户端长这样

![szuDesktop 界面](docs/screenshot-desktop.png)

像素风（星露谷物语那一路）：整页一屏放下不滚动，三栏内容用右下角的小木牌
**◀ 1/4 ▶** 翻页；服务农田、宠物、收成架都在。窗口太矮或太窄时自动退回普通滚动。

首次使用：打开程序 → 顶上点「校园网」进登录页 → 填校园卡号和密码 → 勾「记住账号密码」→ 登录
（凭据同样进 Windows DPAPI，不明文落盘）。不勾「记住」也能登，账号只当次有效。
概览页的「一键登录」会直接跳到登录页，登录这件事只在登录页发生。

不做后台自动重连：要不要登录由你在登录页决定，程序不会在背后周期性发认证请求。

命令行参数和 `szunet` 基本一致，另外多了 `--no-open`（不自动开窗口）、
`--no-auto-login`（启动时不自动登录一次）、`--addr`（固定监听地址，默认随机端口）。

### 校外 VPN（内置深信服协议客户端）

深大校外访问内网用的官方方案是深信服 EasyConnect（`ssl.szu.edu.cn` /
`svpn.szu.edu.cn`）。szuNet 内置了该协议的第三方 Go 实现（`internal/vpn`，
基于开源项目 [EasierConnect](https://github.com/acd407/EasierConnect) 的逆向成果移植）：

- **不用装 EasyConnect**：应用里填服务器地址 + 卡号密码（和统一身份认证相同）直接连
- 登录后在本机开一个 **SOCKS5 代理**（默认 `127.0.0.1:7891`），浏览器代理指过去即可访问校内资源
- 断线自动重连，支持短信 / 动态口令二步验证

状态：**协议内核已完成，桌面界面接入开发中**。协议是逆向产物，学校网关固件升级
可能使其失效；仅支持 IPv4 隧道，DNS 仍走本地解析（校内域名解析问题见
`design/vpn-notes.md` 的风险清单）。

### 自己构建

```bash
# 一键：同步页面 → 编译 → 塞图标和版本信息 → 校验
python desktop/build-windows.py

# 跑冒烟测试（起真服务、逐个打接口、检查页面引用的资源有没有 404）
python desktop/smoke_windows.py

# 打发布包
python desktop/make_release.py
```

界面的源文件是 `desktop/index.html`，**只改这一个**；
构建脚本会自动同步到 `desktop/assets/`（浏览器直接打开用）和
`desktop/internal/ui/assets/`（`go:embed` 能看见的那份）。

几个容易踩的地方，脚本里都有注释：

- `go:embed` 不能引用上级目录，所以资源要复制到包内（`sync` 那一步）
- 同一个页面在 `file://` 和 `http://` 下对相对路径的解析基准不同，
  服务端要剥掉 `/assets` 前缀，否则本地打开正常、跑起来满屏破图
- Go 不支持在 Windows exe 里加资源，`desktop/add_resource.py` 直接改 PE，
  塞图标和版本信息，不引入 `windres` 之类的构建依赖

## 已知限制

- `--ip` 用于绕过域名解析时，HTTPS 证书校验仍按原域名进行，不会降级成跳过校验。
- 宿舍区 Dr.COM 的接口在不同楼栋、不同版本的 ePortal 上可能有细微差异，
  如果遇到解析不出来的返回，加 `--verbose` 看原始内容再提 issue。
- 接口一旦被学校改动，认证就会失效——协议细节集中在
  `internal/portal/srun.go` 和 `internal/portal/drcom.go`，改动只涉及这两个文件。

## 免责声明

- 本项目是**第三方作品，与深圳大学无关**，学校不对它负责。
- 请使用**自己的账号**，不要共享、转借账号。
- **不要和官方客户端同时使用**，两者互斥，会互相踢下线。
- 请遵守学校的校园网管理规定。因使用本工具产生的任何后果由使用者自行承担。

## 开发

### 项目结构

```
cmd/szunet/           命令行入口
desktop/              桌面客户端（Go 内嵌网页 UI + Windows 构建脚本）
internal/crypto/      深澜协议用到的加密原语（HMAC-MD5、XXTEA 变体、自定义 Base64、SHA1）
internal/portal/      两套认证协议的客户端 + 区域探测（协议指纹判区）
internal/vpn/         深信服 EasyConnect 协议客户端（校外 VPN，SOCKS5 出口）
internal/credential/  凭据存储，按平台分文件
internal/diagnose/    诊断逻辑
design/               设计脚本（点阵生成器、布局探针、协议学习笔记）
docs/                 排查指南
```

### 跑测试

```bash
go test ./...
go vet ./...
```

`internal/crypto` 里有对照上游实现的测试向量，锁定 xEncode 和自定义 Base64 的行为。

### 发布

打一个 `v*` 开头的 tag 就会触发 GitHub Actions，自动编译三端产物并附加到 Release：

```bash
git tag v0.1.0
git push origin v0.1.0
```

## 致谢

深澜协议的加密实现参考了这些项目（均为 MIT 许可）：

- [Sleepstars/SZU-login](https://github.com/Sleepstars/SZU-login) — XXTEA（xEncode）实现与整体协议流程
- [vidar-team/srun-login](https://github.com/vidar-team/srun-login) — 上游实现

另外参考了这些项目的设计思路：

- [zu1k/srun](https://github.com/zu1k/srun) — 多网卡绑定、自动探测 IP
- [AatroxChen77/szu-net](https://github.com/AatroxChen77/szu-net) — 双区域策略引擎、断线保活思路
- [Sleepstars/SZU_Utils](https://github.com/Sleepstars/SZU_Utils) — 双区识别脚本

校外 VPN 的深信服协议部分移植自 [acd407/EasierConnect](https://github.com/acd407/EasierConnect)
（lyc8503 原版的活跃 fork），协议细节与改造清单见 `design/vpn-notes.md`。
EasyConnect 的一切权利属深信服所有。

## 许可

[MIT](LICENSE)。第三方代码声明见 LICENSE 文件末尾。
