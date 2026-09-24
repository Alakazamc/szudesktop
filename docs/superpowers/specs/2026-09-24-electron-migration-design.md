# szuDesktop → Electron 迁移设计（外壳 + Go sidecar）

- 日期：2026-09-24
- 状态：已获作者批准（方案 1），待写实施计划
- 决策人：Alakazamc（项目作者）
- 关联：取代 `desktop/internal/ui/server.go:705 openBrowser()` 的浏览器 `--app` 窗口路径

## 1. 背景与现状（已核对代码）

当前是两层结构：

- **Go 引擎**：`desktop/cmd/szudesktop` 起一个本机回环 HTTP 服务（`127.0.0.1:0` 随机端口），
  用 `go:embed` 把前端打进二进制。承载校园网认证（深澜/Dr.COM）、凭据存储（钥匙串/keyring）、
  VPN、系统代理、开机自启、教务（ehall）抓取、日历、场地查询、`/api/workspace` 存档。
  全部 `/api/*` 路由走本地环回保护 `protectAPI`。
- **静态前端**：单一来源 `desktop/index.html` + `desktop/assets/garden/*.mjs`；
  生成副本被 gitignore，靠 `desktop/sync-assets.py` / `build-windows.py` 同步。
- **窗口**：`openBrowser()` 在 Windows 上找 Edge/Chrome 用 `--app=<url> --start-maximized`
  开一个无地址栏窗口；找不到就退回 `rundll32` 默认浏览器。macOS `open`、Linux `xdg-open`。

关键事实（决定可行性）：

1. Go 服务已支持 `--no-open`，并在启动时打印 `szuDesktop 已启动: http://127.0.0.1:<port>`
   ——天然适合被当作 sidecar 子进程拉起、并从 stdout 解析端口。
2. 荔枝庭院 / 宠物存档落在**服务端** `/api/workspace`（JSON 文件，带 `revision` 乐观锁），
   不是 localStorage——所以一个独立的宠物窗能透过同一套 Go API 读写同一份宠物状态。
3. 前端逻辑回归测试是 10 个 `desktop/check-*.mjs`（node，直接 import `assets/garden/*.mjs`）；
   服务端冒烟是 `desktop/smoke_windows.py`。两者都不依赖「窗口怎么开」，迁移后原样有效。

## 2. 目标 / 非目标

**目标**

- 用 Electron 作为**唯一**的窗口与桌面集成层，提供浏览器 `--app` 模式做不到的能力：
  真·无边框、透明、始终置顶、不进任务栏的**常驻桌面宠物窗**。
- 宠物能在**别的 agent 完成工作时冒一句话**（台词复用现有 `say()` 机制）。
- 满足作者硬约束：**不引入重复技术栈**——一套窗口（Electron）、一套引擎（Go）、一份前端来源。

**非目标（本次不做，YAGNI）**

- 不重写 Go 引擎（认证/凭据/VPN/教务一律不动）。
- 不在第一阶段做「文件哨兵」猜测 agent 状态（见 §6 回退项）。
- 不做飞书组织 / 共享笔记平台（另一条线，单独 brainstorm）。
- 不承诺覆盖作者列出的全部 agent；只接「确证有生命周期钩子」的那几个（见 §6）。

## 3. 「不重复技术栈」的落地解释

迁移后**删除** `openBrowser()` 的浏览器 `--app` 启动路径，Electron 成为唯一开窗方式。
最终栈：

- 窗口/桌面集成：**Electron**（唯一）
- 后端引擎：**Go sidecar**（唯一，子进程）
- 前端来源：**`desktop/index.html`**（唯一，仍由 Go `go:embed` 提供，Electron 主窗口 `loadURL` 加载它）

不存在「两套窗口」或「两套后端」。`cmd/szunet` CLI 是同一套 Go 的命令行入口，不算 UI 栈，保留不动。

## 4. 选定方案与被否方案

**方案 1（选定）：Electron 外壳 + Go sidecar。**
Electron 主进程 spawn Go 二进制（`--no-open`），解析端口；主窗口加载 `http://127.0.0.1:<port>/`；
宠物窗是独立 BrowserWindow；Go 仅新增宠物事件端点。已审计/已测试的内核零改动，风险低。

