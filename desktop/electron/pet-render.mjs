// 宠物窗渲染逻辑：只消费主进程经 preload 推来的 pet:state / pet:say / pet:action /
// pet:scale，自己不发任何网络请求（宠物窗是纯本地 file:// 页面）。
// 动画本身全部由 CSS keyframes 播放，本文件只负责「选哪个状态」。
import {PET_ACTIONS, petIdleAction} from './pet-policy.mjs';

const VIEW_BOX = {
  libao: '0 0 52 56',
  'cat-normal': '0 0 20 22',
  'cat-happy': '0 0 20 22',
  'cat-sleep': '0 0 20 22',
  'cat-sad': '0 0 20 22',
};
const SAY_SHOW_MS = 8000;
const IDLE_MIN_MS = 3500;
const IDLE_MAX_MS = 8000;

const pet = document.getElementById('pet');
const use = document.getElementById('pet-use');
const bubble = document.getElementById('bubble');
let hideTimer = null;
let idleTimer = null;
let tick = 0;
let currentPet = null;
let oneShotUntil = 0;
let baseAction = 'idle';

// 未知状态保持原样，不伪造立绘（spec §11：读不到就如实未知）。
function setSprite(key) {
  if (!Object.hasOwn(VIEW_BOX, key)) return;
  use.setAttribute('href', '#' + key);
  pet.setAttribute('viewBox', VIEW_BOX[key]);
}

function applyAction(id) {
  if (!Object.hasOwn(PET_ACTIONS, id)) return;
  pet.setAttribute('data-action', id);
}

// 主进程每 30s 推来的基础动作。正在播放一次性动作时丢弃，避免轮询把动画打断。
function setBaseAction(view) {
  const id = view?.id;
  if (!Object.hasOwn(PET_ACTIONS, id)) return;
  currentPet = {energy: view.energy, sleeping: view.sleeping};
  baseAction = id;
  if (Date.now() < oneShotUntil) return;
  applyAction(id);
}

// 本地触发的一次性动作：用户点击、随机待机。播完自动排下一次待机。
function playOnce(id) {
  const spec = PET_ACTIONS[id];
  if (!spec || spec.kind !== 'once') return;
  oneShotUntil = Date.now() + spec.duration;
  applyAction(id);
  clearTimeout(idleTimer);
  idleTimer = setTimeout(() => {applyAction(baseAction);scheduleIdle();}, spec.duration);
}

function scheduleIdle() {
  clearTimeout(idleTimer);
  const wait = IDLE_MIN_MS + Math.floor(Math.random() * (IDLE_MAX_MS - IDLE_MIN_MS));
  idleTimer = setTimeout(() => {
    const pick = currentPet?.sleeping ? null : petIdleAction(tick++, currentPet);
    if (pick) playOnce(pick);
    else scheduleIdle();
  }, wait);
}

function setScale(value) {
  const n = Number(value);
  if (!Number.isFinite(n)) return;
  document.documentElement.style.setProperty('--pet-scale', String(n));
}

const reduceMotion = () => window.matchMedia('(prefers-reduced-motion: reduce)').matches;

// 镜像庭院 say() 语义：主进程已截到 60 字，这里再兜底一次；
// prefers-reduced-motion 下不加 .pop 弹出动画，直接显示。
function say(text) {
  const line = String(text).slice(0, 60);
  if (!line) return;
  document.getElementById('bubble-text').textContent = line;
  bubble.title = line;
  bubble.classList.remove('pop');
  bubble.classList.add('show');
  if (!reduceMotion()) {
    void bubble.offsetWidth; // 重启动画
    bubble.classList.add('pop');
  }
  clearTimeout(hideTimer);
  hideTimer = setTimeout(() => bubble.classList.remove('show', 'pop'), SAY_SHOW_MS);
}

window.szuPet?.onState(setSprite);
window.szuPet?.onSay(say);
window.szuPet?.onAction(setBaseAction);
window.szuPet?.onScale(setScale);

// 左键点宠物 → 本地播一个回应动作，并请主进程显示/聚焦主窗口。
// （focusable:false 的窗也能收到指针事件。）
pet.addEventListener('pointerdown', (event) => {
  if (event.button !== 0) return;
  playOnce('react');
  window.szuPet?.showMain();
});

// 立绘就位后才开始随机待机，避免首帧就在动。
applyAction('idle');
scheduleIdle();
