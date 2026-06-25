# Electron Task Board Design

## Goal

Add a simple Linux desktop frontend to `learning-go-daemon` without moving core state ownership out of Go.

The result should feel like a small local issue board:

- Go remains the single source of truth for daemon lifecycle and task data
- Electron provides a tray entry and a desktop dashboard
- tasks are created and managed from the UI as structured work items
- the first version stays local-only and single-user

## Product Direction

The desktop app is not a visual shell around arbitrary command execution.

It is a local task board with a daemon behind it:

- tasks are issue-like records, not free-form shell commands
- task progression is manual
- Electron can start and stop the daemon
- Electron does not auto-start at login in the first version

This design intentionally shifts the project from a single current task runner to a local task service with a desktop client.

## Confirmed Scope

The first version is limited to:

- Linux support only
- Electron app with a tray icon and one main dashboard window
- manual launch of the Electron app
- Electron-managed daemon start and stop
- issue-like task creation from the desktop UI
- list-plus-detail dashboard layout
- file-backed local task persistence
- local HTTP API bound to loopback

The first version does not include:

- macOS or Windows support
- login-time auto-start
- remote access
- multi-user auth
- command queues
- drag-and-drop kanban
- WebSocket or SSE push updates
- comments, attachments, or sub-tasks

## Recommended Approach

Three approaches were considered:

1. Keep Go as the backend and add Electron as a thin desktop client
2. Move task ownership into Electron and reduce Go to a helper
3. Split the backend into a more general local service before adding UI

The recommended approach is option 1.

It fits the current repository best because the daemon already owns runtime lifecycle, local state, and health checks. The desktop app should consume that backend rather than replace it.

## User Experience

The intended user flow is:

1. open the Electron app manually
2. Electron checks whether the local daemon is running
3. if not running, Electron starts it
4. a tray icon appears with quick lifecycle actions
5. the user opens the dashboard window
6. the user creates, browses, and updates tasks in the dashboard

The tray is for quick control. The dashboard is for all task management.

## Desktop Layout

The Electron app has two surfaces:

### Tray

The tray menu should expose only the smallest useful control surface:

- `Open Dashboard`
- daemon status summary
- `Start Daemon`
- `Stop Daemon`
- `Quit App`

The tray should not host full task editing or queue inspection.

### Dashboard Window

The main window uses a `list + detail` layout:

- left pane: searchable and filterable task list
- right pane: selected task details and state transition actions
- top bar: `New Task`, search, status filter, priority filter

This layout is preferred over a kanban-first layout because it is simpler to implement while still matching the issue-style task model.

## Task Model

The daemon should evolve from a single `task-state.json` record to a collection of tasks stored as structured records.

Each task includes:

- `id`
- `title`
- `description`
- `status`
- `priority`
- `tags`
- `created_at`
- `updated_at`

Example shape:

```json
{
  "id": "task_001",
  "title": "Draft daemon API",
  "description": "Define local API for the Electron dashboard",
  "status": "in_progress",
  "priority": "high",
  "tags": ["backend", "api"],
  "created_at": "2026-06-25T10:00:00Z",
  "updated_at": "2026-06-25T11:15:00Z"
}
```

The first version does not include assignees, comments, activity feeds, or task ordering metadata.

## Workflow Model

The task workflow is a simplified kanban flow with one side branch:

- `todo`
- `in_progress`
- `needs_review`
- `blocked`
- `done`

Allowed transitions:

- `todo -> in_progress`
- `in_progress -> needs_review`
- `in_progress -> blocked`
- `needs_review -> in_progress`
- `needs_review -> done`
- `blocked -> todo`
- `blocked -> in_progress`

The backend must enforce these transitions. The frontend should not be trusted as the source of workflow validity.

## Backend Architecture

The Go daemon should be reorganized into three clear layers:

- `storage`
  - loads and saves task collections and runtime metadata
- `service`
  - owns task creation, lookup, update, and transition rules
- `api`
  - exposes local HTTP handlers for Electron

Electron should never read or write backend files directly. Go remains the only writer of local state.

## State Files

The current single-task state model should be replaced or supplemented by explicit backend files:

- `tasks.json`
  - persistent task collection
