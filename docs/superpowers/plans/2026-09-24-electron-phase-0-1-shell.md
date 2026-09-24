# Electron 阶段 0/1（可行性尖峰 + 外壳迁移）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 szuDesktop 的窗口层从浏览器 `--app` 模式迁到 Electron——先本机证明「Electron 拉起 Go sidecar + 加载现有 UI + 弹常驻置顶宠物窗」可行（阶段 0，可扔），再把 Electron 外壳做成正式、可打包的 Windows 应用（阶段 1，保留）。

**Architecture:** Electron 主进程 spawn 现有 Go 二进制（`desktop/cmd/szudesktop`，带 `--no-open`）作为 sidecar，从其 stdout 解析 `http://127.0.0.1:<port>`，主窗口 `loadURL` 加载它——前端与 95% 的 Go 代码零改动。electron-builder 把 Go exe 作为 extraResource 打进 NSIS 安装包。单一窗口栈（Electron）、单一引擎（Go）、单一前端来源（`desktop/index.html`）。

**Tech Stack:** Electron（plain ESM JavaScript，无 TypeScript、无打包器、无 React）；electron-builder（Windows NSIS）；Go sidecar（现有）；测试沿用本仓库自定义极简 harness（`node desktop/electron/check-*.mjs`）。

**Spec:** `docs/superpowers/specs/2026-09-24-electron-migration-design.md`

## Global Constraints

- 不重写 Go 引擎；校园网认证/凭据/VPN/教务代码一行不动。
- sidecar 一律用 `--no-open` 启动，Electron 是唯一开窗者。
- 端口只有一个来源：解析 sidecar stdout 的 `szuDesktop 已启动: http://127.0.0.1:<port>`（`desktop/internal/ui/server.go:191`）。
- 凭据只由用户在本地 UI 输入；任何新文件/端点不得含账号密码。
- 公开仓库不得出现真实凭据、内部拓扑、未发布接口。
- 验收给真实证据（截图 / 命令输出），不报假成功；「读不到 ≠ 没有」。
- Electron 主进程/preload/renderer 用 plain ESM JS，`"type":"module"`；不引入 TS/ bundler / React。
- 现有回归（`desktop/check-*.mjs`、`desktop/smoke_windows.py`、`go vet`、`go test`）必须继续通过。
- 版本号单一来源 `internal/version/VERSION`（当前 `beta0.7.3`），不得另写一份。
- 阶段 0 产物是一次性证明，放 `.scratch_probe/`，不作为产品代码提交。

## 与 spec 的偏差（执行前必读）

spec §3 写「阶段 1 删除 `openBrowser` 浏览器路径」。落地核查发现该路径与三处纠缠：
`internal/autostart/autostart_windows.go:28`（开机自启用 ` --no-open` 注册 Go exe）、
`desktop/internal/ui/instance.go:102`（第二实例把首实例 URL 用浏览器打开）、
`desktop/smoke_windows.py`（冒烟全程依赖 `--no-open`）。
因此**阶段 1 只做加法**（新增 Electron 外壳并打包，sidecar 用 `--no-open`，Go 的浏览器路径暂留为死代码但不动），
把「删 Go 浏览器路径 + 开机自启改指向 Electron + 单实例改由 Electron 锁」整体挪到**阶段 2**（与托盘/宠物一起），
避免在 Electron 打包尚未验证前就拆掉现有可用路径。README 定位改写仍在阶段 1（Task 1.8）。

---

# Part A · 阶段 0：可行性尖峰（一次性、可扔）

目的：在作者这台 Windows 机上，用最小代码证明三件有风险的事——(1) Electron 能 spawn 现有 Go exe 并解析端口、(2) 主窗口能加载现有 UI、(3) 能弹出真·无边框透明置顶宠物窗。产物在 `.scratch_probe/`，验证以截图为准，之后可整目录删除。

### Task 0.1: 准备一个可被拉起的 Go sidecar

**Files:**
- 复用产物：`dist/szudesktop-windows-amd64.exe`（由 `desktop/build-windows.py` 生成）

- [ ] **Step 1: 构建现有 Windows 桌面 exe（含内嵌页面与图标）**

Run（PowerShell，作者可粘）:
```
python desktop/build-windows.py
```
Expected: 末尾打印 `完成。下一步跑冒烟测试:`，且 `dist/szudesktop-windows-amd64.exe` 存在。

- [ ] **Step 2: 确认它能无头启动并打印端口**

Run:
```
dist\szudesktop-windows-amd64.exe --no-open --no-auto-login --addr 127.0.0.1:0
```
Expected: stdout 出现一行 `szuDesktop 已启动: http://127.0.0.1:<某端口>`。看到后 Ctrl+C 结束。记下这行格式，Task 0.2 的正则按它写。

