# szuDesktop

深圳大学校园网客户端。像素风桌面应用 + 命令行，一套内核（Go），Windows / macOS / Linux
都能编译成**单个可执行文件**，不依赖 Python、Node 之类的运行时。

自动判断你在**教学区**还是**宿舍区**，走对应的那套认证协议；连不上的时候，
告诉你卡在哪一步。

![szuDesktop 界面](docs/screenshot-desktop.png)

---

## 当前进度

**首版只做校园网**：判区、登录、注销、断线诊断，都接了真内核。

| 功能 | 状态 |
|---|---|
| 教学区登录（深澜 SRun） | ✅ 已实现 |
| 宿舍区登录（Dr.COM ePortal） | ✅ 已实现，待真机复测 |
| 自动判区（协议指纹，不靠连通性）+ 手动指定 | ✅ 已实现 |
| 注销下线 / 断线诊断 / 账号存系统保险箱 | ✅ 已实现 |
| 校外 VPN（内置深信服 EasyConnect 协议） | 🚧 协议内核已完成，界面接入开发中 |
| 图书馆 / 一卡通 / 教务 / 服务等页面 | 🚧 开发中（目前是占位页） |
| 余额、流量、在线时长等真实数据 | 🚧 开发中（界面上标「演示」的数字是写死的示例） |
| macOS / Linux 客户端 | 🚧 开发中（先做 Windows） |

## 怎么用

### 下载

到 [Releases](../../releases) 页面下载 `szudesktop-<版本>-windows-amd64.zip`，
解压到任意位置，双击 `szudesktop.exe`（绿色版，不用安装）。

### 首次使用

1. 打开程序（会弹一个无边框窗口，任务栏图标是荔枝）
2. 顶上点「**校园网**」标签，进登录页
3. 填校园卡号和统一身份认证密码，勾「记住账号密码」（不勾也能登，这次有效）
4. 点「登录」

凭据存进 Windows 保险箱（DPAPI 加密），**不明文落盘**，换机器或换用户都解不开。
不放心随时点「忘掉账号」。

连不上就先点「断线诊断」：它会列出区域判定、门户可达性、协议指纹和对应的结论，
比单纯一句「登录失败」有用得多。

### 界面说明

- **一屏放下，不滚动**：整页固定在一个窗口里，左中右三栏各自用右下角的小木牌
  **◀ 1/4 ▶** 翻页；窗口太矮或太窄时自动退回普通滚动
- 宠物、农田、收成架是游戏化外壳（星露谷那一路），不影响功能
- **不做后台自动重连**：要不要登录由你在登录页决定，程序不会在背后周期性发认证请求
- 关掉窗口不等于退出：程序还在后台。要彻底退，任务管理器结束 `szudesktop.exe`

### 命令行参数

| 参数 | 说明 |
|---|---|
| `--addr` | 固定监听地址（默认随机端口），如 `--addr 127.0.0.1:8620` |
| `--no-open` | 只起服务，不自动开窗口 |
| `--no-auto-login` | 启动时不自动登录一次 |
| `-u / -p` | 临时指定账号密码（不落盘） |
| `--zone` | 强制指定区域：`auto`（默认）/ `teaching` / `dorm` |
| `--verbose` | 打印服务端原始返回，排错用 |

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

szuDesktop 同时对付这两件事：一个按钮完成认证，一个按钮完成诊断。

判区**不看「门户能不能连上」，而看协议指纹**（深澜看 `get_challenge` 能不能握手、
宿舍区看 ePortal 登录接口在不在）。原因是实测发现宿舍门户在教学区机器上首页也返回 200，
只测连通性会在掉线时误判成宿舍区，然后拿 Dr.COM 协议去打一个 404 的接口。

## 为什么自己做

社区已经有十来个深大校园网工具了，这个项目的取舍是：

- **带图形界面的单文件**。同类工具大多只有命令行，或者要装 Python / Node 运行时；
  这个把界面编译进 exe，在本机回环地址起个小服务驱动内核，双击就能跑。
- **三端原生单文件**。Go 写成，加密逻辑是纯 Go 实现，交叉编译出来直接跑。
- **凭据进系统保险箱**。macOS 用钥匙串，Windows 用 DPAPI（换机器、换用户都解不开），
  Linux 用 Secret Service。很多同类工具把密码明文写在配置文件里。
- **诊断优先**。诊断不是简单报「登录失败」，而是把区域判定、门户可达性、
  协议指纹、账号在线状态列出来，并给出对应结论。
  配套排查思路见 [`docs/深圳大学校园网连不上排查指南.md`](docs/深圳大学校园网连不上排查指南.md)。
