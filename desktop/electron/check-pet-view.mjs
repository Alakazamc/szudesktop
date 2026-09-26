import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {PET_ACTIONS} from './pet-policy.mjs';

const here = new URL('.', import.meta.url);
const read = name => readFileSync(new URL(name, here), 'utf8');
const html = read('pet.html');
const render = read('pet-render.mjs');
const preload = read('pet-preload.cjs');

// 每个动作都要有对应的 CSS 规则，否则状态机会推出看不见的动作。
for (const id of Object.keys(PET_ACTIONS)) {
  assert.ok(html.includes(`#pet[data-action="${id}"]`), `pet.html 缺少动作 ${id} 的 CSS 规则`);
  assert.ok(html.includes(`@keyframes pet-${id} `), `pet.html 缺少动作 ${id} 的 keyframes`);
}

// 缩放必须经由 CSS 变量，且默认值为 1。
assert.match(html,/:root\s*\{\s*--pet-scale:\s*1;/);
assert.ok(html.includes('calc(150px * var(--pet-scale))'), '立绘宽高必须随缩放变化');
assert.ok(html.includes('calc(232px * var(--pet-scale))'), '气泡宽度必须随缩放变化');

// CSP 不变：禁内联脚本、default-src 'none'。
assert.match(html,/default-src 'none'/);
assert.match(html,/script-src 'self'/);
assert.doesNotMatch(html,/onclick=/);

// 减少动态效果时必须连动作一起关掉。
assert.match(html,/#pet, #pet\[data-action\] \{ animation: none; \}/);

// 渲染层只选状态，不自己发请求；并从纯策略模块 import。
assert.ok(render.includes("from './pet-policy.mjs'"), '渲染层必须复用纯策略模块');
assert.ok(render.includes("setAttribute('data-action'"), '渲染层必须把状态写进 data-action');
assert.doesNotMatch(render, /\bfetch\s*\(/, '宠物窗渲染层不得发网络请求');

// 一次性动作播放期间不得被主进程轮询打断。
assert.match(render,/if \(Date\.now\(\) < oneShotUntil\) return;/);

// preload 不暴露任意设置值或 IPC；仅允许固定的缩放步进、拖动阶段和菜单请求。
assert.match(preload,/onScale: \(cb\) =>/);
assert.match(preload,/onAction: \(cb\) =>/);
assert.doesNotMatch(preload,/setPetScale|pet-scale-set/, '宠物窗不得拥有任意设置写入能力');
assert.match(preload,/direction === 1 \|\| direction === -1/);
assert.match(preload,/\['start', 'move', 'end'\]\.includes\(phase\)/);
assert.match(render,/pointer\.moved/,'拖动后不得误开菜单');
assert.match(html,/aria-haspopup="menu"/);

// 设置页滑杆必须被 electron 门禁，浏览器模式下不得出现。
const app=read('../assets/garden/app.mjs');
const settings=app.slice(app.indexOf('function settings(){'),app.indexOf('function render(){'));
assert.match(settings,/globalThis\.szuDesktop\?\.shell==='electron'/,'滑杆必须只在安装版渲染');
assert.ok(settings.includes('id="pet-scale"'),'设置页缺少宠物大小滑杆');
assert.ok(settings.includes('id="pet-scale-value"'),'设置页缺少百分比显示');
assert.ok(settings.includes('min="0.4"')&&settings.includes('max="2"'),'滑杆范围必须与 PET_SCALE_MIN/MAX 一致');

// 主进程必须把动作与精力/睡眠一起推给渲染层，否则加权待机永远用默认权重。
const main=read('main.mjs');
assert.match(main,/sendPet\('pet:action',\{id:petActionFor\(pet,null\)/,'主进程必须推送动作载荷');
assert.match(main,/sendPet\('pet:scale',Math\.min\(petScale/,'主进程必须推送适合当前屏幕的缩放');
assert.match(main,/ipcMain\.handle\('szu:pet-scale-get'/);
assert.match(main,/ipcMain\.handle\('szu:pet-scale-set'/);
assert.match(main,/screen\.on\('display-metrics-changed'/,'DPI 变化必须重算布局');
// 绝不开启原生缩放。
assert.doesNotMatch(main,/resizable:\s*true/);
assert.match(main,/petWin\.setBounds\(petWindowBounds\(workArea,petScale,petPosition\)\)/,'显示器变化必须保持可见位置');

// 主窗桥：读回当前值 + 写回归一化值，且不得把宠物大小写进 Go workspace。
const mainPreload=read('preload.cjs');
assert.match(mainPreload,/petScale: \(\) =>/);
assert.match(mainPreload,/setPetScale: \(value\) =>/);

console.log('Pet view: action CSS, scale variables, renderer wiring and preload surface checks passed');