### Task 0.2: 最小 Electron 外壳（spawn sidecar + 主窗口 + 宠物窗）

**Files:**
- Create: `.scratch_probe/electron-spike/package.json`
- Create: `.scratch_probe/electron-spike/main.mjs`
- Create: `.scratch_probe/electron-spike/pet.html`

- [ ] **Step 1: 写尖峰 package.json**

`.scratch_probe/electron-spike/package.json`:
```json
{
  "name": "szudesktop-electron-spike",
  "private": true,
  "type": "module",
  "main": "main.mjs",
  "scripts": { "start": "electron ." },
  "devDependencies": { "electron": "^33.0.0" }
}
```

- [ ] **Step 2: 装 Electron（仅尖峰目录）**

Run:
```
cd .scratch_probe/electron-spike
npm install
```
Expected: 生成 `node_modules/` 与 `package-lock.json`，无报错。

- [ ] **Step 3: 写尖峰主进程**

`.scratch_probe/electron-spike/main.mjs`:
```js
import {app, BrowserWindow} from 'electron';
import {spawn} from 'node:child_process';
import path from 'node:path';

const SIDECAR = path.resolve('..', '..', 'dist', 'szudesktop-windows-amd64.exe');

function parseListenUrl(text) {
  const m = /http:\/\/(127\.0\.0\.1|localhost):(\d+)/.exec(text || '');
  return m ? `http://127.0.0.1:${m[2]}` : null;
}

function startSidecar() {
  return new Promise((resolve, reject) => {
    const child = spawn(SIDECAR, ['--no-open', '--no-auto-login', '--addr', '127.0.0.1:0'], {
      stdio: ['ignore', 'pipe', 'inherit'],
    });
    let buf = '';
    const timer = setTimeout(() => reject(new Error('sidecar 启动超时')), 15000);
    child.stdout.setEncoding('utf8');
    child.stdout.on('data', (chunk) => {
      buf += chunk;
      const url = parseListenUrl(buf);
      if (url) { clearTimeout(timer); resolve({url, child}); }
    });
    child.on('exit', (code) => { clearTimeout(timer); reject(new Error('sidecar 退出，码 ' + code)); });
  });
}

let mainWin, petWin, child;

async function boot() {
  const side = await startSidecar();
  child = side.child;

  mainWin = new BrowserWindow({width: 1200, height: 820, title: 'szuDesktop'});
  await mainWin.loadURL(side.url);

  petWin = new BrowserWindow({
    width: 220, height: 220, frame: false, transparent: true, resizable: false,
    alwaysOnTop: true, skipTaskbar: true, focusable: false, hasShadow: false,
    webPreferences: {contextIsolation: true, nodeIntegration: false},
  });
  petWin.setAlwaysOnTop(true, 'screen-saver');
  await petWin.loadFile('pet.html');
  const {width, height} = petWin.getBounds();
  petWin.setPosition(Math.round(width * -1 + (await screenWidth()) - 260), Math.round((await screenHeight()) - height - 60));
}

import {screen} from 'electron';
const screenWidth = async () => screen.getPrimaryDisplay().workAreaSize.width;
const screenHeight = async () => screen.getPrimaryDisplay().workAreaSize.height;