**方案 2（否决）：全量 Node/TS 重写，弃 Go。**
唯一好处是「更纯粹的单语言」，但要把深澜握手、凭据钥匙串、教务一次性 challenge 这些
踩过坑、且 F21/F24/F26 刚修过的脆弱代码全部重赌一遍，回归风险与工作量都不可接受。否。

## 5. 架构

```
Electron 主进程 (TypeScript)
 ├─ app ready → spawn Go sidecar:  szudesktop --no-open
 │     └─ 读 stdout 直到匹配 "已启动: http://127.0.0.1:<port>"，拿到 port
 ├─ 主窗口 BrowserWindow
 │     └─ loadURL(http://127.0.0.1:<port>/)        ← 现有前端原样运行
 ├─ 宠物窗 BrowserWindow（frame:false, transparent:true, alwaysOnTop, skipTaskbar, resizable:false）
 │     └─ 加载本地 pet.html；preload 用 contextBridge 暴露最小 IPC
 ├─ 托盘 Tray（显示主窗口 / 显隐宠物 / 退出）
 ├─ 订阅 Go 的 GET /api/pet/events (SSE)
 │     └─ 收到 finish 事件 → webContents.send 给宠物窗 → 冒话
 └─ 退出时 kill sidecar（进程树清理）
Go sidecar（desktop/cmd/szudesktop，几乎不改）
 ├─ 现有全部 /api/* 不变
 └─ 新增：POST /api/pet/event（写事件）、GET /api/pet/events（SSE 推事件），均走 protectAPI
```

### 5.1 组件职责（单一职责、接口清晰、可独立测）

- **sidecar 监督器（main/sidecar.ts）**：拉起 Go、解析端口、健康探测、崩溃重启（限次）、退出清理。
  对外只暴露 `start(): Promise<{port, baseUrl}>` 与 `stop()`。
- **主窗口（main/mainWindow.ts）**：建窗、加载 baseUrl、处理外部链接（`setWindowOpenHandler` 走系统浏览器）。
- **宠物窗（main/petWindow.ts + renderer/pet.html）**：置顶无边框窗、精灵动画、台词气泡。
  只接收主进程 IPC（`pet:set-state`、`pet:say`），不直接访问 Go。
- **事件桥（main/agentEvents.ts）**：订阅 Go SSE，把 agent 事件转成 IPC 发给宠物窗。
- **托盘（main/tray.ts）**：菜单与显隐。
- **Go 宠物端点（desktop/internal/ui/pet.go）**：内存 event bus + 最新状态表；
  `POST /api/pet/event` 收事件，`GET /api/pet/events` 用 SSE 推；端点信息文件见 §6。

### 5.2 数据流

- **启动**：Electron → spawn Go → 解析端口 → 主窗口 loadURL → 前端照旧驱动 `/api/*`。
- **agent 完成 → 宠物冒话**：
  agent 钩子 `curl POST /api/pet/event {agent,status:"finish",detail}` →
  Go event bus → SSE 推 → Electron 事件桥 → IPC `pet:say` → 宠物窗气泡（如「Claude 干完啦」）。
- **宠物游戏状态**：宠物窗（或主窗口庭院页）读写 `/api/workspace` 里同一份存档，复用 `petSprite`/`say`。

## 6. agent 状态采集（最难、最易失真，据实写）

核心约束：**szuDesktop 自身不是 agent**，无法原生得知别的 agent 是否完工。只能「对方主动报」或「盯文件猜」。

**主路（采用）：事件端点 + 生命周期钩子。**

- Go 启动时把 `{port, token}` 写到固定路径的端点信息文件：
  - Windows：`%APPDATA%/szudesktop/pet-endpoint.json`
  - macOS/Linux：`~/.config/szudesktop/pet-endpoint.json`
  - `token` 为每次启动随机生成的一次性串，`POST /api/pet/event` 校验它，避免本机其他进程乱发。
- 支持钩子的 agent 配一行脚本：读该文件拿 `port`+`token`，在「停止/通知」事件时 `curl` POST 过来。
- **能接（确证有 hooks/notify）**：Claude Code、Codex(gpt)、Qoder、Cursor。
- **暂不接（未确证钩子，不假装能读）**：workbuddy、antigravity、hermes、openclaw、trae、
  deepseek-harness、piagent。逐个调研后再决定；「读不到 ≠ 没有」，UI 不得谎报这些 agent 的状态。

