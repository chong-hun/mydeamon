# Command Runner With Human Review Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the existing Go daemon skeleton so it executes a configured command, persists a single work-item state machine, and supports `approve`, `reject`, and `resume` when human review is required.

**Architecture:** The daemon keeps its existing lifecycle shell in `internal/app`, but the periodic runner moves from log-only behavior to a persisted single-work-item state machine in `internal/task`. Command execution is abstracted behind a small executor so tests can drive state transitions deterministically without running real subprocesses.

**Tech Stack:** Go, standard library (`os/exec`, `encoding/json`, `net/http`, `testing`), existing app/state/task packages

---

## File Structure

Planned files and responsibilities:

- `internal/task/state.go`
  - work-item schema, state constants, load/save helpers
- `internal/task/state_test.go`
  - persistence and startup recovery tests
- `internal/task/executor.go`
  - executor interface and subprocess-backed implementation
- `internal/task/executor_test.go`
  - executor result mapping tests
- `internal/task/runner.go`
  - state-machine orchestration around the executor
- `internal/task/runner_test.go`
  - transition behavior tests for success, review, failure, and gating
- `internal/app/app.go`
  - startup recovery of persisted `running` state
- `cmd/mydaemon/main.go`
  - add `approve`, `reject`, and `resume` command dispatch
- `cmd/mydaemon/main_test.go`
  - parser coverage for the new control commands

## Task 1: Add persisted work-item state

**Files:**
- Create: `internal/task/state.go`
- Create: `internal/task/state_test.go`

- [ ] **Step 1: Write the failing persistence tests**

Create `internal/task/state_test.go`:

```go
package task

import (
	"path/filepath"
	"testing"
)

func TestSaveAndLoadStateRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "task-state.json")
	want := WorkState{
		Command: "date",
		Args:    []string{"+%F %T"},
		Status:  StatusIdle,
	}

	if err := SaveState(path, want); err != nil {
		t.Fatalf("SaveState returned error: %v", err)
	}

	got, ok, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected state file to exist")
	}
	if got.Command != want.Command || got.Status != want.Status {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if len(got.Args) != 1 || got.Args[0] != "+%F %T" {
		t.Fatalf("unexpected args: %#v", got.Args)
	}
}

func TestLoadStateMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "task-state.json")

	_, ok, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState returned error: %v", err)
	}
	if ok {
		t.Fatal("expected ok=false for missing file")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/task -run 'TestSaveAndLoadStateRoundTrip|TestLoadStateMissingFile' -v`
Expected: FAIL because `WorkState`, `StatusIdle`, `SaveState`, and `LoadState` are undefined

- [ ] **Step 3: Write minimal implementation**

Create `internal/task/state.go`:

```go
package task

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	StatusIdle        = "idle"
	StatusRunning     = "running"
	StatusNeedsReview = "needs_review"
	StatusBlocked     = "blocked"
	StatusCompleted   = "completed"
)

type WorkState struct {
	Command          string   `json:"command"`
	Args             []string `json:"args"`
	Status           string   `json:"status"`
	LastStartAt      string   `json:"last_start_at,omitempty"`
	LastFinishAt     string   `json:"last_finish_at,omitempty"`
	LastExitCode     int      `json:"last_exit_code,omitempty"`
	LastStdout       string   `json:"last_stdout,omitempty"`
	LastStderr       string   `json:"last_stderr,omitempty"`
	LastErrorSummary string   `json:"last_error_summary,omitempty"`
	LastReviewAction string   `json:"last_review_action,omitempty"`
}

func SaveState(path string, state WorkState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func LoadState(path string) (WorkState, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return WorkState{}, false, nil
	}
	if err != nil {
		return WorkState{}, false, err
	}
	var state WorkState
	if err := json.Unmarshal(data, &state); err != nil {
		return WorkState{}, true, err
	}
	return state, true, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/task -run 'TestSaveAndLoadStateRoundTrip|TestLoadStateMissingFile' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/task/state.go internal/task/state_test.go
git commit -m "feat: add persisted work state"
```

## Task 2: Add executor abstraction and result mapping

