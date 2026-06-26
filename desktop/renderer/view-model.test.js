const test = require('node:test');
const assert = require('node:assert/strict');
const { filterTasks, allowedActions } = require('./view-model');

test('filterTasks matches title text and status', () => {
  const tasks = [
    { id: '1', title: 'Draft daemon API', description: '', status: 'todo', priority: 'high', tags: [] },
    { id: '2', title: 'Review tray UX', description: '', status: 'done', priority: 'medium', tags: [] },
  ];

  const result = filterTasks(tasks, { query: 'draft', status: 'todo', priority: 'all' });
  assert.equal(result.length, 1);
  assert.equal(result[0].id, '1');
});

test('allowedActions returns workflow actions for in_progress', () => {
  assert.deepEqual(allowedActions('in_progress'), ['review', 'block']);
});
