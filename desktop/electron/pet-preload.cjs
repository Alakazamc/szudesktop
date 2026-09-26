// 宠物窗专用 preload：只暴露 IPC 订阅 + 一个「点开主窗口」的单向通知。
// 宠物渲染进程没有任何 fetch/Node 能力，数据全部由主进程推过来。
// 注意：不暴露任何设置写入通道——宠物大小只能从主窗改。
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
  showMain: () => ipcRenderer.send('pet:show-main'),
}));
