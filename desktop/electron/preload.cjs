const {contextBridge, ipcRenderer} = require('electron');
const petCommands = new Set(['pat', 'feed', 'play', 'sleep', 'garden', 'farm', 'study', 'home']);
contextBridge.exposeInMainWorld('szuDesktop', Object.freeze({
  shell: 'electron',
  quit: () => ipcRenderer.invoke('szu:quit'),
  // 大小来自主进程统一设置；滑杆、菜单和滚轮共享同一数值。
  petScale: () => ipcRenderer.invoke('szu:pet-scale-get'),
  setPetScale: (value) => ipcRenderer.invoke('szu:pet-scale-set', value),
  onPetScale: (callback) => {
    if (typeof callback !== 'function') return () => {};
    const listener = (_event, scale) => {
      if (typeof scale === 'number' && Number.isFinite(scale) && scale >= 0.4 && scale <= 2) callback(scale);
    };
    ipcRenderer.on('szu:pet-scale', listener);
    return () => ipcRenderer.removeListener('szu:pet-scale', listener);
  },
  onPetCommand: (callback) => {
    if (typeof callback !== 'function') return () => {};
    const listener = (_event, command) => {
      if (petCommands.has(command)) callback(command);
    };
    ipcRenderer.on('szu:pet-command', listener);
    return () => ipcRenderer.removeListener('szu:pet-command', listener);
  },
  petResult: (result) => {
    if (!result || typeof result.ok !== 'boolean' || typeof result.message !== 'string') return;
    ipcRenderer.send('szu:pet-result', {ok: result.ok, message: result.message.slice(0, 120)});
  },
  openSchool: (target) => ipcRenderer.invoke('szu:school-open', target),
  syncSchool: (business) => ipcRenderer.invoke('szu:school-sync', business),
  clearSchool: () => ipcRenderer.invoke('szu:school-clear'),
}));
