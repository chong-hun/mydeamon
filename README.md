# learning-go-daemon

Small Go daemon learning project.

## Commands

- `go run ./cmd/mydaemon start --foreground`
- `go run ./cmd/mydaemon status`
- `go run ./cmd/mydaemon stop`
- `go run ./cmd/mydaemon logs`

## Work State

The daemon persists one current work item in `~/.mydaemon/task-state.json`.

Work item states:

- `idle`
- `running`
- `needs_review`
- `blocked`
- `completed`

Command exit code contract:

- `0` = completed
- `10` = needs review
- other non-zero = blocked

Review commands:

- `go run ./cmd/mydaemon approve`
- `go run ./cmd/mydaemon reject`
- `go run ./cmd/mydaemon resume`
