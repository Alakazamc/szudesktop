// 宠物窗渲染逻辑：只消费主进程经 preload 推来的 pet:state / pet:say，
// 自己不发任何网络请求（宠物窗是纯本地 file:// 页面）。
const VIEW_BOX = {
  libao: '0 0 52 56',
  'cat-normal': '0 0 20 22',
  'cat-happy': '0 0 20 22',
  'cat-sleep': '0 0 20 22',
  'cat-sad': '0 0 20 22',
};
const SAY_SHOW_MS = 8000;

const pet = document.getElementById('pet');
const use = document.getElementById('pet-use');
const bubble = document.getElementById('bubble');
let hideTimer = null;

// 未知状态保持原样，不伪造立绘（spec §11：读不到就如实未知）。
function setSprite(key) {
  if (!Object.hasOwn(VIEW_BOX, key)) return;
  use.setAttribute('href', '#' + key);
  pet.setAttribute('viewBox', VIEW_BOX[key]);
}

const reduceMotion = () => window.matchMedia('(prefers-reduced-motion: reduce)').matches;

// 镜像庭院 say() 语义：主进程已截到 60 字，这里再兜底一次；
// prefers-reduced-motion 下不加 .pop 弹出动画，直接显示。
function say(text) {
  const line = String(text).slice(0, 60);
  if (!line) return;
  bubble.textContent = line;
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

// 左键点宠物 → 请主进程显示/聚焦主窗口（focusable:false 的窗也能收到指针事件）。
pet.addEventListener('pointerdown', (event) => {
  if (event.button === 0) window.szuPet?.showMain();
});
