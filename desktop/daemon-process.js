const { execFile } = require('node:child_process');
const { promisify } = require('node:util');

const execFileAsync = promisify(execFile);

function buildStartCommand(binaryPath) {
  return { command: binaryPath, args: ['start'] };
}

function buildTaskURL(address, path) {
  return `http://${address}${path}`;
}

async function startDaemon(binaryPath) {
  const { command, args } = buildStartCommand(binaryPath);
  await execFileAsync(command, args);
}

module.exports = {
  buildStartCommand,
  buildTaskURL,
  startDaemon,
};
