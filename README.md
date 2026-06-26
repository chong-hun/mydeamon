# learning-go-daemon

Local Go daemon backend for a Linux Electron task board.

## Commands

- `go run ./cmd/mydaemon start --foreground`
- `go run ./cmd/mydaemon status`
- `go run ./cmd/mydaemon stop`
- `go run ./cmd/mydaemon logs`

The daemon exposes local HTTP endpoints for runtime health and task management, with task state persisted in `~/.mydaemon/tasks.json`.