app.whenReady().then(boot).catch((e) => { console.error(e); app.quit(); });
app.on('window-all-closed', () => { if (child) child.kill(); app.quit(); });
```

- [ ] **Step 4: 写宠物窗页面（静态证明：精灵 + 气泡）**

`.scratch_probe/electron-spike/pet.html`:
```html
<!doctype html><html lang="zh-CN"><head><meta charset="utf-8">
<style>
  html,body{margin:0;background:transparent;overflow:hidden;font-family:system-ui,sans-serif}
  #wrap{position:relative;width:220px;height:220px;display:flex;align-items:flex-end;justify-content:center}
  #bubble{position:absolute;top:8px;left:50%;transform:translateX(-50%);max-width:190px;
    background:#fffdf6;border:2px solid #e7c9a9;border-radius:14px;padding:8px 12px;
    color:#5a4632;font-size:14px;line-height:1.4;box-shadow:0 4px 14px rgba(0,0,0,.18)}
  #pet{width:120px;height:120px;border-radius:50%;
    background:radial-gradient(circle at 40% 35%,#ffd9a0,#f0a860 70%);
    border:3px solid #d98b45;display:flex;align-items:center;justify-content:center;font-size:48px}
</style></head>
<body><div id="wrap"><div id="bubble">荔宝待命中～有 agent 干完活我会喊你</div><div id="pet">🐾</div></div></body></html>
```

- [ ] **Step 5: 启动尖峰并肉眼验收**

Run:
```
cd .scratch_probe/electron-spike
npm start
```
Expected（用眼睛看 + 截图存证）:
1. 出现一个 szuDesktop 主窗口，里面是现有荔枝庭院界面（不是空白、不是报错页）。
2. 屏幕右下角出现一个无边框、背景透明、始终置顶的宠物小窗，显示气泡文字。
3. 把别的应用窗口拖到宠物窗上方——宠物窗仍在最前（验证 `alwaysOnTop`）。
4. 关闭主窗口 → 进程退出，任务管理器里 `szudesktop-windows-amd64.exe` 也随之消失（验证 `child.kill()`）。

- [ ] **Step 6: 记录结论**

把三件事的成败与截图路径写进本任务的执行记录（不留产品代码）。若任一失败，**停**：这是尖峰的价值——在投入阶段 1 前暴露 Electron/Windows 的坑（常见：transparent+alwaysOnTop 在某些显卡驱动下发黑、`focusable:false` 导致气泡不刷新）。尖峰目录验证完即可 `rm -rf .scratch_probe/electron-spike`。

---

# Part B · 阶段 1：正式外壳迁移（保留、TDD）

目的：把尖峰验证过的结构做成正式、可测、可打包的 `desktop/electron/`，产出 Windows NSIS 安装包，并在 CI 里构建。**本阶段不交付宠物窗**（宠物是阶段 2）；主窗口 + sidecar 监督 + 单实例 + 打包 + CI + 文档定位即为完成。

## 文件结构（阶段 1 落地）

- `desktop/electron/package.json` — Electron 应用清单，`"type":"module"`，`main: main.mjs`，devDeps：`electron`、`electron-builder`。
- `desktop/electron/listen-url.mjs` — 纯函数 `parseListenUrl(text)`，从 stdout 文本解析 baseUrl。单独成文件以便单测。
- `desktop/electron/sidecar.mjs` — 监督器 `startSidecar(opts)` / `stopSidecar(handle)`：spawn、解析端口、健康探测、退出清理（含 Windows 进程树）。
- `desktop/electron/main.mjs` — 应用入口：单实例锁、起 sidecar、建主窗口、生命周期清理。
- `desktop/electron/testdata/fake-sidecar.mjs` — 测试替身：打印 listen 行并提供 `/api/status`，让监督器测试跨平台可跑（不依赖 Windows exe）。
- `desktop/electron/check-sidecar.mjs` — `parseListenUrl` 与 `startSidecar` 的回归测试（自定义 harness）。
- `desktop/electron/electron-builder.yml` — Windows NSIS 打包配置，extraResources 带入 Go exe。
- `desktop/electron/build.mjs` — 一键：编 Go sidecar → electron-builder 出安装包。
- 修改：`.github/workflows/release.yml`（新增 Windows Electron 构建 job + 把 check-sidecar 纳入 test job）。
- 修改：`README.md`、`docs/README_en.md`（桌面版定位措辞）。
- 修改：`docs/STATUS.md`、`CHANGELOG.md`（记录）。

### Task 1.1: 端口解析纯函数（TDD）

**Files:**
- Create: `desktop/electron/listen-url.mjs`
- Test: `desktop/electron/check-sidecar.mjs`

**Interfaces:**
- Produces: `parseListenUrl(text: string): string | null` —— 命中返回 `http://127.0.0.1:<port>`，否则 `null`。

- [ ] **Step 1: 写失败测试**

`desktop/electron/check-sidecar.mjs`:
```js
import assert from 'node:assert/strict';
import {parseListenUrl} from './listen-url.mjs';
let checks=0;function test(name,fn){fn();checks++;console.log('PASS',name)}

test('parseListenUrl: 解析打印出的监听地址',()=>{
  assert.equal(parseListenUrl('szuDesktop 已启动: http://127.0.0.1:52344\n'),'http://127.0.0.1:52344');
});
test('parseListenUrl: 行还没出现时返回 null',()=>{
  assert.equal(parseListenUrl('正在启动...\n'),null);
  assert.equal(parseListenUrl(''),null);
});
test('parseListenUrl: 忽略之后的自动登录噪声',()=>{
  const out='szuDesktop 已启动: http://127.0.0.1:6001\n[自动登录] 成功\n';
  assert.equal(parseListenUrl(out),'http://127.0.0.1:6001');
});
test('parseListenUrl: localhost 归一到 127.0.0.1',()=>{
  assert.equal(parseListenUrl('http://localhost:7000'),'http://127.0.0.1:7000');
});

console.log(checks+' checks passed');
```

- [ ] **Step 2: 跑测试，确认因缺实现而失败**

Run: `node desktop/electron/check-sidecar.mjs`
Expected: FAIL —— `Cannot find module '.../listen-url.mjs'`。

- [ ] **Step 3: 写最小实现**

`desktop/electron/listen-url.mjs`:
```js
// 端口只有一个来源：sidecar 打印的 "szuDesktop 已启动: http://127.0.0.1:<port>"。
export function parseListenUrl(text){
  const m=/http:\/\/(127\.0\.0\.1|localhost):(\d+)/.exec(text||'');
  return m?`http://127.0.0.1:${m[2]}`:null;
}
```

- [ ] **Step 4: 跑测试，确认通过**

Run: `node desktop/electron/check-sidecar.mjs`
Expected: PASS，末行 `4 checks passed`。

- [ ] **Step 5: 提交**

```bash
git add desktop/electron/listen-url.mjs desktop/electron/check-sidecar.mjs
git commit -m "feat(electron): 解析 sidecar 监听地址的纯函数 + 回归"
```

### Task 1.2: sidecar 监督器（TDD，用 node 替身跨平台测）

**Files:**
- Create: `desktop/electron/sidecar.mjs`
- Create: `desktop/electron/testdata/fake-sidecar.mjs`
- Modify: `desktop/electron/check-sidecar.mjs`

**Interfaces:**
- Consumes: `parseListenUrl` (Task 1.1)。
- Produces:
  - `startSidecar({command, args, env, cwd, readyPath='/api/status', readyTimeoutMs=15000, healthTimeoutMs=8000}): Promise<{baseUrl, child, stop(): Promise<void>}>`
  - 解析不到端口或健康探测超时则 reject。

- [ ] **Step 1: 写测试替身**

`desktop/electron/testdata/fake-sidecar.mjs`:
```js
import http from 'node:http';
const srv=http.createServer((req,res)=>{
  if(req.url==='/api/status'){res.writeHead(200,{'content-type':'application/json'});res.end('{"ok":true}');return;}
  res.writeHead(404);res.end();
});
srv.listen(0,'127.0.0.1',()=>{
  const port=srv.address().port;
  process.stdout.write(`szuDesktop 已启动: http://127.0.0.1:${port}\n`);
});
```

- [ ] **Step 2: 追加失败测试**

在 `desktop/electron/check-sidecar.mjs` 末尾（`console.log` 之前）加：
```js
import {startSidecar} from './sidecar.mjs';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
const here=path.dirname(fileURLToPath(import.meta.url));
const fake=path.join(here,'testdata','fake-sidecar.mjs');

