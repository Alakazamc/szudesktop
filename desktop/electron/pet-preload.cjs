// 宠物窗专用 preload：仅暴露状态订阅和受限的菜单、缩放步进、拖动通知。
// 宠物渲染进程没有任何 fetch/Node 能力，数据全部由主进程推过来。
// 不接收路径或任意频道；拖动只传事件坐标，主进程将窗口限制在显示器工作区。
const {contextBridge, ipcRenderer} = require('electron');
contextBridge.exposeInMainWorld('szuPet', Object.freeze({
  onState: (cb) => {
    if (typeof cb !== 'function') return;
    ipcRenderer.on('pet:state', (_event, key) => cb(String(key)));
  },
  onSay: (cb) => {
    if (typeof cb !== 'function') return;
    ipcRenderer.on('pet:say', (_event, text) => cb(String(text).slice(0, 60)));
  },
  // 主进程推送的基础动作与配套的精力/睡眠状态，供渲染层做加权随机待机。
  onAction: (cb) => {
    if (typeof cb !== 'function') return;
    ipcRenderer.on('pet:action', (_event, payload) => {
      const view = payload && typeof payload === 'object' ? payload : {};
      cb({id: String(view.id ?? ''), energy: Number(view.energy), sleeping: Boolean(view.sleeping)});
    });
  },
  // 主进程推送的缩放倍数，用于驱动 CSS 变量。
  onScale: (cb) => {
    if (typeof cb !== 'function') return;
    ipcRenderer.on('pet:scale', (_event, value) => cb(Number(value)));
  },
  onReaction: (cb) => {
    if (typeof cb === 'function') ipcRenderer.on('pet:react', () => cb());
  },
  openMenu: () => ipcRenderer.send('pet:menu'),
  scaleStep: (direction) => {
    if (direction === 1 || direction === -1) ipcRenderer.send('pet:scale-step', direction);
  },
  drag: (phase, point) => {
    if (['start', 'move', 'end'].includes(phase) && Number.isFinite(point?.x) && Number.isFinite(point?.y)) {
      ipcRenderer.send('pet:drag', phase, {x:Math.round(point.x), y:Math.round(point.y)});
    }
  },
}));
