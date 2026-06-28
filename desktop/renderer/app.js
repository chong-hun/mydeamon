const fallbackViewModel = {
  filterTasks(tasks) {
    return Array.isArray(tasks) ? tasks : [];
  },
  allowedActions() {
    return [];
  },
};

const rendererViewModel = window.viewModel || fallbackViewModel;
const filterTasksFromModel = rendererViewModel.filterTasks.bind(rendererViewModel);
const allowedActionsFromModel = rendererViewModel.allowedActions.bind(rendererViewModel);
let rendererBooted = false;

const state = {
  tasks: [],
  selectedTaskID: null,
  filters: { query: '', status: 'all', priority: 'all' },
  daemonStatus: { status: 'checking', address: null },
  busy: false,
  error: '',
  notice: '',
};

function escapeHTML(value) {
  return String(value || '')
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;');
}

function formatDate(value) {
  if (!value) {
    return 'Unknown';
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return String(value);
  }

  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date);
}

function setBusy(nextBusy) {
  state.busy = nextBusy;
  const buttons = document.querySelectorAll('button');
  for (const button of buttons) {
    if (
      button.id === 'new-task-button' ||
      button.id === 'create-cancel' ||
      button.id === 'create-dismiss'
    ) {
      continue;
    }
    button.disabled = nextBusy;
  }
}

function setNotice(message) {
  state.notice = message;
  if (message) {
    state.error = '';
  }
}

function setError(message) {
  state.error = message;
  if (message) {
    state.notice = '';
  }
}

async function callDesktop(fn, args, successMessage) {
  setBusy(true);
  setError('');
  try {
    const result = await fn(...args);
    if (successMessage) {
      setNotice(successMessage);
    }
    return result;
  } catch (error) {
    setError(error && error.message ? error.message : 'Request failed');
    throw error;
  } finally {
    setBusy(false);
    render();
  }
}

async function loadDaemonStatus() {
  try {
    state.daemonStatus = await window.desktopAPI.daemonStatus();
  } catch (error) {
    state.daemonStatus = { status: 'error', address: null };
    setError(error && error.message ? error.message : 'Unable to read daemon status');
  }
}

function selectFirstVisible(items) {
  if (items.length === 0) {
    state.selectedTaskID = null;
    return;
  }

  const exists = items.some((task) => task.id === state.selectedTaskID);
  if (!exists) {
    state.selectedTaskID = items[0].id;
  }
}

async function refreshTasks() {
  const response = await callDesktop(window.desktopAPI.loadTasks, [], '');
  state.tasks = Array.isArray(response && response.tasks) ? response.tasks : [];
  const visibleItems = filterTasksFromModel(state.tasks, state.filters);
  selectFirstVisible(visibleItems);
  render();
}

async function refreshAll() {
  setNotice('');
  await loadDaemonStatus();
  await refreshTasks();
}

function renderStatus() {
  const indicator = document.getElementById('daemon-indicator');
  const status = state.daemonStatus.status || 'unknown';
  indicator.textContent =
    status === 'running'
      ? `Daemon running${state.daemonStatus.address ? ` at ${state.daemonStatus.address}` : ''}`
      : status === 'stopped'
        ? 'Daemon stopped'
        : status === 'error'
          ? 'Daemon status error'
          : 'Checking daemon';

  indicator.className = `status-pill ${
    status === 'running'
      ? 'status-running'
      : status === 'stopped'
        ? 'status-stopped'
        : status === 'error'
          ? 'status-error'
          : 'status-idle'
  }`;
}

function renderList(items) {
  const list = document.getElementById('task-list');
  const count = document.getElementById('task-count');
  count.textContent = `${items.length} item${items.length === 1 ? '' : 's'}`;

  if (items.length === 0) {
    list.innerHTML = `
      <div class="empty-state">
        <div>
          <h2>No matching tasks</h2>
          <p>Adjust the filters or create a new task.</p>
        </div>
      </div>
    `;
    return;
  }

  list.innerHTML = items
    .map((task) => {
      const selectedClass = task.id === state.selectedTaskID ? ' is-selected' : '';
      const tags = Array.isArray(task.tags) ? task.tags : [];
      return `
        <button class="task-row${selectedClass}" data-task-id="${escapeHTML(task.id)}" type="button">
          <strong>${escapeHTML(task.title)}</strong>
          <div class="task-meta">
            <span>${escapeHTML(task.status)}</span>
            <span>${escapeHTML(task.priority)}</span>
            <span>Updated ${escapeHTML(formatDate(task.updated_at || task.updatedAt))}</span>
          </div>
          <div class="task-row-tags">
            ${tags.length > 0 ? tags.map((tag) => `<span class="task-chip">${escapeHTML(tag)}</span>`).join('') : '<span class="task-chip">No tags</span>'}
          </div>
        </button>
      `;
    })
    .join('');

  for (const button of list.querySelectorAll('[data-task-id]')) {
    button.addEventListener('click', () => {
      state.selectedTaskID = button.dataset.taskId;
      setNotice('');
      render();
    });
  }
}

