const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

class FakeElement {
  constructor(id, options = {}) {
    this.id = id;
    this.tagName = (options.tagName || 'div').toUpperCase();
    this.listeners = new Map();
    this.dataset = options.dataset || {};
    this.value = options.value || '';
    this.disabled = false;
    this.className = '';
    this.textContent = '';
    this.innerHTML = '';
    this.open = false;
    this.attributes = new Map();
  }

  addEventListener(type, handler) {
    const handlers = this.listeners.get(type) || [];
    handlers.push(handler);
    this.listeners.set(type, handlers);
  }

  async dispatch(type, event = {}) {
    const handlers = this.listeners.get(type) || [];
    for (const handler of handlers) {
      await handler({
        preventDefault() {},
        target: this,
        currentTarget: this,
        ...event,
      });
    }
  }

  click() {
    const handlers = this.listeners.get('click') || [];
    for (const handler of handlers) {
      handler({
        preventDefault() {},
        target: this,
        currentTarget: this,
      });
    }
  }

  querySelectorAll(selector) {
    if (selector === '[data-task-id]' || selector === '[data-action]') {
      return [];
    }
    return [];
  }

  reset() {
    this.wasReset = true;
  }

  setAttribute(name, value) {
    this.attributes.set(name, value);
    if (name === 'open') {
      this.open = true;
    }
  }

  removeAttribute(name) {
    this.attributes.delete(name);
    if (name === 'open') {
      this.open = false;
    }
  }

  close() {
    this.open = false;
  }
}

function createEnvironment(options = {}) {
  const elements = {
    daemonIndicator: new FakeElement('daemon-indicator'),
    refreshButton: new FakeElement('refresh-button', { tagName: 'button' }),
    search: new FakeElement('search', { tagName: 'input' }),
    statusFilter: new FakeElement('status-filter', { tagName: 'select', value: 'all' }),
    priorityFilter: new FakeElement('priority-filter', { tagName: 'select', value: 'all' }),
    newTaskButton: new FakeElement('new-task-button', { tagName: 'button' }),
    taskList: new FakeElement('task-list'),
    taskCount: new FakeElement('task-count'),
    taskDetail: new FakeElement('task-detail'),
    createPanel: new FakeElement('create-panel', { tagName: 'section' }),
    createForm: new FakeElement('create-form', { tagName: 'form' }),
    createTitle: new FakeElement('create-title', { tagName: 'input' }),
    createCancel: new FakeElement('create-cancel', { tagName: 'button' }),
    createDismiss: new FakeElement('create-dismiss', { tagName: 'button' }),
  };
  elements.createPanel.hidden = true;
  elements.createTitle.focus = () => {
    elements.createTitle.focused = true;
  };

  const elementByID = {
    'daemon-indicator': elements.daemonIndicator,
    'refresh-button': elements.refreshButton,
    search: elements.search,
    'status-filter': elements.statusFilter,
    'priority-filter': elements.priorityFilter,
    'new-task-button': elements.newTaskButton,
    'task-list': elements.taskList,
    'task-count': elements.taskCount,
    'task-detail': elements.taskDetail,
    'create-panel': elements.createPanel,
    'create-form': elements.createForm,
    'create-title': elements.createTitle,
    'create-cancel': elements.createCancel,
    'create-dismiss': elements.createDismiss,
  };

  const document = {
    readyState: options.readyState || 'loading',
    body: {
      classList: {
        classes: new Set(),
        add(name) {
          this.classes.add(name);
        },
        remove(name) {
          this.classes.delete(name);
        },
        contains(name) {
          return this.classes.has(name);
        },
      },
    },
    getElementById(id) {
      return elementByID[id];
    },
    querySelectorAll(selector) {
      if (selector === 'button') {
        return [
          elements.refreshButton,
          elements.newTaskButton,
          elements.createCancel,
          elements.createDismiss,
        ];
      }
      return [];
    },
  };

  const window = {
    document,
    desktopAPI: {
      async daemonStatus() {
        return { status: 'running', address: '127.0.0.1:19514' };
      },
      async loadTasks() {
        return { tasks: [] };
      },
      async createTask() {
        return {};
      },
      async updateTask() {
        return {};
      },
      async transitionTask() {
        return {};
      },
    },
    listeners: new Map(),
    addEventListener(type, handler) {
      const handlers = this.listeners.get(type) || [];
      handlers.push(handler);
      this.listeners.set(type, handlers);
    },
    async dispatch(type) {
      const handlers = this.listeners.get(type) || [];
      for (const handler of handlers) {
        await handler();
      }
    },
  };

  if (!options.omitViewModel) {
    window.viewModel = {
      filterTasks(tasks) {
        return tasks;
      },
      allowedActions() {
        return [];
      },
    };
  }

  const context = vm.createContext({
    window,
    document,
    console,
    Intl,
    Date,
    String,
    FormData: class {},
    setTimeout,
    clearTimeout,
  });

  return { context, elements, window };
}

function loadRendererApp(context) {
  const scriptPath = path.join(__dirname, 'app.js');
  const source = fs.readFileSync(scriptPath, 'utf8');
  vm.runInContext(source, context, { filename: scriptPath });
}

test('new task button opens the create panel', async () => {
  const environment = createEnvironment();
  loadRendererApp(environment.context);

  assert.equal(environment.elements.createPanel.hidden, true);
  assert.doesNotThrow(() => {
    environment.elements.newTaskButton.click();
  });
  assert.equal(environment.elements.createPanel.hidden, false);
  assert.equal(environment.elements.createTitle.focused, true);
  assert.equal(environment.context.document.body.classList.contains('dialog-fallback-open'), true);
});

test('renderer still boots when viewModel is missing', async () => {
  const environment = createEnvironment({ omitViewModel: true });

  assert.doesNotThrow(() => {
    loadRendererApp(environment.context);
  });
  assert.equal(environment.elements.newTaskButton.listeners.has('click'), true);
});

test('renderer boots immediately when document is already ready', async () => {
  const environment = createEnvironment({ readyState: 'complete' });

  loadRendererApp(environment.context);
  await Promise.resolve();

  assert.equal(environment.elements.newTaskButton.listeners.has('click'), true);
});

test('renderer boots immediately even before DOMContentLoaded fires', async () => {
  const environment = createEnvironment({ readyState: 'loading' });

  loadRendererApp(environment.context);
  await Promise.resolve();

  assert.equal(environment.elements.newTaskButton.listeners.has('click'), true);
});

test('renderer app avoids redeclaring global filterTasks and allowedActions names', () => {
  const source = fs.readFileSync(path.join(__dirname, 'app.js'), 'utf8');

  assert.equal(source.includes('const { filterTasks, allowedActions } = rendererViewModel;'), false);
});