test('startSidecar: 拉起替身、解析端口、健康探测通过、可停止',async()=>{
  const handle=await startSidecar({command:process.execPath,args:[fake]});
  try{
    assert.match(handle.baseUrl,/^http:\/\/127\.0\.0\.1:\d+$/);
    const r=await fetch(handle.baseUrl+'/api/status');
    assert.equal(r.status,200);
  } finally { await handle.stop(); }
});
```
并把末行改成异步收尾：
```js
// 顶层 await：node 直接跑 .mjs 支持
console.log(checks+' checks passed');
```
（将 `test` 调用改为可等待：把上面这条 test 写成 `await (async()=>{...})()` 形式，或把整个文件用顶层 `await` 串起来。最简做法：把该 test 的 fn 设为 async 并在文件末尾 `await` 一个收集 promise——见 Step 3 的 harness 调整。）

- [ ] **Step 3: 让 harness 支持 async 测试**

把 `desktop/electron/check-sidecar.mjs` 顶部的 harness 换成：
```js
import assert from 'node:assert/strict';
import {parseListenUrl} from './listen-url.mjs';
import {startSidecar} from './sidecar.mjs';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
const here=path.dirname(fileURLToPath(import.meta.url));
const fake=path.join(here,'testdata','fake-sidecar.mjs');
let checks=0;const queue=[];
function test(name,fn){queue.push((async()=>{await fn();checks++;console.log('PASS',name)})());}
```
文件末尾改为：
```js
await Promise.all(queue);
console.log(checks+' checks passed');
```
并把 Task 1.1 的同步 test 保持不变（async harness 兼容同步 fn）。异步那条 test 直接写 `test('...', async ()=>{...})`。

- [ ] **Step 4: 跑测试，确认因缺 sidecar.mjs 而失败**

Run: `node desktop/electron/check-sidecar.mjs`
Expected: FAIL —— `Cannot find module '.../sidecar.mjs'`。

- [ ] **Step 5: 写监督器实现**

`desktop/electron/sidecar.mjs`:
```js
import {spawn, execFile} from 'node:child_process';
import {parseListenUrl} from './listen-url.mjs';