function renderDetail(task) {
  const panel = document.getElementById('task-detail');

  if (!task) {
    panel.innerHTML = `
      <div class="empty-state">
        <div>
          <h2>Select a task</h2>
          <p>The right pane shows editable fields and workflow actions for the current task.</p>
        </div>
      </div>
    `;
    return;
  }

  const tagsValue = Array.isArray(task.tags) ? task.tags.join(', ') : '';
  const actions = allowedActionsFromModel(task.status);
  const createdAt = task.created_at || task.createdAt;
  const updatedAt = task.updated_at || task.updatedAt;

  panel.innerHTML = `
    <div class="detail-shell">
      <div class="detail-header">
        <div>
          <h2>${escapeHTML(task.title)}</h2>
          <p class="detail-subtitle">Task ${escapeHTML(task.id)}</p>
        </div>
        <div class="action-strip">
          ${actions.map((action) => `<button class="action-button" data-action="${escapeHTML(action)}" type="button">${escapeHTML(action)}</button>`).join('')}
        </div>
      </div>

      ${state.error ? `<p class="message message-error">${escapeHTML(state.error)}</p>` : ''}
      ${state.notice ? `<p class="message message-success">${escapeHTML(state.notice)}</p>` : ''}

      <div class="detail-grid">
        <div class="detail-card">
          <h3>Edit task</h3>
          <form id="detail-form" class="detail-form">
            <div class="field-group">
              <label for="detail-title">Title</label>
              <input id="detail-title" name="title" value="${escapeHTML(task.title)}" required maxlength="120">
            </div>
            <div class="field-group">
              <label for="detail-description">Description</label>
              <textarea id="detail-description" name="description" rows="8">${escapeHTML(task.description || '')}</textarea>
            </div>
            <div class="inline-fields">
              <div class="field-group">
                <label for="detail-priority">Priority</label>
                <select id="detail-priority" name="priority">
                  <option value="low"${task.priority === 'low' ? ' selected' : ''}>Low</option>
                  <option value="medium"${task.priority === 'medium' ? ' selected' : ''}>Medium</option>
                  <option value="high"${task.priority === 'high' ? ' selected' : ''}>High</option>
                </select>
              </div>
              <div class="field-group">
                <label for="detail-tags">Tags</label>
                <input id="detail-tags" name="tags" value="${escapeHTML(tagsValue)}" placeholder="backend, api">
                <span class="field-help">Comma-separated tags</span>
              </div>
            </div>
            <div class="detail-actions">
              <button type="submit">Save changes</button>
            </div>
          </form>
        </div>

        <div class="detail-card">
          <h3>Task details</h3>
          <p><strong>Status:</strong> ${escapeHTML(task.status)}</p>
          <p><strong>Priority:</strong> ${escapeHTML(task.priority)}</p>
          <p><strong>Created:</strong> ${escapeHTML(formatDate(createdAt))}</p>
          <p><strong>Updated:</strong> ${escapeHTML(formatDate(updatedAt))}</p>
          <p class="hint">Workflow actions are based on the current status and refresh the task list after each transition.</p>
        </div>
      </div>
    </div>
  `;

  for (const button of panel.querySelectorAll('[data-action]')) {
    button.addEventListener('click', async () => {
      await callDesktop(window.desktopAPI.transitionTask, [task.id, button.dataset.action], `Task moved via ${button.dataset.action}`);
      await refreshAll();
    });
  }

  const form = document.getElementById('detail-form');
  form.addEventListener('submit', async (event) => {
    event.preventDefault();
    const formData = new FormData(form);
    await callDesktop(
      window.desktopAPI.updateTask,
      [
        task.id,
        {
          title: String(formData.get('title') || '').trim(),
          description: String(formData.get('description') || '').trim(),
          priority: String(formData.get('priority') || 'medium'),
          tags: String(formData.get('tags') || '')
            .split(',')
            .map((item) => item.trim())
            .filter(Boolean),
        },
      ],
      'Task updated'
    );
    await refreshAll();
  });
}

function render() {
  renderStatus();
  const items = filterTasksFromModel(state.tasks, state.filters);
  selectFirstVisible(items);
  renderList(items);
  const selected = state.tasks.find((task) => task.id === state.selectedTaskID) || null;
  renderDetail(selected);
}

function bindFilters() {
  document.getElementById('search').addEventListener('input', (event) => {
    state.filters.query = event.target.value;
    setNotice('');
    render();
  });

  document.getElementById('status-filter').addEventListener('change', (event) => {
    state.filters.status = event.target.value;
    setNotice('');
    render();
  });

  document.getElementById('priority-filter').addEventListener('change', (event) => {
    state.filters.priority = event.target.value;
    setNotice('');
    render();
  });
}

function bindRefresh() {
  document.getElementById('refresh-button').addEventListener('click', async () => {
    await refreshAll();
  });
}

function bindCreateDialog() {
  const panel = document.getElementById('create-panel');
  const form = document.getElementById('create-form');
  const titleInput = document.getElementById('create-title');

  function openDialog() {
    panel.hidden = false;
    document.body.classList.add('dialog-fallback-open');
    if (titleInput && typeof titleInput.focus === 'function') {
      titleInput.focus();
    }
  }

  function closeDialog() {
    panel.hidden = true;
    document.body.classList.remove('dialog-fallback-open');
    form.reset();
  }

  document.getElementById('new-task-button').addEventListener('click', () => {
    setError('');
    setNotice('');
    openDialog();
  });

  document.getElementById('create-cancel').addEventListener('click', closeDialog);
  document.getElementById('create-dismiss').addEventListener('click', closeDialog);

  form.addEventListener('submit', async (event) => {
    event.preventDefault();
    const formData = new FormData(form);
    await callDesktop(
      window.desktopAPI.createTask,
      [
        {
          title: String(formData.get('title') || '').trim(),
          description: String(formData.get('description') || '').trim(),
          priority: String(formData.get('priority') || 'medium'),
          tags: String(formData.get('tags') || '')
            .split(',')
            .map((item) => item.trim())
            .filter(Boolean),
        },
      ],
      'Task created'
    );
    closeDialog();
    await refreshAll();
  });
}

async function bootRenderer() {
  if (rendererBooted) {
    return;
  }
  rendererBooted = true;
  bindFilters();
  bindRefresh();
  bindCreateDialog();
  render();
  await refreshAll();
}

window.addEventListener('DOMContentLoaded', () => {
  void bootRenderer();
});

void bootRenderer();
