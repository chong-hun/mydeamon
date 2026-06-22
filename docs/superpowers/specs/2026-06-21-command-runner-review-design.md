# Command Runner With Human Review Design

## Goal

Extend the existing daemon skeleton so it can execute a configured command, persist execution state, and stop automatic progression when human review is required.

This phase is not a task queue yet. It is a single-work-item runner with explicit operator control.

## Scope

This design adds:

- configured command execution instead of log-only ticker work
- a single current work item with persisted state
- explicit runner states
- a human-review state
- `approve`, `reject`, and `resume` control commands
- persisted execution results for operator inspection

This design does not add:

- SQLite or a multi-task queue
- automatic planning of next commands
- multiple concurrent tasks
- remote APIs
- shell pipelines or arbitrary shell parsing in the first version

## Why This Shape

The current project already has a working daemon shell:

- foreground/background lifecycle
- health endpoint
- PID and address files
- status and stop commands
- periodic ticker loop

The next useful learning step is not "run a command every tick forever". That would create exactly the failure amplification problem the user called out: a bad result would keep triggering more bad runs.

The next step should instead look like a minimal task runner:

- one unit of work
- one explicit state
- one transition at a time
- operator control at decision boundaries

## Recommended Model

Use a **single-work-item state machine**.

The daemon owns exactly one current work item. On each tick it only acts if the current state allows progress.

This is the smallest design that supports:

- successful automatic execution
- halting on failure
- halting for human inspection
- resuming only through explicit operator intent

## Work Item Model

The work item is a persisted document containing:

- command executable
- command arguments
- current state
- last start time
- last finish time
- last exit code
- last stdout
- last stderr
- last error summary
- last review action

This lives in one local state file, for example:

- `~/.mydaemon/task-state.json`

This file is distinct from:

- `~/.mydaemon/mydaemon.pid`
- `~/.mydaemon/mydaemon.log`
- `~/.mydaemon/mydaemon.addr`

## State Machine

The work item uses these states:

- `idle`
- `running`
- `needs_review`
- `blocked`
- `completed`

### Meaning

- `idle`
  - the command is eligible to run on the next tick
- `running`
  - the daemon is currently executing the command
- `needs_review`
  - the command finished but explicitly requires a human decision before more progress
- `blocked`
  - execution failed or the operator rejected the result
- `completed`
  - the command reached a terminal successful result for this work item

### Tick Behavior

Each ticker cycle behaves as follows:

- if state is `idle`, start one execution
- if state is `running`, do nothing
- if state is `needs_review`, do nothing
- if state is `blocked`, do nothing
- if state is `completed`, do nothing

This prevents blind repeated execution.

## Command Configuration

The first version should use:

- executable path or command name
- argument array

This avoids shell quoting, pipes, redirection, and shell injection complexity.

Example shape:

```json
{
  "command": "date",
  "args": ["+%F %T"]
}
```

The command source can be a simple local config file or fields inside `task-state.json`. The important part is that execution uses `exec.CommandContext(command, args...)`, not shell evaluation.

## Result Contract

The executed command communicates disposition through exit codes:

- `0` = success
- `10` = needs human review
- any other non-zero code = failure

The daemon records:

- exit code
- stdout
- stderr
- finish timestamp

and transitions state as follows:

- `0` -> `completed`
- `10` -> `needs_review`
- non-zero other -> `blocked`

This gives a clear operator contract without needing a richer protocol yet.

## Human Control Commands

The CLI adds three control commands:

- `approve`
- `reject`
- `resume`

### Semantics

- `approve`
  - valid only from `needs_review`
  - marks the last review action as approved
  - transitions to `completed`
- `reject`
  - valid only from `needs_review`
  - marks the last review action as rejected
  - transitions to `blocked`
- `resume`
  - valid only from `blocked`
  - clears the blocked condition
  - transitions to `idle`

The first version should not guess what the human intended. The operator must choose explicitly.

## Execution Rules

The daemon should execute one command at a time for the current work item.

Execution flow:

1. read task state
2. if state is not `idle`, return
3. set state to `running`
4. execute command with context
5. capture stdout, stderr, exit code, and timestamps
6. persist result
7. transition to `completed`, `needs_review`, or `blocked`

If the daemon crashes mid-run, the next startup should treat `running` conservatively. The simplest rule for this phase:

- on startup, if persisted state is `running`, rewrite it to `blocked`

That mirrors the broader daemon principle already used in Multica: uncertain in-flight work should not silently continue as if it succeeded.

## Logging

Daemon logs should still record activity to `~/.mydaemon/mydaemon.log`, including:

- command start
- command end
- exit code
- state transition

The state file is the source of structured truth. The log is operational history.

## Error Handling

Failure classes:

- command cannot be spawned
- command exits with failure
- command requests human review
- state file cannot be read or written

Rules:

- state persistence failure is fatal to that tick and should be logged loudly
- command spawn failure transitions to `blocked`
- command non-zero failure transitions to `blocked`
- review exit code transitions to `needs_review`

## Testing Strategy

The first implementation should add tests for:

1. `idle` work item executes and transitions to `completed` on exit code `0`
2. exit code `10` transitions to `needs_review`
3. non-zero failure transitions to `blocked`
4. `approve` transitions `needs_review` -> `completed`
5. `reject` transitions `needs_review` -> `blocked`
6. `resume` transitions `blocked` -> `idle`
7. ticker does not execute when state is `needs_review`
8. ticker does not execute when state is `blocked`
9. startup rewrites persisted `running` -> `blocked`

Use a fake command execution layer or injectable executor in tests so the state machine can be tested deterministically.

## File Structure Changes

Add or modify a minimal set of files:

- `internal/task/runner.go`
  - move from log-only tick action to command execution orchestration
- `internal/task/state.go`
  - persisted work-item schema and load/save helpers
- `internal/task/executor.go`
  - small command execution abstraction
- `cmd/mydaemon/main.go`
  - add `approve`, `reject`, `resume`
- `internal/app/app.go`
  - startup recovery for persisted `running`

The exact file split can be adjusted during planning, but the boundaries should stay:

- app lifecycle in `internal/app`
- work-item state and transitions in `internal/task`
- filesystem helpers in `internal/state` only when they are generic enough to belong there

## Non-Goals

This phase intentionally does not solve:

- multiple tasks
- task dependencies
- automatic next-command selection
- human review UI
- remote dispatch
- resumable subprocess continuation

Those belong to later phases once this state-machine runner is stable.

## Success Criteria

This phase is successful when:

- the daemon no longer blindly executes a command every tick
- command results drive explicit persisted state transitions
- `needs_review` halts automatic progression
- `approve`, `reject`, and `resume` work predictably
- restart behavior is conservative for interrupted `running` work

At that point the project graduates from a lifecycle skeleton into a real minimal local runner with operator gates.
