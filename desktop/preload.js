const { contextBridge, ipcRenderer } = require('electron');

contextBridge.exposeInMainWorld('desktopAPI', {
  loadTasks: () => ipcRenderer.invoke('tasks:list'),
  createTask: (payload) => ipcRenderer.invoke('tasks:create', payload),
  transitionTask: (id, action) => ipcRenderer.invoke('tasks:action', { id, action }),
  updateTask: (id, payload) => ipcRenderer.invoke('tasks:update', { id, payload }),
  daemonStatus: () => ipcRenderer.invoke('daemon:status'),
});