async function waitForUrl(child, timeoutMs){
  return new Promise((resolve,reject)=>{
    let buf='';const timer=setTimeout(()=>reject(new Error('sidecar 启动超时（没打印监听地址）')),timeoutMs);
    child.stdout.setEncoding('utf8');
    child.stdout.on('data',(c)=>{buf+=c;const url=parseListenUrl(buf);if(url){clearTimeout(timer);resolve(url);}});
    child.on('exit',(code)=>{clearTimeout(timer);reject(new Error('sidecar 提前退出，码 '+code));});
  });
}

async function waitHealthy(baseUrl, readyPath, timeoutMs){
  const deadline=Date.now()+timeoutMs;
  while(Date.now()<deadline){
    try{const r=await fetch(baseUrl+readyPath);if(r.ok)return;}catch{}
    await new Promise(r=>setTimeout(r,150));
  }
  throw new Error('sidecar 健康探测超时: '+baseUrl+readyPath);
}

export async function startSidecar({command,args=[],env,cwd,readyPath='/api/status',readyTimeoutMs=15000,healthTimeoutMs=8000}){
  const child=spawn(command,args,{env:env||process.env,cwd,stdio:['ignore','pipe','inherit']});
  const baseUrl=await waitForUrl(child,readyTimeoutMs);
  await waitHealthy(baseUrl,readyPath,healthTimeoutMs);
  return {baseUrl,child,stop:()=>stopSidecar(child)};
}

export function stopSidecar(child){
  return new Promise((resolve)=>{
    if(!child||child.exitCode!=null||child.killed){resolve();return;}
    const done=()=>resolve();
    child.once('exit',done);
    try{child.kill('SIGKILL');}catch{}
    if(process.platform==='win32'&&child.pid){
      // Windows 下子进程不随父进程自动结束，杀整棵进程树兜底
      execFile('taskkill',['/pid',String(child.pid),'/T','/F'],()=>{});
    }
    setTimeout(done,3000);
  });
}
```

- [ ] **Step 6: 跑测试，确认通过**

Run: `node desktop/electron/check-sidecar.mjs`
Expected: PASS，末行 `5 checks passed`。

- [ ] **Step 7: 提交**

```bash
git add desktop/electron/sidecar.mjs desktop/electron/testdata/fake-sidecar.mjs desktop/electron/check-sidecar.mjs
git commit -m "feat(electron): sidecar 监督器（拉起/解析/健康探测/进程树清理）+ 回归"
```

### Task 1.3: Electron 应用清单与主进程入口

**Files:**
- Create: `desktop/electron/package.json`
- Create: `desktop/electron/main.mjs`

**Interfaces:**
- Consumes: `startSidecar` / `stopSidecar` (Task 1.2)。
- Produces: 可 `npm start` 启动的 Electron 应用；主窗口加载 sidecar UI；单实例锁。

- [ ] **Step 1: 写 package.json**

`desktop/electron/package.json`:
```json
{
  "name": "szudesktop-desktop",
  "productName": "szuDesktop",
  "version": "0.0.0",
  "private": true,
  "type": "module",
  "main": "main.mjs",
  "scripts": {
    "start": "electron .",
    "dist": "node build.mjs",
    "check": "node check-sidecar.mjs"
  },
  "devDependencies": {
    "electron": "^33.0.0",
    "electron-builder": "^25.0.0"
  }
}
```
说明：`version` 在打包时由 `build.mjs` 用 `internal/version/VERSION` 覆盖，避免版本号两处写死。

- [ ] **Step 2: 写主进程入口**

`desktop/electron/main.mjs`:
```js
import {app, BrowserWindow, shell} from 'electron';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
import {startSidecar, stopSidecar} from './sidecar.mjs';

// dev: 仓库里 dist/ 的 exe；打包后: resources 里的 extraResource
function sidecarCommand(){
  const exe=process.platform==='win32'?'szudesktop-windows-amd64.exe':'szudesktop';
  if(app.isPackaged) return {command:path.join(process.resourcesPath,exe),args:['--no-open']};
  const here=path.dirname(fileURLToPath(import.meta.url)); // desktop/electron
  const repoRoot=path.resolve(here,'..','..');
  return {command:path.join(repoRoot,'dist',exe),args:['--no-open']};
}

let handle=null, mainWin=null;

async function boot(){
  const {command,args}=sidecarCommand();
  handle=await startSidecar({command,args});
  mainWin=new BrowserWindow({width:1200,height:820,title:'szuDesktop',
    webPreferences:{contextIsolation:true,nodeIntegration:false}});
  // 外链交给系统浏览器，不在应用内开新窗
  mainWin.webContents.setWindowOpenHandler(({url})=>{shell.openExternal(url);return {action:'deny'};});
  await mainWin.loadURL(handle.baseUrl);
  mainWin.on('closed',()=>{mainWin=null;});
}

