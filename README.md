<div align="center">

# szuDesktop · 荔枝庭院

给深大日常留一小块绿地：校园网络、学校入口、学习工具，
和一座离线也会生长的小庭院。

<p>
  <img alt="platform" src="https://img.shields.io/badge/platform-Windows%20%7C%20macOS%20%7C%20Linux-4a6fa5?style=flat-square">
  <img alt="go" src="https://img.shields.io/badge/Go-1.26%2B-00ADD8?style=flat-square&logo=go&logoColor=white">
  <img alt="license" src="https://img.shields.io/badge/license-MIT-2f7d32?style=flat-square">
  <img alt="release" src="https://img.shields.io/github/v/release/Alakazamc/szudesktop?include_prereleases&style=flat-square&label=release&color=c9a227">
  <img alt="runtime" src="https://img.shields.io/badge/runtime-%E6%97%A0%E4%BE%9D%E8%B5%96-6b7280?style=flat-square">
</p>

**简体中文** · [English](docs/README_en.md)

学生自制 · 与深圳大学官方无关

[下载](#下载) · [功能](#功能) · [命令行](#命令行-szunet) · [常见问题](#常见问题) · [安全与隐私](#安全与隐私) · [开发状态](docs/STATUS.md)

</div>

---

## 预览

| 桌面主界面 | 荔枝庭院 | 校园网登录 |
| :--------: | :------: | :--------: |
| ![桌面主界面](docs/screenshot-desktop.png) | ![荔枝庭院](docs/screenshot-garden.png) | ![校园网登录](docs/screenshot-login.png) |

---

## 它解决什么问题

深大校园网分成两个**互不通用**的区域，认证方式完全是两套东西：

| 区域 | 系统 | 协议特点 |
| :--- | :--- | :------- |
| 教学区 / 办公区 / 图书馆 | 深澜（SRun） | 2025 年寒假上线，密码经 HMAC-MD5，用户信息经 XXTEA 加密与自定义 Base64，另需 SHA1 校验和 |
| 宿舍区 / 教工区 | Dr.COM（ePortal） | 一次 GET 请求即可完成 |

由此带来两个麻烦：

1. **2024 年以前的教程和脚本在教学区全部失效** —— 它们都是为老的 Dr.COM 写的。
2. **报错信息五花八门没人解释** —— `ldap auth error`、`Rad:userid error1`、`登陆失败[05]`、
   `Unknow ac-type`…… 用户只能瞎试。

szuDesktop 同时对付这两件事：**一个按钮完成认证，一个按钮说出卡在哪一步。**

判区不看「门户能不能连上」，而看**协议指纹**（就是那套协议特有的握手行为：教学区看
`get_challenge` 能否握手、宿舍区看 ePortal 登录接口是否存在）。原因是实测发现宿舍门户
在教学区机器上首页同样返回 200，只测连通性会在掉线时误判成宿舍区，然后拿 Dr.COM 协议
去打一个不存在的接口。

---

## 下载

到 [Releases](../../releases) 页面下载 `szudesktop-<版本>-windows-amd64.zip`，
解压到任意位置，双击 `szudesktop.exe`。**解压就能用，不用安装。**

| 项 | 说明 |
| :-- | :---- |
| 桌面端 | Windows x64（`szudesktop.exe`） |
| 命令行 | Windows / macOS / Linux，单文件 `szunet` |
| 当前版本 | [beta0.7](https://github.com/Alakazamc/szudesktop/releases/tag/beta0.7) · 公开测试版（预发布），Windows 下载包已发布 |
| 运行环境 | 无需安装依赖；窗口由本机浏览器提供（Edge / Chrome 应用窗口） |

### 首次使用

1. 打开程序，进入「**校园网**」
2. 填校园卡号和统一身份认证密码；想免输就勾「记住账号密码」
3. 点「**登录**」—— 程序会自动识别教学区还是宿舍区，走对应的认证
4. 连不上就点「**断线诊断**」，它会列出区域判定、门户可达性、协议指纹和结论

### 打开与退出

- 同一份存档重复启动会复用本机服务，不会开出第二个程序
- 关闭所有应用窗口约 10 秒后自动退出；想立即退出走「设置 → 退出应用」
- 刷新页面不会结束服务
- 首次打开会给一张可跳过的短引导，说明数据存在哪、怎么退出、先做什么
- 开机自启可在「设置」里开关（命令行对应 `szunet autostart`），状态读不到时会明说「状态未知」，不会显示成未开启

### 我的数据存在哪

- 默认在用户目录 `.szunet` 下
- 校园网密码由 Windows DPAPI 加密（只有这台机器的当前账户能解开），不明文保存
- 庭院存档是 `workspace-v1.json`，普通 JSON，可自行备份；导出的存档不含校园账号密码
- 换电脑前请在设置中导出存档，再在新电脑导入

---

## 功能

beta0.7 为公开测试版。已验证的查询与待验收的账号业务分别标明，详细状态统一见 STATUS.md。

| 能力 | 状态 | 说明 |
| :--- | :--- | :---- |
| 校园网认证 | ✅ | 教学区深澜 / 宿舍区 Dr.COM，自动判区；支持注销与手动指定区域 |
| 接入点编号（`ac_id`）自动发现 | ✅ | 按「手动指定 → 本机在这个网口的记录 → 网关跳转 → 猜测」依次取，猜的会标注 |
| 断线诊断 | ✅ | 列出区域判定、门户可达性、协议指纹与结论 |
| 凭据保管 | ✅ | Windows DPAPI 加密；**认证成功且你勾选记住后**才保存 |
| 校园服务 · 公告 | 部分 | 当前源码支持按学院／部门查看：28 个院系入口，17 个学院栏目可直接读取，另保留教务部、研究生院。日期与原文链接保留，10 分钟缓存；其余院系提供官网入口 |
| 校园服务 · 场地查询与预约 | 部分 | 校内可查社区真实场地和半小时空位；当前源码将预约交给学校官方页面，用户直接在原页登录和提交，不要求复制预约 Cookie。图书馆使用独立官方入口 |
| 学习 · 校历与课表 | 部分 | 官方校历自动更新、教学周可手动调整；本科个人课表和研究生登录读取已接入，**仍待真实账号完整验收** |
| 校园服务 · 自习提醒 | ✅ | 手动登记后导出标准 ICS 日历（开始前 15 分钟）；**提醒不等于预约成功** |
| 校园服务 · 常用电话 | 部分 | 只列能从学校官网核实的号码（图书馆咨询），其他部门只给官方入口 |
| 学习 · 成绩与绩点 | 部分 | 支持粘贴 / CSV / TSV 导入本科与研究生成绩表，按学校规则换算；**不解析 PDF / 图片 / XLSX，不能在线自动同步** |
| 学习 · 待办与专注 | ✅ | 待办清单与 5 / 25 / 45 分钟专注 |
| 荔枝庭院 | ✅ | 伙伴照料与成长、作物、农田、浇水收获、装饰、每日目标、成就与图鉴；无充值与现金交易 |
| 存档 | ✅ | 固定本机文件，跨端口重启恢复，支持导出导入与多窗口冲突保护 |
| 开机自启 | ✅ | 登录 Windows 后静默起服务并自动连一次校园网，不弹窗口；设置里可开关，并显示登记的真实状态 |

**当前限制**：成绩只读第一页；余额未接入；课表仍需手动读取并等待真实账号完整验收。当前源码的预约在学校原页面办理，应用内承载官方页面尚未完成。
现状、真实验收结果与待办统一记在 [docs/STATUS.md](docs/STATUS.md)。

**学院公告筛选**：选择学院后自动读取公开栏目；计软等暂未接入的院系提供官网入口。不包含需登录的校内公告，该更新已发布。

**预约体验修复**：当前源码撤下了预约 Cookie 输入、F12 教程和未经真实验收的本地提交表单。空位为只读预览，点击「登录并预约」在浏览器打开学校页面。这项调整已随 beta0.6.1 发布；原网页内嵌与真实登录流程仍待验证。

---

### 实验性成绩读取

beta0.6.1 起包含本科 / 研究生在线成绩读取，尚未完成真实成绩验收。用户仅在本机应用里输入对应成绩业务的 Cookie；系统安全存储不可用时拒绝保存。验证按培养层次分别进行，无权限与会话失效分开提示；目前只查询第一页，总数未确认或尚未取全时会明确标示。社区预约在学校原页办理，体育场馆尚未接入，不能由成绩验证推断可用。

官方校历与教学周：启动后读取本机缓存，每天检查学校校历页；Windows 在校历图片更新时调用本机系统文字识别，失败保留旧日期并提示，可手动覆盖。研究生提供本地登录与课表读取；本科通过「我的课表」业务 Cookie 读取时间地点原文，两者仍待真实账号完整验收。社区场地可在校园网内读取公开场地和实时空位；当前源码通过「登录并预约」进入学校原页，由用户完成登录、提交并查看结果。beta0.6 曾提供的预约会话输入与实验性本地提交入口已在 beta0.6.1 撤下。当前验收状态统一见 [STATUS.md](docs/STATUS.md)。

## 命令行 szunet

不想开窗口、或者想把认证写进脚本时用。源码在 `cmd/szunet`，
`go build -o dist/szunet.exe ./cmd/szunet` 即可编译。

| 子命令 | 作用 |
| :----- | :--- |
| `login` | 登录（可临时指定 `-u` 卡号 `-p` 密码，不保存） |
| `logout` | 注销下线 |
| `status` | 查看当前状态 |
| `detect` | 探测当前网络区域与接入点编号 |
| `diag` | 断线诊断，输出区域判定与协议指纹 |
| `config` | 查看 / 修改本机配置 |
| `autostart` | 开机自启设置 |
| `version` | 查看版本号 |

```text
szunet detect                       # 看当前识别出的是哪个区域、接入点编号是多少
szunet login --zone teaching        # 指定走教学区协议（auto / teaching / dorm）
szunet login --ac-id 12             # 手动指定接入点编号
szunet diag                         # 连不上时先跑它，再按结论排查
```

`--help` 看全部参数。**不要把真实账号密码写进共享脚本或日志。**

---

## 常见问题

<details>
<summary><b>接了自己的路由器后登录不上</b></summary>

校园网每个接入点有独立编号（`ac_id`），你插哪个网口就得报哪个号。旧版本固定用 `1`，
而路由器那条线路要求 `12`，编号不对会被服务端直接拒绝。

现在程序按下面的顺序找编号，从上往下、哪个先成用哪个：

| 优先级 | 来源 | 可信度 |
| :-- | :-- | :-- |
| 1 | 你自己指定的（`--ac-id 12`） | 最高 |
| 2 | 这台机器在**这个网口**上认证成功用过的 | 高 |
| 3 | 未认证时被网络拦下，从它给的跳转地址里读出来的 | 权威 |
| 4 | 挨个试常见编号 | 低（界面上标「猜的」） |

**编号跟墙上那个网口走，不跟路由器走**：同一台路由器插回原来的口 → 编号不变；
换个口 → 变了；换一台路由器插原口 → 还是原来的编号。

出现「认证失败：ac_id 用错了」时用 `szunet detect` 看当前识别出的接入点。原理解析见
[docs/blog/ac_id-接入点编号.md](docs/blog/ac_id-接入点编号.md)。
</details>

<details>
<summary><b>杀毒软件报警</b></summary>

这是个没有数字签名的单文件程序，属于常见误报。但**不要把安全软件的所有提示都笼统当成误报**：
代码是开源的，可以自己看、自己编译（`go build`）后比对行为。
</details>

<details>
<summary><b>关掉浏览器窗口，程序还在跑吗</b></summary>

关掉**所有**应用窗口约 10 秒后自动退出；期间刷新页面不会结束服务。想立即退出用
「设置 → 退出应用」。想只起服务、不弹窗口，启动 `szudesktop.exe` 时加 `--no-open`。
</details>

<details>
<summary><b>会不会在后台自动重连</b></summary>

不会。要不要登录由你在登录页决定，程序不会在背后周期性发认证请求。
</details>

<details>
<summary><b>账号密码存在哪</b></summary>

Windows 下由 DPAPI 加密，只有这台机器的当前账户能解开，不明文落盘。
**认证成功且勾选「记住账号密码」后才会保存**，密码错误时不会覆盖原来存好的凭据。
随时可以「忘掉账号」。
</details>

---

## 安全与隐私

- **不收集任何数据**：没有 telemetry，没有埋点，所有内容只留在本机
- **凭据本地加密**：Windows DPAPI，换机器或换用户都解不开
- **业务在学校页面办理**：当前源码中的社区预约、选课和付款由你在官方系统操作；本应用不自动抢位或代提交
- **不做绕过计费或共享上网的功能**，请遵守学校网络使用规定
- **官方业务只走预设来源**：公告只读取学校公开页面，不提供任意网址代理

---

## 构建与验证

需要 Go（版本见 `go.mod`）、Python 3 和 Node.js。

```text
python desktop/sync-assets.py      # 同步界面资源
node   desktop/check-ui.mjs        # 界面规则回归
node   desktop/check-campus.mjs    # 校园服务与成绩导入回归
node   desktop/check-autostart-ui.mjs # 设置页开机自启回归
go vet ./... && go test ./...      # 静态检查与单元测试
python desktop/build-windows.py    # 构建 Windows 桌面版
python desktop/smoke_windows.py    # 整机冒烟
python desktop/make_release.py     # 生成发布包
```

界面唯一源文件是 `desktop/index.html` 与 `desktop/assets/garden/`，
构建产物不要手改。默认桌面构建**排除实验 VPN 协议**与未核实的第三方游戏原型美术，
只提供官方 WebVPN 入口；实验源码保留供来源核验与协议研究，不能按已验收功能宣传。

---

## 致谢

- [teleostnacl/LoveSzu](https://github.com/teleostnacl/LoveSzu) —— 本科个人课表接口路径与字段的调研参考；按接口事实独立实现，没有复制其源码
- [Fusion Pixel Font（缝合像素字体）](https://github.com/TakWolf/fusion-pixel-font) ——
  TakWolf，SIL Open Font License 1.1，声明随包附在 `FONT-LICENSE-OFL.txt`
- [Sleepstars/SZU-login](https://github.com/Sleepstars/SZU-login) ——
  深澜 xEncode 实现的来源归属保留在 [LICENSE](LICENSE) 中
- [Clash Verge Rev](https://github.com/clash-verge-rev/clash-verge-rev) 与
  [福大助手](https://github.com/west2-online/fzuhelper-app) —— 界面层级与校园服务组织的参考
- [MattDong123/tools4szu](https://github.com/MattDong123/tools4szu) —— 作者 Matt，
  经其授权用于学校系统接口调研。本项目没有复制它的代码，而是按它记录的接口地址、
  数据表名与字段名，用 Go 重新实现了一套带会话失效判定的读取逻辑。
  该仓库未声明开源许可，因此这里只作事实性致谢，不构成对其代码的再分发

---

## 许可

MIT，见 [LICENSE](LICENSE)。

其中第三方组件的原有许可不受本项目 MIT 替代：Fusion Pixel 字体遵循 OFL 1.1；
实验 VPN 模块的第三方来源与授权范围仍在核对中，默认桌面构建不包含该模块。
EasyConnect 等第三方名称的权利归各自权利人所有。
