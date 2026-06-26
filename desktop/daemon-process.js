const { execFile } = require('node:child_process');
const path = require('node:path');
const { promisify } = require('node:util');

const execFileAsync = promisify(execFile);
const LOCAL_ADDRESS = '127.0.0.1:19514';

function buildStartCommand(binaryPath) {
  return { command: binaryPath, args: ['start'] };
}

function buildTaskURL(address, path) {
  return `http://${address}${path}`;
}

async function startDaemon(binaryPath) {
  if (binaryPath) {
    const { command, args } = buildStartCommand(binaryPath);
    await execFileAsync(command, args);
    return;
  }

  const repoRoot = path.join(__dirname, '..');
  await execFileAsync('go', ['run', './cmd/mydaemon', 'start'], {
    cwd: repoRoot,
  });
}

async function fetchJSON(address, requestPath, options = {}) {
  const response = await fetch(buildTaskURL(address, requestPath), options);
  if (!response.ok) {
    throw new Error(`request failed with status ${response.status}`);
  }

  return response.status === 204 ? null : response.json();
}

async function getDaemonStatus() {
  try {
    await fetchJSON(LOCAL_ADDRESS, '/health');
    return { status: 'running', address: LOCAL_ADDRESS };
  } catch {
    return { status: 'stopped', address: LOCAL_ADDRESS };
  }
}

async function ensureDaemonRunning(binaryPath) {
  const status = await getDaemonStatus();
  if (status.status === 'running') {
    return status;
  }

  await startDaemon(binaryPath);
  return getDaemonStatus();
}

async function listTasks() {
  return fetchJSON(LOCAL_ADDRESS, '/tasks');
}

async function createTask(payload) {
  return fetchJSON(LOCAL_ADDRESS, '/tasks', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
}

async function updateTask(id, payload) {
  return fetchJSON(LOCAL_ADDRESS, `/tasks/${id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
}

async function runTaskAction(id, action) {
  return fetchJSON(LOCAL_ADDRESS, `/tasks/${id}/${action}`, {
    method: 'POST',
  });
}

module.exports = {
  LOCAL_ADDRESS,
  buildStartCommand,
  buildTaskURL,
  startDaemon,
  fetchJSON,
  getDaemonStatus,
  ensureDaemonRunning,
  listTasks,
  createTask,
  updateTask,
  runTaskAction,
};