const gotLock=app.requestSingleInstanceLock();
if(!gotLock){ app.quit(); }
else{
  app.on('second-instance',()=>{ if(mainWin){ if(mainWin.isMinimized())mainWin.restore(); mainWin.focus(); }});
  app.whenReady().then(boot).catch((e)=>{ console.error('启动失败',e); app.quit(); });
  app.on('window-all-closed',async()=>{ if(handle)await stopSidecar(handle.child); app.quit(); });
  app.on('before-quit',async()=>{ if(handle)await stopSidecar(handle.child); });
}
```

- [ ] **Step 3: 装依赖**

Run:
```
cd desktop/electron
npm install
```
Expected: 生成 `node_modules/` 与 `package-lock.json`（**两者都要提交**，CI 用 `npm ci`）。

- [ ] **Step 4: 本机启动验收（需先有 dist exe）**

Run（先确保 `dist/szudesktop-windows-amd64.exe` 存在，见 Task 0.1 Step 1）:
```
cd desktop/electron
npm start
```
Expected: 弹出 szuDesktop 主窗口，内为现有界面；关闭窗口后任务管理器中 sidecar 进程消失。截图存证。

- [ ] **Step 5: 提交**

```bash
git add desktop/electron/package.json desktop/electron/package-lock.json desktop/electron/main.mjs
git commit -m "feat(electron): 应用入口——单实例锁、起 sidecar、主窗口加载现有 UI"
```

### Task 1.4: 打包配置与一键构建脚本

**Files:**
- Create: `desktop/electron/electron-builder.yml`
- Create: `desktop/electron/build.mjs`
- Modify: `.gitignore`（忽略 Electron 打包输出）

- [ ] **Step 1: 写 electron-builder 配置**

`desktop/electron/electron-builder.yml`:
```yaml
appId: com.szudesktop.app
productName: szuDesktop
directories:
  output: release
  buildResources: build
files:
  - main.mjs
  - sidecar.mjs
  - listen-url.mjs
  - package.json
extraResources:
  - from: ../../dist/szudesktop-windows-amd64.exe
    to: szudesktop-windows-amd64.exe
win:
  target: nsis
  icon: ../assets/szudesktop.ico
nsis:
  oneClick: false
  allowToChangeInstallationDirectory: true
  artifactName: szuDesktop-Setup-${version}.exe
```

- [ ] **Step 2: 写一键构建脚本**

`desktop/electron/build.mjs`:
```js
import {execFileSync} from 'node:child_process';
import {readFileSync, writeFileSync} from 'node:fs';
import path from 'node:path';
import {fileURLToPath} from 'node:url';

const here=path.dirname(fileURLToPath(import.meta.url));
const repoRoot=path.resolve(here,'..','..');

// 1. 编 Go sidecar（复用现有脚本：同步页面 + 编译 + 打图标/版本）
execFileSync('python',[path.join(repoRoot,'desktop','build-windows.py')],{cwd:repoRoot,stdio:'inherit'});

// 2. 用单一来源版本号覆盖 package.json 的 version（不另写死）
const ver=readFileSync(path.join(repoRoot,'internal','version','VERSION'),'utf8').trim();
if(!ver){console.error('VERSION 为空');process.exit(1);}
const pkgPath=path.join(here,'package.json');
const pkg=JSON.parse(readFileSync(pkgPath,'utf8'));
pkg.version=ver.replace(/^beta/,''); // electron-builder 要 x.y.z；beta0.7.3 -> 0.7.3
writeFileSync(pkgPath,JSON.stringify(pkg,null,2)+'\n');

// 3. 打包
execFileSync('npx',['electron-builder','--win','--config','electron-builder.yml'],{cwd:here,stdio:'inherit'});
console.log('\n完成。安装包在 desktop/electron/release/，版本 '+ver);
```

- [ ] **Step 3: 忽略打包输出与依赖**

在 `.gitignore` 追加：
```
desktop/electron/node_modules/
desktop/electron/release/
```

- [ ] **Step 4: 本机出包验收**

Run:
```
cd desktop/electron
npm install
node build.mjs
```
Expected: `desktop/electron/release/` 下出现 `szuDesktop-Setup-<ver>.exe`（约 100MB+）。双击安装→启动→出现主窗口与现有界面。截图存证。若 electron-builder 报图标/证书错，先去掉 `win.icon` 再试，定位是图标格式还是签名问题。

- [ ] **Step 5: 提交**

```bash
git add desktop/electron/electron-builder.yml desktop/electron/build.mjs .gitignore
git commit -m "feat(electron): electron-builder 打包（NSIS）+ 一键构建脚本，版本号取自 VERSION"
```

### Task 1.5: 把 sidecar 回归纳入 CI test job

**Files:**
- Modify: `.github/workflows/release.yml:42`（在 autostart 回归后插入一步）

- [ ] **Step 1: 在 test job 增加一步**

在 `release.yml` 的 `test` job、`- name: Settings autostart UI regression` 之后、`- name: 发布说明抽取回归` 之前插入：
```yaml
      - name: Electron sidecar 回归
        run: node desktop/electron/check-sidecar.mjs