- `runtime.json`
  - daemon runtime metadata such as uptime, last error, or backend version

Existing PID, address, and log files can remain for the first version. They already solve process lifecycle concerns and do not need to be redesigned immediately.

## API Boundary

The local API should stay small and action-oriented.

Recommended endpoints:

- `GET /health`
- `GET /runtime`
- `GET /tasks`
- `POST /tasks`
- `GET /tasks/:id`
- `PATCH /tasks/:id`
- `POST /tasks/:id/start`
- `POST /tasks/:id/block`
- `POST /tasks/:id/review`
- `POST /tasks/:id/reopen`
- `POST /tasks/:id/complete`

The split between `PATCH` and action endpoints is intentional:

- `PATCH` updates editable fields such as title, description, priority, and tags
- action endpoints perform validated workflow transitions

This keeps backend rules explicit and avoids ambiguous generic status updates.

## Dashboard Behavior

### Task List

Each row in the left pane should show:

- title
- status
- priority
- tags
- relative update time

The list should support:

- free-text search over title and description
- status filter
- priority filter

### Task Detail

The right pane should show:

- title and description
- status
- priority
- tags
- created and updated timestamps
- transition actions allowed by the current state

### New Task

Task creation should use a simple modal, not a separate page.

The creation form contains:

- title
- description
- priority
- tags

New tasks start in `todo`.

## Daemon Control Model

Electron is responsible for starting and stopping the daemon while the app is open.

On app launch:

1. Electron checks daemon health
2. if the daemon is not reachable, Electron attempts to start it
3. if startup succeeds, the UI continues normally
4. if startup fails, the UI shows a clear error state with retry

Because the user chose not to auto-start the desktop app at login, no background resident behavior is required when Electron is closed.

## Data Flow

The first version should use polling, not push.

Recommended behavior:

- fetch `GET /runtime` and `GET /tasks` on window open
- refresh every 2 to 5 seconds while the dashboard is visible
- refresh after any mutation action completes

Polling is sufficient for a single-user local desktop app and avoids early complexity from WebSocket or SSE support.

## Error Handling

The design should explicitly handle these cases:

### Daemon Not Running

- Electron shows a temporary startup state
- if startup fails, the dashboard shows a retryable error
- tray status reflects `stopped` or `error`

### API Unreachable After Startup

- the dashboard shows a disconnected banner
- mutation controls are disabled until connectivity returns
- tray status updates accordingly

### Corrupt Local Data

- the backend returns a clear error response
- the frontend shows a read-only error state
- the backend must not silently wipe task data

### Invalid State Transition

- the backend rejects the action with a client-visible error
- the frontend surfaces the message and refreshes task state

### Invalid Task Input

- frontend performs basic required-field checks
- backend performs final validation and returns clear error messages

## Testing Strategy

The first implementation should validate behavior at three levels.

### Go Backend Tests

- create task persists expected fields
- valid transitions succeed
- invalid transitions fail
- task collection survives reload from disk
- API handlers return expected status codes and payloads

### Electron Integration Tests

- app detects a running backend
- app starts backend when missing
- tray menu exposes expected lifecycle actions
- dashboard loads task list and detail view
- task creation form submits successfully

### Manual Linux Checks

- Electron launches correctly on a Linux desktop session
- tray icon appears and updates status
- opening the dashboard works reliably
- stopping the daemon updates the UI cleanly

## Implementation Order

Implementation should proceed in this order:

1. replace single-task backend state with task collection storage
2. add backend service layer and transition validation
3. add local HTTP task API
4. add Electron shell with daemon lifecycle integration
5. build dashboard list, detail, and create-task flow
6. add tray status and quick actions

This order keeps the backend authoritative before the desktop client depends on it.

## Milestone Definition

The first milestone is complete when all of the following are true:

- Electron can start and stop the local daemon
- the tray shows daemon status and basic actions
- the dashboard opens from the tray
- the user can create tasks from the UI
- the user can browse tasks in a list and inspect one task in detail
- the user can move tasks through the approved workflow
- all task data persists locally through daemon restarts

At that point the project has a coherent Linux desktop frontend backed by a local Go task service, without taking on multi-user or remote-system complexity.
