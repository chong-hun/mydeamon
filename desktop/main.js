const path = require('node:path');
const { app, BrowserWindow, Menu, Tray, nativeImage, ipcMain } = require('electron');
const {
  createTask,
  listTasks,
  updateTask,
  runTaskAction,
  ensureDaemonRunning,
  getDaemonStatus,
} = require('./daemon-process');

let mainWindow = null;
let tray = null;

function trayIconDataURL() {
  return 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAABAAAAAQCAQAAAC1+jfqAAAApElEQVR42mNgoBvg4uJiwA2YGBh+g4GB4T8DA8P/BgaG/zMwMPyfgYHhP4PBgf8MDAz/JyAg+E8QwMDA8J8B4n8GBob/DAwM/ycgIPgPEMDBwYGBgYHhPwMDw38GBob/MzAw/J+BgWEgkB8SExP9T0BA8B8mJiY+AwPDf4aGhv8MDAz/JyAg+E8QwMDA8J8B4n8GBob/DAwM/ycgIPgPAwMDAwPjPwYGhv8MDAz/JwAA3twTM8VnZwoAAAAASUVORK5CYII=';
}

function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1100,
    height: 760,
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
    },
  });

  mainWindow.loadFile(path.join(__dirname, 'renderer/index.html'));
}

function createTray() {
  tray = new Tray(nativeImage.createFromDataURL(trayIconDataURL()));
  tray.setContextMenu(
    Menu.buildFromTemplate([
      {
        label: 'Open Dashboard',
        click: () => {
          if (mainWindow) {
            mainWindow.show();
          }
        },
      },
      { type: 'separator' },
      { label: 'Quit App', click: () => app.quit() },
    ])
  );
}

ipcMain.handle('tasks:list', async () => listTasks());
ipcMain.handle('tasks:create', async (_event, payload) => createTask(payload));
ipcMain.handle('tasks:update', async (_event, { id, payload }) => updateTask(id, payload));
ipcMain.handle('tasks:action', async (_event, { id, action }) => runTaskAction(id, action));
ipcMain.handle('daemon:status', async () => getDaemonStatus());

app.whenReady().then(async () => {
  await ensureDaemonRunning();
  createWindow();
  createTray();
});