```
（该测试用 node 替身，不依赖 Windows exe，ubuntu 可跑。）

- [ ] **Step 2: 本地等价校验**

Run: `node desktop/electron/check-sidecar.mjs`
Expected: PASS（与 CI 同命令）。

- [ ] **Step 3: 提交**

```bash
git add .github/workflows/release.yml
git commit -m "ci: test job 增加 Electron sidecar 回归"
```

### Task 1.6: 新增 Windows Electron 安装包构建 job（仅产出 artifact，暂不接入 release）

**Files:**
- Modify: `.github/workflows/release.yml`（在 `build-desktop-windows` job 之后新增一个 job）

- [ ] **Step 1: 新增 job**

在 `build-desktop-windows` job 之后插入：
```yaml
  build-desktop-electron-windows:
    needs: test
    runs-on: windows-latest
    steps:
      - uses: actions/checkout@v7.0.1
      - uses: actions/setup-go@v7.0.0
        with:
          go-version-file: go.mod
      - uses: actions/setup-python@v7.0.0
        with:
          python-version: '3.13'
      - uses: actions/setup-node@v6.0.0
        with:
          node-version: '22'
      - name: 构建 Go sidecar
        run: python desktop/build-windows.py
      - name: 装 Electron 依赖
        run: npm ci
        working-directory: desktop/electron
      - name: 打 Windows 安装包
        run: npx electron-builder --win --config electron-builder.yml
        working-directory: desktop/electron
      - uses: actions/upload-artifact@v7.0.1
        with:
          name: szudesktop-electron-windows
          path: desktop/electron/release/*.exe
          if-no-files-found: error
```
说明：**暂不**把此 job 加进 `release` 的 `needs`——先让作者本机验证安装包可用，再决定是否随 tag 公开发布（见 Task 1.9 的 go/no-go）。

- [ ] **Step 2: 校验 YAML 缩进与 job 依赖**

Run（本地无 YAML 解析器，作者可粘；CI 上以 push 后运行为准）:
```
python -c "import yaml,sys;yaml.safe_load(open('.github/workflows/release.yml',encoding='utf-8'));print('YAML OK')"
```
Expected: `YAML OK`（若本机无 pyyaml，则改为推一个分支触发 CI 看是否解析通过）。

- [ ] **Step 3: 提交**

```bash
git add .github/workflows/release.yml
git commit -m "ci: 新增 Windows Electron 安装包构建 job（产出 artifact，暂不接入 release）"
```

### Task 1.7: 全量回归与无头路径不破

**Files:** 无（验证任务）

- [ ] **Step 1: 跑前端 + 服务端 + Go 全套回归**

Run:
```
node desktop/check-ui.mjs
node desktop/check-campus.mjs
node desktop/check-notices.mjs
node desktop/check-session-ui.mjs
node desktop/check-academic.mjs
node desktop/check-school.mjs
node desktop/check-booking.mjs
node desktop/check-network-ui.mjs
node desktop/check-workspace-ui.mjs
node desktop/check-autostart-ui.mjs
node desktop/electron/check-sidecar.mjs
go vet ./...
go test ./...
```
Expected: 全部 PASS，无回归。

- [ ] **Step 2: 确认现有无头/冒烟路径未受影响**

Run: `python desktop/smoke_windows.py`
Expected: 与迁移前一致通过（阶段 1 未改 Go 行为，sidecar 仍 `--no-open`）。

- [ ] **Step 3: 若有失败，停并修；全绿则记录**

把本步输出作为阶段 1 的回归证据存档（执行记录），不单独提交。

### Task 1.8: 改写桌面版定位措辞（README 中英）

**Files:**
- Modify: `README.md`（桌面版下载/运行段落，约第 200 行附近 `--no-open` 说明处）
- Modify: `docs/README_en.md:92`

- [ ] **Step 1: 改中文 README**

把「双击单个 exe 即跑、无依赖」这类对**桌面版**的表述，改为：桌面版现在是 Electron 应用（NSIS 安装包，约 100MB+，内含 Chromium 运行时与 Go 引擎 sidecar）；**命令行版 `szunet` 仍是单文件、无依赖**。保留 `--no-open` 说明，但注明它面向 sidecar/无头场景。给出新的下载/安装/运行步骤。

- [ ] **Step 2: 改英文 README**

`docs/README_en.md:92` 同步：desktop app is now an Electron installer (bundles Chromium + Go sidecar); the `szunet` CLI remains a single dependency-free binary; keep the `--no-open` note scoped to headless/sidecar use.

- [ ] **Step 3: 自查措辞与事实一致**

确认没有残留「桌面版单文件无依赖」的旧话；安装包体积、组成与 Task 1.4 实测一致。

- [ ] **Step 4: 提交**

```bash
git add README.md docs/README_en.md
git commit -m "docs: 桌面版定位改为 Electron 安装包；CLI 仍单文件无依赖"
```

### Task 1.9: 记录状态与变更日志（含发布 go/no-go）

**Files:**
- Modify: `docs/STATUS.md`（追加一节：阶段 0/1 完成证据、偏差、未决）
- Modify: `CHANGELOG.md`（在对应版本节追加「桌面版迁移到 Electron 外壳」条目）

- [ ] **Step 1: 写 STATUS 一节**

记录：阶段 0 尖峰三件事的结论与截图路径；阶段 1 落地的文件、CI job、回归输出；与 spec 的偏差（openBrowser 删除/autostart 改指/单实例改 Electron 锁挪到阶段 2）；未决（mac/linux Electron 打包、release 接入 go/no-go）。

- [ ] **Step 2: 写 CHANGELOG 条目**

在下一版本号那一节（发布前由作者定版本号；**VERSION 仍为 beta0.7.3 时不要跑 make_release.py**）加一条：桌面版窗口层从浏览器 `--app` 迁到 Electron 外壳，Go 引擎作为 sidecar；新增 Windows Electron 安装包构建（暂随 CI artifact，未接入公开 release）。

- [ ] **Step 3: 提交**

```bash
git add docs/STATUS.md CHANGELOG.md
git commit -m "docs: 记录 Electron 阶段 0/1 结果、偏差与发布 go/no-go"
```

- [ ] **Step 4: 发布 go/no-go（交给作者决定，不自动发布）**

向作者汇报：本机安装包是否可用、CI artifact 是否产出、是否要把 `build-desktop-electron-windows` 接入 `release.needs` 并随下一个 tag 公开。**未获明确同意不接入 release、不打 tag、不推送。**

---

## Self-Review（写计划后自查）

**1. spec 覆盖：**
- spec §5 架构（Electron 主进程 + sidecar + 主窗口）→ Task 1.2/1.3。
- spec §5.1 sidecar 监督器/主窗口职责 → Task 1.2/1.3。
- spec §8 阶段 0（尖峰三件事 + 截图）→ Task 0.1/0.2。
- spec §8 阶段 1（唯一窗口栈、打包、CI、README 定位）→ Task 1.3–1.8。
- spec §9 风险（体积、CI 复杂度、定位变更、sidecar 生命周期）→ Task 1.4（体积实测）、1.5/1.6（CI）、1.8（定位）、1.2（进程树清理）。
- spec §10 测试（保留 check-*/smoke/go + 新增 sidecar 单测）→ Task 1.1/1.2/1.7。
- spec §11 安全（不含凭据、本地环回）→ Global Constraints + sidecar 仅本机 127.0.0.1。
- **spec §6 agent 状态、§7 宠物窗 = 阶段 2/3，本计划不覆盖（已在 Goal/Part B 目的中声明）。**
- **spec §3「删除 openBrowser」→ 本计划有意挪到阶段 2，见「与 spec 的偏差」，已记录理由。**

**2. 占位符扫描：** 无 TBD/TODO；每个代码步给了完整可跑代码；文档步给了确切措辞方向与行号锚点。Task 1.2 Step 2/3 对 harness 异步化给了完整改法，未留「类似上文」。

**3. 类型/命名一致：** `parseListenUrl`（1.1 定义，1.2 消费，签名一致）；`startSidecar`/`stopSidecar`（1.2 定义，1.3 消费，参数与返回 `{baseUrl,child,stop}` 一致）；sidecar exe 名 `szudesktop-windows-amd64.exe` 在 0.1/1.3/1.4/CI 一致；版本来源 `internal/version/VERSION` 全程唯一。

**4. 待执行时定死的值：** Electron `^33.0.0`、electron-builder `^25.0.0`、CI node `22`——执行 `npm install` 时以解析到的稳定版为准并写进 `package-lock.json`；若 ^33 装不上，记录实际可用主版本再继续。
