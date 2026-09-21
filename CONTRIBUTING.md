# 参与贡献

欢迎提 issue 和 PR。这个项目处理校园网密码和学校业务系统的登录 Cookie，
所以有几条硬规矩，动手前请先看完。

**唯一事实源是 [docs/STATUS.md](docs/STATUS.md)**：所有问题编号（F/U/D/X/Q/R 系列）、
功能范围、验收记录和发布历史都在那里。本文只讲「怎么改代码、怎么验证」，
不重复维护另一套任务清单。

## 环境

| 依赖 | 用途 |
|---|---|
| Go | 版本见 `go.mod`，编译 CLI 与桌面服务 |
| Python 3 | 构建、冒烟、打包脚本 |
| Node.js | 前端回归检查脚本（`desktop/check-*.mjs`） |

Windows 上构建桌面版需要本机有 Edge 或 Chrome（冒烟测试会起真实浏览器窗口）。

## 界面资源的规矩

- **唯一源文件**：`desktop/index.html` 与 `desktop/assets/garden/`。
- `desktop/assets/index.html` 和 `desktop/internal/ui/assets/` 是 `python desktop/sync-assets.py`
  生成的副本，已被 gitignore，**不要手改**。
- 改完界面先跑 `sync-assets.py`，再跑检查脚本，否则检查读到的是旧副本。
- 页面源码里不允许写死版本号：顶栏与关于页从 `/api/status` 的 `app_version` 取，
  `check-ui.mjs` 和 `smoke_windows.py` 各有回归守着。

## 提交前请跑

```text
python desktop/sync-assets.py
node desktop/check-ui.mjs            # 以下 10 个前端回归，CI 全部会跑
node desktop/check-campus.mjs
node desktop/check-notices.mjs
node desktop/check-session-ui.mjs
node desktop/check-academic.mjs
node desktop/check-school.mjs
node desktop/check-booking.mjs
node desktop/check-network-ui.mjs
node desktop/check-workspace-ui.mjs
node desktop/check-autostart-ui.mjs
go vet ./...
go test ./...
python desktop/check_release_notes.py   # 发布说明抽取回归
```

改了桌面端还要在 Windows 上跑：

```text
python desktop/build-windows.py      # 构建
python desktop/smoke_windows.py      # 整机冒烟（74 项）
```

**关于 `gofmt`**：在 Windows 工作副本上直接跑 `gofmt -l .` 会把几乎所有文件列出来，
那是 CRLF 行尾造成的，不是格式问题。CI 不跑 gofmt；要检查格式请在 Linux 检出后跑，
或用 `gofmt -d` 看具体差异再判断。

## 硬规矩（红线）

1. **不许谎报成功。** 读不到状态就报「状态未知」，请求失败就报错，
   绝不把失败退化成假数据或演示模式。「读不到 ≠ 没有」。
2. **凭据不进聊天、不进源码、不进发布包、不进日志。** 测试一律用
   `SZUNET_CONFIG_DIR` 隔离配置目录，不使用真实账号。
3. **不代用户提交。** 预约、选课、付款、签到、抢位一律交给学校官方页面完成，
   界面不留代提交入口，服务端也不留（见 STATUS.md F23）。
4. **不新增任意转发。** 业务聚合只走允许的目标清单，不做通用代理，不绕认证。
5. **来源没核实就不要宣称独立实现。** 代码和素材先核实来源与许可（见 F11）。
6. **测试只终止自己启动的进程**，不调整用户现有的系统代理。
7. **文档只改 STATUS.md 这一套**，历史资料冻结，不再同步多套清单。

## 修 bug 的方式

先写一个能**复现这个 bug 的失败测试**，亲眼看它红，再改代码让它变绿。
这个仓库里已有的例子：

- `cmd/szunet/status_query_test.go` — F22：CLI 没账号时跳过在线查询
- `desktop/internal/ui/booking_test.go` — F23：预约写端点必须不存在（断言 404）
- `internal/credential/store_unavailable_test.go` — F24：没有密钥环时拒绝把密码写成明文

新发现的问题请按 STATUS.md 的编号体系追加一行（功能/安全/工程用 `F`，
排版与交互用 `U`），写清重要程度（P0–P3）、难度（S/M/L）、状态和验收标准。
「已完成」必须附验收证据；没验证过的就写「未验证」，不要含糊过去。

## 发布

- 版本号只有一个来源：`internal/version/VERSION`。发版时改这个文件，
  构建脚本、打包脚本、页面顶栏与关于页都会跟着走。
- **发布说明只有一个来源：根目录的 `CHANGELOG.md`。** 升版本号的同时，
  把 `## 未发布` 那一节的标题改成新版本号并补齐内容。说明写给用户看：
  说清「相对上一版有什么变化」和「哪些还没验证」，不要只写内部编号。
  - `python desktop/release_notes.py <版本号>` 可以本地预览将要发出去的正文。
  - 抽不到那一节、正文是空的、或正文里还留着 `__VERSION__`，
    `make_release.py` 会在打包前失败，CI 的 release job 也会在上传附件前失败。
  - 这道关卡是有来历的：beta0.7 发出去时正文只有一行自动生成的 compare 链接。
    本项目直接提交到 main、没有 PR，GitHub 的 `generate_release_notes`
    拿不到任何可分类的内容，所以说明必须自己写。
- 打 `beta*` 或 `v*` 标签会触发 `.github/workflows/release.yml`：
  test（ubuntu）→ test-macos（探针，不阻断）→ build-cli（5 平台）→
  build-desktop-windows → release。
- **不要在未发布的改动上跑 `python desktop/make_release.py`**：它会覆盖
  与 GitHub Release 对应的本地包，导致线上附件没法再和本地产物逐字节核对。
- 发布后要核对附件：`sha256sum -c`、ZIP 内 exe 与独立 exe 是否逐字节一致、
  程序自报版本是否等于 VERSION 文件、**Release 正文是否真的是你写的那一节**。
  核对结果写进 STATUS.md。

安全问题的报告方式见 [SECURITY.md](SECURITY.md)，**不要**开公开 issue 写可利用细节。
