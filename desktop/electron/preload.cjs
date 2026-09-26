const {contextBridge, ipcRenderer} = require('electron');
contextBridge.exposeInMainWorld('szuDesktop', Object.freeze({
  shell: 'electron',
  quit: () => ipcRenderer.invoke('szu:quit'),
  // 宠物大小：主窗是唯一入口，setPetScale 返回归一化后的真实值。
  petScale: () => ipcRenderer.invoke('szu:pet-scale-get'),
  setPetScale: (value) => ipcRenderer.invoke('szu:pet-scale-set', value),
  openSchool: (target) => ipcRenderer.invoke('szu:school-open', target),
  syncSchool: (business) => ipcRenderer.invoke('szu:school-sync', business),
  clearSchool: () => ipcRenderer.invoke('szu:school-clear'),
}));
