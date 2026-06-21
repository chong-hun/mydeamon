# Learning Go Daemon Runner Design

## Goal

Build a small learning project that teaches the core mechanics of a local daemon runner without introducing a remote server, task queue distribution, or OS-level service integration.

The project is intentionally limited to:

- Go implementation
- Cross-platform behavior with conservative assumptions
- CLI-controlled daemon lifecycle
- Local health checks
- Periodic fixed work
- File-based local state

The first milestone is not a full agent runtime. It is a daemon skeleton that can be started, observed, and shut down cleanly.

## Scope

This learning project includes:

- `start`, `stop`, `status`, and `logs` commands
- Foreground and background execution modes
- Single-instance protection using a health port as the source of truth
- A local HTTP health server
- PID file management
- File-based logging
- A periodic ticker loop that performs one fixed action every few seconds
- Graceful shutdown through signals and an HTTP shutdown endpoint

This learning project does not include:

- Remote task claim APIs
- SQLite-backed task queues
- Concurrent task execution
- Worker pools
- Automatic restart
- OS-native service managers such as `systemd`, `launchd`, or Windows Services
- Dynamic configuration systems
- Shell command execution in the first implementation milestone

## Why This Scope

The purpose is to isolate daemon fundamentals before adding queueing or agent execution complexity. A real local agent runner usually combines several concerns at once:

- background process control
- liveness detection
- task scheduling
- subprocess management
- retry and timeout policy

Trying to learn all of them at once makes it harder to understand which part failed and why. This design focuses first on the daemon shell itself.

## Recommended Approach

Three approaches were considered:

1. CLI-controlled daemon with background mode, health endpoint, PID file, and periodic work
2. Foreground-only long-running process with no lifecycle commands
3. Foreground worker supervised by an external process manager

The recommended approach is option 1 because it exposes the mechanics that matter for learning:

- how a daemon backgrounds itself
- how `status` distinguishes live versus stale state
- how `stop` performs graceful termination
- how the process maintains a long-running work loop

## Architecture

The project is split into four areas:

- `cmd/mydaemon`
  - CLI entrypoint and command dispatch only
- `internal/app`
  - daemon lifecycle, startup, health server, shutdown coordination
- `internal/state`
  - filesystem paths, PID file management, log file management
- `internal/task`
  - periodic fixed work loop

The main architectural constraint is that command parsing must stay separate from runtime behavior. The CLI should call the application layer rather than directly manipulating files, signals, or goroutines.

## Directory Layout

```text
mydaemon/
├── cmd/
│   └── mydaemon/
│       └── main.go
├── internal/
│   ├── app/
│   │   ├── app.go
│   │   ├── start.go
│   │   ├── stop.go
│   │   ├── health.go
│   │   └── signal.go
│   ├── state/
│   │   ├── paths.go
│   │   ├── pid.go
│   │   └── log.go
│   └── task/
│       ├── runner.go
│       └── ticker.go
└── go.mod
```

## Runtime Model

When the daemon runs in the foreground, it maintains three long-lived components:

- a local HTTP health server
- a periodic task ticker loop
- a signal watcher that cancels a shared root context

The runtime flow is:

1. create a root context
2. start the health server
3. write the PID file
4. start the periodic task loop
5. start signal handling
6. block until the root context is cancelled
7. stop the ticker
8. shut down the health server
9. delete the PID file
10. flush logs and exit

This ordering keeps startup and shutdown simple and deterministic.

## Command Behavior

### `start`

- `start --foreground` runs in the current terminal
- `start` defaults to background mode
- background mode respawns the current binary with `start --foreground`
- startup checks the health port first
- if the health endpoint is alive, startup fails with an "already running" result
- if the health endpoint is dead but a PID file exists, startup treats it as stale state and replaces it

### `stop`

- first tries `POST /shutdown` on the local health server
- waits briefly and confirms that the health endpoint is gone
- if health is already unavailable but a PID file still exists, it may attempt direct process termination as a fallback
- removes stale PID state before returning

### `status`

- reports `running` if the health endpoint responds
- reports `stale pid file` if health is down but a PID file exists
- reports `stopped` otherwise

### `logs`

- reads from a fixed log file under the daemon state directory
- first implementation only needs recent log output
- streaming follow mode is optional and not required for the first milestone

## State and Paths

The daemon stores local state under a fixed directory:

- `~/.mydaemon/mydaemon.pid`
- `~/.mydaemon/mydaemon.log`

The health endpoint binds to:

- `127.0.0.1:19514`

The health endpoint, not the PID file, is the source of truth for whether the daemon is actually alive. The PID file is only auxiliary state.

## Periodic Work

The first implementation does not run user commands or process queued tasks. Instead, every five seconds the daemon performs one fixed action:

- write a structured log entry containing:
  - current timestamp
  - process ID
  - execution counter

This is enough to prove that the daemon loop is alive, repeating, and observable through logs.

Task failures in this stage must not stop the daemon. A failed iteration is logged and the next tick proceeds normally.

## Error Handling

The daemon uses two error classes:

- fatal startup/runtime infrastructure errors
- non-fatal periodic task errors

Fatal errors:

- health server cannot bind
- state directory cannot be created
- PID file cannot be written

Non-fatal errors:

- one periodic task iteration fails
- PID cleanup fails during shutdown

The rule is simple:

- infrastructure failure prevents startup or clean shutdown
- work failure is logged and isolated to that iteration

## Testing Strategy

The first round of tests should cover the smallest useful behaviors:

1. foreground start exposes a working health endpoint
2. the periodic loop writes logs repeatedly
3. `POST /shutdown` terminates the daemon cleanly
4. `status` distinguishes `running` and `stopped`
5. a dead health endpoint plus existing PID file is reported as `stale pid file`

These tests validate the daemon shell before any queueing or subprocess execution is introduced.

## Implementation Order

Implementation should proceed in this order:

1. foreground-only `start --foreground` with health endpoint and ticker loop
2. `status` using only the health endpoint
3. PID file creation and stale-state detection
4. graceful `stop` using `/shutdown`
5. background mode by respawning `start --foreground`
6. optional `logs` command improvements

This order keeps the system debuggable throughout development. Backgrounding is intentionally delayed until the foreground runtime is already correct.

## Non-Goals for the First Milestone

The following capabilities are explicitly deferred:

- shell command execution
- per-task timeout handling
- task retries
- SQLite task storage
- remote control plane integration
- multiple concurrent workers
- metrics and tracing

Those belong to later milestones after the daemon shell is stable and understood.

## Milestone Definition

The first milestone is complete when all of the following are true:

- `start --foreground` runs a long-lived process
- `start` can launch the same process in the background
- `status` reports live state correctly
- `stop` shuts the daemon down cleanly
- the daemon emits periodic logs every five seconds
- the process removes its PID file on normal shutdown

At that point the project has achieved its real learning objective: understanding how a daemon maintains lifecycle, liveness, and periodic work in a controlled process boundary.