**Files:**
- Create: `internal/task/executor.go`
- Create: `internal/task/executor_test.go`

- [ ] **Step 1: Write the failing executor classification test**

Create `internal/task/executor_test.go`:

```go
package task

import "testing"

func TestClassifyExitCode(t *testing.T) {
	tests := []struct {
		code int
		want string
	}{
		{code: 0, want: StatusCompleted},
		{code: 10, want: StatusNeedsReview},
		{code: 1, want: StatusBlocked},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := classifyExitCode(tt.code)
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/task -run TestClassifyExitCode -v`
Expected: FAIL because `classifyExitCode` is undefined

- [ ] **Step 3: Write minimal implementation**

Create `internal/task/executor.go`:

```go
package task

import (
	"bytes"
	"context"
	"os/exec"
)

type ExecResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

type Executor interface {
	Run(context.Context, string, []string) (ExecResult, error)
}

type OSExecutor struct{}

func (OSExecutor) Run(ctx context.Context, command string, args []string) (ExecResult, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return ExecResult{}, err
		}
	}

	return ExecResult{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}, nil
}

func classifyExitCode(code int) string {
	switch code {
	case 0:
		return StatusCompleted
	case 10:
		return StatusNeedsReview
	default:
		return StatusBlocked
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/task -run TestClassifyExitCode -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/task/executor.go internal/task/executor_test.go
git commit -m "feat: add command executor abstraction"
```

## Task 3: Convert runner to a persisted state machine

**Files:**
- Modify: `internal/task/runner.go`
- Create: `internal/task/runner_test.go`

- [ ] **Step 1: Write the failing state transition tests**

Create `internal/task/runner_test.go`:

```go
package task

import (
	"context"
	"io"
	"log"
	"path/filepath"
	"testing"
)

type fakeExecutor struct {
	result ExecResult
	err    error
	calls  int
}

func (f *fakeExecutor) Run(context.Context, string, []string) (ExecResult, error) {
	f.calls++
	return f.result, f.err
}

func TestRunnerTransitionsIdleToCompletedOnExitZero(t *testing.T) {
	path := filepath.Join(t.TempDir(), "task-state.json")
	if err := SaveState(path, WorkState{Command: "date", Status: StatusIdle}); err != nil {
		t.Fatalf("SaveState returned error: %v", err)
	}

	exec := &fakeExecutor{result: ExecResult{ExitCode: 0, Stdout: "ok"}}
	runner := NewRunner(log.New(io.Discard, "", 0), path, exec)

	if err := runner.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	state, ok, err := LoadState(path)
	if err != nil || !ok {
		t.Fatalf("LoadState failed: ok=%v err=%v", ok, err)
	}
	if state.Status != StatusCompleted {
		t.Fatalf("expected completed, got %q", state.Status)
	}
}

func TestRunnerDoesNotExecuteWhenBlocked(t *testing.T) {
	path := filepath.Join(t.TempDir(), "task-state.json")
	if err := SaveState(path, WorkState{Command: "date", Status: StatusBlocked}); err != nil {
		t.Fatalf("SaveState returned error: %v", err)
	}

	exec := &fakeExecutor{}
	runner := NewRunner(log.New(io.Discard, "", 0), path, exec)

	if err := runner.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if exec.calls != 0 {
		t.Fatalf("expected executor not to run, got %d calls", exec.calls)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/task -run 'TestRunnerTransitionsIdleToCompletedOnExitZero|TestRunnerDoesNotExecuteWhenBlocked' -v`
Expected: FAIL because the existing `NewRunner` signature and `RunOnce` behavior do not support persisted state/executor orchestration

- [ ] **Step 3: Write minimal implementation**

Replace `internal/task/runner.go` with:

```go
package task

import (
	"context"
	"fmt"
	"log"
	"time"
)

type Runner struct {
	logger    *log.Logger
	statePath string
	executor  Executor
}

func NewRunner(logger *log.Logger, statePath string, executor Executor) *Runner {
	return &Runner{
		logger:    logger,
		statePath: statePath,
		executor:  executor,
	}
}

func (r *Runner) RunOnce(ctx context.Context) error {
	state, ok, err := LoadState(r.statePath)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	if state.Status != StatusIdle {
		return nil
	}

	state.Status = StatusRunning
	state.LastStartAt = time.Now().Format(time.RFC3339)
	if err := SaveState(r.statePath, state); err != nil {
		return err
	}

	r.logger.Printf("command start: %s %v", state.Command, state.Args)
	result, err := r.executor.Run(ctx, state.Command, state.Args)
	if err != nil {
		state.Status = StatusBlocked
		state.LastErrorSummary = err.Error()
		state.LastFinishAt = time.Now().Format(time.RFC3339)
		return SaveState(r.statePath, state)
	}

	state.LastExitCode = result.ExitCode
	state.LastStdout = result.Stdout
	state.LastStderr = result.Stderr
	state.LastFinishAt = time.Now().Format(time.RFC3339)
	state.Status = classifyExitCode(result.ExitCode)
	r.logger.Printf("command end: exit=%d next_state=%s", result.ExitCode, state.Status)

	if err := SaveState(r.statePath, state); err != nil {
		return err
	}
	return nil
}

func (r *Runner) Approve() error {
	return r.transitionReview(StatusNeedsReview, StatusCompleted, "approved")
}

func (r *Runner) Reject() error {
	return r.transitionReview(StatusNeedsReview, StatusBlocked, "rejected")
}

func (r *Runner) Resume() error {
	state, ok, err := LoadState(r.statePath)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("no task state found")
	}
	if state.Status != StatusBlocked {
		return fmt.Errorf("resume requires blocked state, got %s", state.Status)
	}
	state.Status = StatusIdle
	return SaveState(r.statePath, state)
}

func (r *Runner) transitionReview(from, to, action string) error {
	state, ok, err := LoadState(r.statePath)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("no task state found")
	}
	if state.Status != from {
		return fmt.Errorf("expected %s state, got %s", from, state.Status)
	}
	state.Status = to
	state.LastReviewAction = action
	return SaveState(r.statePath, state)
}
```

- [ ] **Step 4: Expand the tests for review transitions**

Append to `internal/task/runner_test.go`:

```go
func TestRunnerTransitionsIdleToNeedsReviewOnExitTen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "task-state.json")
	if err := SaveState(path, WorkState{Command: "date", Status: StatusIdle}); err != nil {
		t.Fatalf("SaveState returned error: %v", err)
	}

	exec := &fakeExecutor{result: ExecResult{ExitCode: 10}}
	runner := NewRunner(log.New(io.Discard, "", 0), path, exec)

	if err := runner.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	state, _, _ := LoadState(path)
	if state.Status != StatusNeedsReview {
		t.Fatalf("expected needs_review, got %q", state.Status)
	}
}

func TestApproveRejectResumeTransitions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "task-state.json")
	runner := NewRunner(log.New(io.Discard, "", 0), path, &fakeExecutor{})

	if err := SaveState(path, WorkState{Command: "date", Status: StatusNeedsReview}); err != nil {
		t.Fatalf("SaveState returned error: %v", err)
	}
	if err := runner.Approve(); err != nil {
		t.Fatalf("Approve returned error: %v", err)
	}
	state, _, _ := LoadState(path)
	if state.Status != StatusCompleted || state.LastReviewAction != "approved" {
		t.Fatalf("unexpected state after approve: %+v", state)
	}

	if err := SaveState(path, WorkState{Command: "date", Status: StatusNeedsReview}); err != nil {
		t.Fatalf("SaveState returned error: %v", err)
	}
	if err := runner.Reject(); err != nil {
		t.Fatalf("Reject returned error: %v", err)
	}
	state, _, _ = LoadState(path)
	if state.Status != StatusBlocked || state.LastReviewAction != "rejected" {
		t.Fatalf("unexpected state after reject: %+v", state)
	}

	if err := runner.Resume(); err != nil {
		t.Fatalf("Resume returned error: %v", err)
	}
	state, _, _ = LoadState(path)
	if state.Status != StatusIdle {
		t.Fatalf("unexpected state after resume: %+v", state)
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/task -run 'TestRunner|TestApproveRejectResumeTransitions' -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/task/runner.go internal/task/runner_test.go
git commit -m "feat: add command runner state machine"
```

