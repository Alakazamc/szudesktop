const {contextBridge, ipcRenderer} = require('electron');
contextBridge.exposeInMainWorld('szuDesktop', Object.freeze({
  shell: 'electron',
  quit: () => ipcRenderer.invoke('szu:quit'),
}));
