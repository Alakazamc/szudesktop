// 宠物窗专用 preload：只暴露两个 IPC 订阅 + 一个「点开主窗口」的单向通知。
// 宠物渲染进程没有任何 fetch/Node 能力，数据全部由主进程推过来。
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
  showMain: () => ipcRenderer.send('pet:show-main'),
}));