## Task 4: Add startup recovery for interrupted running work

**Files:**
- Modify: `internal/app/app.go`
- Modify: `internal/task/state_test.go`

- [ ] **Step 1: Write the failing recovery test**

Append to `internal/task/state_test.go`:

```go
func TestRewriteRunningStateToBlocked(t *testing.T) {
	path := filepath.Join(t.TempDir(), "task-state.json")
	if err := SaveState(path, WorkState{
		Command: "date",
		Status:  StatusRunning,
	}); err != nil {
		t.Fatalf("SaveState returned error: %v", err)
	}

	if err := RewriteRunningStateToBlocked(path); err != nil {
		t.Fatalf("RewriteRunningStateToBlocked returned error: %v", err)
	}

	state, ok, err := LoadState(path)
	if err != nil || !ok {
		t.Fatalf("LoadState failed: ok=%v err=%v", ok, err)
	}
	if state.Status != StatusBlocked {
		t.Fatalf("expected blocked, got %q", state.Status)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/task -run TestRewriteRunningStateToBlocked -v`
Expected: FAIL because `RewriteRunningStateToBlocked` is undefined

- [ ] **Step 3: Implement the recovery helper**

Append to `internal/task/state.go`:

```go
func RewriteRunningStateToBlocked(path string) error {
	state, ok, err := LoadState(path)
	if err != nil || !ok {
		return err
	}
	if state.Status != StatusRunning {
		return nil
	}
	state.Status = StatusBlocked
	state.LastErrorSummary = "daemon restarted while command was running"
	return SaveState(path, state)
}
```

- [ ] **Step 4: Wire startup recovery in app**

Update `internal/app/app.go` imports and add before `server := newHealthServer(...)` in `RunForeground`:

```go
	if err := task.RewriteRunningStateToBlocked(taskStatePath(a.cfg.StateDir)); err != nil {
		return err
	}
```

Also add a helper near `healthAddressPath`:

```go
func taskStatePath(stateDir string) string {
	return filepath.Join(stateDir, "task-state.json")
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/task ./internal/app -run 'TestRewriteRunningStateToBlocked' -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/task/state.go internal/task/state_test.go internal/app/app.go
git commit -m "feat: recover interrupted running work on startup"
```

## Task 5: Wire the runner into the daemon tick loop

**Files:**
- Modify: `internal/app/app.go`
- Modify: `internal/task/ticker_test.go`

- [ ] **Step 1: Write the failing gating test**

Append to `internal/task/ticker_test.go`:

```go
func TestTickerLoopCanDriveRunnerOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "task-state.json")
	if err := SaveState(path, WorkState{Command: "date", Status: StatusIdle}); err != nil {
		t.Fatalf("SaveState returned error: %v", err)
	}

	exec := &fakeExecutor{result: ExecResult{ExitCode: 0}}
	runner := NewRunner(log.New(io.Discard, "", 0), path, exec)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	ticks := make(chan time.Time, 1)

	go func() {
		runTickerLoop(ctx, ticks, func(ctx context.Context) error {
			err := runner.RunOnce(ctx)
			cancel()
			return err
		})
		close(done)
	}()

	ticks <- time.Now()
	waitForSignal(t, done, "ticker loop to stop")

	if exec.calls != 1 {
		t.Fatalf("expected 1 execution, got %d", exec.calls)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/task -run TestTickerLoopCanDriveRunnerOnce -v`
Expected: FAIL until the fake executor from `runner_test.go` is available package-wide or duplicated in this file

- [ ] **Step 3: Make the test compile and pass**

Add a local test double to `internal/task/ticker_test.go` if needed:

```go
type tickerFakeExecutor struct {
	result ExecResult
	calls  int
}

func (f *tickerFakeExecutor) Run(context.Context, string, []string) (ExecResult, error) {
	f.calls++
	return f.result, nil
}
```

Then use `tickerFakeExecutor` in the new test.