- **认证请求强制直连**。显式禁用系统代理，避免请求被 Clash / Mihomo 这类工具
  抓走——这是校园网认证失败的一个高频原因，但很少有工具处理它。

## 账号密码存在哪

**不落明文盘。** 各平台用各平台自己的安全设施：

| 平台 | 存放方式 |
|---|---|
| Windows | DPAPI 加密后写文件，密钥由当前用户账户派生 |
| macOS | 钥匙串（Keychain），条目 `service=szunet` |
| Linux | Secret Service（gnome-keyring / KWallet）；机器上没有则退化成 `~/.szunet/credentials.json`，权限 600 |

## 命令行 szunet

同一个内核还带一个命令行工具，适合脚本和没有图形界面的机器。

```bash
szunet detect          # 看看你在哪个区、网络通不通
szunet config set -u 2023xxxx -p 你的密码   # 存一次账号（进系统保险箱）
szunet login           # 登录
szunet diag            # 连不上时跑这个
```

| 命令 | 作用 |
|---|---|
| `szunet login` / `logout` | 登录 / 注销 |
| `szunet status` | 看当前在哪个区、账号在不在线 |
| `szunet detect` | 只探测网络区域，不做认证 |
| `szunet diag` | 连不上时跑这个，给出排查结论 |
| `szunet config set/show/delete` | 管理保存的账号密码 |
| `szunet version` | 看版本 |

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

账号密码优先级：命令行参数 > 环境变量（`SZUNET_USERNAME` / `SZUNET_PASSWORD`）> 已保存的凭据。

`internal/portal` 里的两套协议客户端也可以直接 import 当库用：

```go
c := portal.NewSrunClient(portal.DefaultSrunHost, user, pass)
res, err := c.Login()
```

## 自己构建

需要 Go 1.26+，以及 Python 3（只用于桌面端的构建脚本）。

```bash
git clone https://github.com/Alakazamc/szudesktop.git
cd szudesktop

# 命令行
go build -o szunet ./cmd/szunet

# 桌面端：同步页面 → 编译 → 塞图标和版本信息 → 校验
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

- **改完记得重新打开程序**：界面是编译进 exe 的，旧窗口不会自动更新。
- `--ip` 用于绕过域名解析时，HTTPS 证书校验仍按原域名进行，不会降级成跳过校验。
- 宿舍区 Dr.COM 的接口在不同楼栋、不同版本的 ePortal 上可能有细微差异，
  如果遇到解析不出来的返回，加 `--verbose` 看原始内容再提 issue。
- 接口一旦被学校改动，认证就会失效——协议细节集中在
  `internal/portal/srun.go` 和 `internal/portal/drcom.go`，改动只涉及这两个文件。
- 界面上的余额、流量、在线时长是演示数据（学校没有公开接口）。

## 免责声明

- 本项目是**第三方作品，与深圳大学无关**，学校不对它负责。
- 请使用**自己的账号**，不要共享、转借账号。
- **不要和官方客户端同时使用**，两者互斥，会互相踢下线。
- 请遵守学校的校园网管理规定。因使用本工具产生的任何后果由使用者自行承担。

## 开发

### 项目结构

```
cmd/szunet/           命令行入口
desktop/              桌面客户端
  index.html            界面源文件（唯一要改的那份）
  build-windows.py      构建：同步页面 → 编译 → 注入图标版本 → 校验
  smoke_windows.py      冒烟测试（40 项）
  add_resource.py       纯 Python 改 PE 加图标/版本信息
  internal/ui/          本地服务 + 界面接口（/api/*）
internal/crypto/      深澜协议用到的加密原语（HMAC-MD5、XXTEA 变体、自定义 Base64、SHA1）
internal/portal/      两套认证协议的客户端 + 区域探测（协议指纹判区）
internal/vpn/         深信服 EasyConnect 协议客户端（校外 VPN，开发中）
internal/credential/  凭据存储，按平台分文件
internal/diagnose/    诊断逻辑
design/               设计与学习笔记（点阵生成器、布局探针、VPN 协议笔记）
docs/                 排查指南
```

### 跑测试

```bash
go test ./...          # 加密原语有对照上游实现的测试向量
go vet ./...
python desktop/smoke_windows.py   # 桌面端 40 项冒烟
```

### 发布

首个公开测试版为 `beta0.1`。发布前必须依次通过全仓测试、页面静态检查、真实点击验收和 Windows 整机冒烟，再生成 ZIP：

```bash
python desktop/build-windows.py
python desktop/design/check_login_e2e.py
python desktop/smoke_windows.py
python desktop/make_release.py
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

界面素材与字体版权归原作者所有，本地原型用途。

## 许可

[MIT](LICENSE)。第三方代码声明见 LICENSE 文件末尾。
