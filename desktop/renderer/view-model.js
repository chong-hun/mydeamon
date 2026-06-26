function filterTasks(tasks, filters) {
  const query = String(filters.query || '').trim().toLowerCase();

  return tasks.filter((task) => {
    const title = String(task.title || '').toLowerCase();
    const description = String(task.description || '').toLowerCase();
    const matchesQuery = query === '' || title.includes(query) || description.includes(query);
    const matchesStatus = filters.status === 'all' || task.status === filters.status;
    const matchesPriority = filters.priority === 'all' || task.priority === filters.priority;
    return matchesQuery && matchesStatus && matchesPriority;
  });
}

function allowedActions(status) {
  switch (status) {
    case 'todo':
      return ['start'];
    case 'in_progress':
      return ['review', 'block'];
    case 'needs_review':
      return ['reopen', 'complete'];
    case 'blocked':
      return ['resume', 'todo'];
    default:
      return [];
  }
}

const api = { filterTasks, allowedActions };

if (typeof module !== 'undefined') {
  module.exports = api;
}

if (typeof window !== 'undefined') {
  window.viewModel = api;
}
