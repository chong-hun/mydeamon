# learning-go-daemon

Local Go daemon backend for a Linux Electron task board.

## Commands

- `go run ./cmd/mydaemon start --foreground`
- `go run ./cmd/mydaemon status`
- `go run ./cmd/mydaemon stop`
- `go run ./cmd/mydaemon logs`

The daemon exposes local HTTP endpoints for runtime health and task management, with task state persisted in `~/.mydaemon/tasks.json`.

## Desktop Development

1. Install desktop dependencies:
   - `npm install`
2. Start the daemon manually when debugging backend-only work:
   - `go run ./cmd/mydaemon start --foreground`
3. Launch the Electron shell:
   - `npm run electron`

The Electron app will:

- check whether the daemon is reachable
- start the daemon if it is not running
- show a tray entry with dashboard and lifecycle actions
- let you create and update tasks from the dashboard