**回退（以后再说，不进第一阶段）：文件哨兵。**
盯 `~/.claude ~/.codex ~/.qoder ~/.trae-cn ~/.cursor` 等会话/日志目录变动来猜开始/结束。
各家格式不一、多数不写「完成」信号，脆且难维护，故仅作日后插件式扩展，event bus 已为它留口。

## 7. 常驻宠物窗规格

- 窗口：`frame:false`、`transparent:true`、`alwaysOnTop:true`（`screen-saver` 级）、
  `skipTaskbar:true`、`resizable:false`、`focusable:false`（默认不抢焦点），默认贴屏幕右下角。
- 精灵：复用 `petSprite()`——荔宝（默认）/ 栗栗（cat-normal/happy/sad/sleep）。
- 台词：复用 `say()`，60 字上限；`prefers-reduced-motion` 下不做弹出动画。
- 交互：左键点击切换「显示主窗口」；右键托盘菜单控制显隐/退出；可选鼠标穿透模式。
- 这是浏览器 `--app` 模式**做不到**的能力，也是本次迁移的主要用户可见收益。

## 8. 分阶段（每阶段独立可验收、可回退）

- **阶段 0 · 可行性尖峰（一次性、可扔）**：本机证明「Electron spawn Go sidecar + 加载现有 UI +
  弹一个置顶无边框宠物窗」能跑，先用真机截图给作者看到常驻宠物。产物标注 throwaway。
- **阶段 1 · 外壳迁移**：Electron 成唯一窗口栈；删 `openBrowser` 浏览器路径；Go 打成 sidecar
  随包分发；electron-builder 出 win/mac/linux 安装包；改 README 定位（放弃「单文件无依赖」措辞）。
- **阶段 2 · 宠物窗**：精灵状态、台词气泡、托盘、开机随启动。
- **阶段 3 · agent 状态**：端点信息文件 + `POST /api/pet/event` + SSE + Claude/Codex/Qoder/Cursor
  钩子适配；完成时宠物冒话。

每阶段各走自己的 spec → plan → 实现 → 验收循环。本文件是总体设计；阶段 0/1 先出实施计划。

## 9. 风险与取舍

- **安装包体积** ~10MB → ~120MB（Chromium 运行时）。作者已明确接受。
- **CI 复杂度**：需同时具备 Node 工具链 + Go；Electron 跨三端打包。
- **macOS 签名/公证**：Electron 引入后签名链更复杂，与既有 F26（钥匙串）线叠加，需单独验证。
- **定位变更**：README「单文件、双击就跑、无依赖」必须改写——这是真实且需同步的对外口径变化。
- **迁移回归**：改的是已发布产品的开窗方式；阶段化 + 保留 `--no-open` 无头模式作为回退路径。
- **sidecar 生命周期**：Go 子进程崩溃/僵尸需监督器兜底（健康探测 + 限次重启 + 退出 kill 进程树）。

## 10. 测试策略

- 保留：`desktop/check-*.mjs`（前端逻辑）、`desktop/smoke_windows.py`（Go 服务）、`go vet`/`go test`。
- 新增：sidecar 启动/端口解析/崩溃重启的单测；宠物窗与事件桥的 e2e（Playwright 驱 Electron）；
  `POST /api/pet/event` + SSE 的 Go 端点测试（含 token 校验、本地环回保护）。
- 阶段 0 以真机截图为验收证据（作者要「看到」常驻宠物）。
- 验收一律给真实证据，不报假成功。

## 11. 安全与既有约束（沿用，不因迁移放宽）

- 凭据只由用户在本地 UI 输入；端点信息文件只含 `port`+一次性 `token`，**不含任何账号密码**。
- `/api/pet/event` 仅本机环回 + token 校验；不新增任何服务端写学校系统的端点（沿用 F23 红线）。
- 不自动提交预约/选课/签到；不拿真账密试探线上错误信息。
- 公开仓库不得出现真实凭据、内部拓扑、未发布接口。
- 宠物/agent 状态「读不到」时如实显示未知，绝不伪造。

## 12. 待决 / deferred

- 其余 agent（workbuddy/antigravity/hermes/openclaw/trae/deepseek-harness/piagent）钩子能力逐个调研。
- 文件哨兵回退是否做、做哪几家。
- Electron 主进程语言细节（electron-vite vs 纯 tsc）、打包目标格式（NSIS/dmg/AppImage）在阶段 1 计划里定。
- 飞书组织 / 共享笔记平台：另案。