- [ ] **Step 4: Wire app to use the new runner**

Update `internal/app/app.go` inside `RunForeground`:

```go
	runner := task.NewRunner(a.logger, taskStatePath(a.cfg.StateDir), task.OSExecutor{})
	go task.StartTickerLoop(ctx, a.cfg.Interval, runner.RunOnce)
```

This replaces the old `task.NewRunner(a.logger)` usage.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/task ./internal/app -run 'TestTickerLoopCanDriveRunnerOnce' -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/app/app.go internal/task/ticker_test.go
git commit -m "feat: wire command runner into daemon loop"
```

## Task 6: Add approve, reject, and resume CLI commands

**Files:**
- Modify: `cmd/mydaemon/main.go`
- Modify: `cmd/mydaemon/main_test.go`

- [ ] **Step 1: Write the failing parser test for new commands**

Append to `cmd/mydaemon/main_test.go`:

```go
func TestParseArgsAcceptsReviewCommands(t *testing.T) {
	for _, command := range []string{"approve", "reject", "resume"} {
		t.Run(command, func(t *testing.T) {
			parsed, err := parseArgs([]string{command})
			if err != nil {
				t.Fatalf("parseArgs returned error: %v", err)
			}
			if parsed.command != command {
				t.Fatalf("expected %q, got %q", command, parsed.command)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/mydaemon -run TestParseArgsAcceptsReviewCommands -v`
Expected: FAIL because parser does not accept the new commands yet

- [ ] **Step 3: Update parser and dispatch**

Modify the `switch` inside `parseArgs` in `cmd/mydaemon/main.go`:

```go
		case "start", "stop", "status", "logs", "approve", "reject", "resume":
```

In `main()`, create one daemon app plus one task runner after `cfg` is built:

```go
	taskRunner := task.NewRunner(log.New(os.Stderr, "", 0), filepath.Join(stateDir, "task-state.json"), task.OSExecutor{})
```

Add imports:

```go
	"path/filepath"
	"github.com/chenxian/learning-go-daemon/internal/task"
```

Then add new branches:

```go
	case "approve":
		if err := taskRunner.Approve(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "reject":
		if err := taskRunner.Reject(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "resume":
		if err := taskRunner.Resume(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/mydaemon -run 'TestParseArgsAcceptsReviewCommands|TestParse' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/mydaemon/main.go cmd/mydaemon/main_test.go
git commit -m "feat: add review control commands"
```

## Task 7: Document the command-runner workflow

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Update README with state-machine usage**

Replace `README.md` with:

```md
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
```

- [ ] **Step 2: Run full automated tests**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 3: Manually verify review flow**

Create a test state file:

```bash
mkdir -p ~/.mydaemon
cat > ~/.mydaemon/task-state.json <<'EOF'
{
  "command": "sh",
  "args": ["-c", "exit 10"],
  "status": "idle"
}
EOF
```

Run:

```bash
go run ./cmd/mydaemon start --foreground
```

In another terminal:

```bash
go run ./cmd/mydaemon status
cat ~/.mydaemon/task-state.json
go run ./cmd/mydaemon approve
cat ~/.mydaemon/task-state.json
```

Expected:

- daemon runs once and does not keep re-running
- state changes from `idle` -> `running` -> `needs_review`
- `approve` changes state to `completed`

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs: describe command runner review workflow"
```

## Self-Review

Spec coverage:

- configured command execution: Task 2 and Task 3
- persisted single work item: Task 1
- explicit states: Task 1 and Task 3
- human review state: Task 2 and Task 3
- approve/reject/resume commands: Task 3 and Task 6
- startup recovery of interrupted running work: Task 4
- no blind repeated execution while blocked or needs_review: Task 3 and Task 5

Placeholder scan:

- no `TODO`, `TBD`, or implicit “fill this in later” placeholders remain
- each task names exact files and exact commands
- code steps contain concrete code blocks

Type consistency:

- `WorkState`, `Executor`, `ExecResult`, `Runner`, and state constants are introduced before later tasks use them
- task-state path is consistently `task-state.json`
- review methods consistently use `Approve`, `Reject`, and `Resume`
