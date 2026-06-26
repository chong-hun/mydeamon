const test = require('node:test');
const assert = require('node:assert/strict');
const { buildStartCommand, buildTaskURL } = require('./daemon-process');

test('buildStartCommand starts mydaemon in background mode', () => {
  assert.deepEqual(buildStartCommand('/tmp/mydaemon'), {
    command: '/tmp/mydaemon',
    args: ['start'],
  });
});

test('buildTaskURL points at the local daemon', () => {
  assert.equal(buildTaskURL('127.0.0.1:19514', '/tasks'), 'http://127.0.0.1:19514/tasks');
});
