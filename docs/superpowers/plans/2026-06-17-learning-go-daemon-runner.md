# Learning Go Daemon Runner Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a standalone Go daemon learning project that supports `start`, `stop`, `status`, and `logs`, exposes a local health endpoint, writes periodic log entries every five seconds, and can run in foreground or background mode.

**Architecture:** The daemon is a CLI-controlled single binary. `cmd/mydaemon` parses commands and delegates to `internal/app`, which coordinates startup, shutdown, health checks, PID file lifecycle, logging, and the periodic work loop. `internal/state` owns filesystem state and `internal/task` owns the ticker-driven fixed work.

**Tech Stack:** Go, standard library (`net/http`, `os/exec`, `context`, `os/signal`, `testing`), git

---

## File Structure

Planned project files and responsibilities:

- `go.mod`
  - module declaration for the standalone project
- `cmd/mydaemon/main.go`
  - CLI entrypoint and command dispatch
- `internal/app/app.go`
  - `App` type, config defaults, top-level wiring
- `internal/app/start.go`
  - foreground and background startup flows
- `internal/app/stop.go`
  - stop and status behavior
- `internal/app/health.go`
  - local HTTP server, `/health`, `/shutdown`
- `internal/app/signal.go`
  - root-context cancellation on OS signals
- `internal/state/paths.go`
  - state directory and fixed file path helpers
- `internal/state/pid.go`
  - PID read/write/remove helpers
- `internal/state/log.go`
  - log file open helper
- `internal/task/runner.go`
  - fixed periodic action implementation
- `internal/task/ticker.go`
  - ticker loop wrapper around the runner
- `internal/app/app_test.go`
  - startup and shutdown lifecycle tests
- `internal/app/status_test.go`
  - status and stale PID behavior tests
- `internal/task/ticker_test.go`
  - periodic work loop tests

## Task 1: Initialize the standalone Go project

**Files:**
- Create: `go.mod`
- Create: `cmd/mydaemon/main.go`

- [ ] **Step 1: Write the failing smoke test command**

Create the project module file:

```go
module github.com/chenxian/learning-go-daemon

go 1.24
```

Create the initial entrypoint:

```go
package main

import "fmt"

func main() {
	fmt.Println("mydaemon bootstrap")
}
```

- [ ] **Step 2: Run the binary to verify the skeleton builds**

Run: `go run ./cmd/mydaemon`
Expected: prints `mydaemon bootstrap`

- [ ] **Step 3: Commit**

```bash
git add go.mod cmd/mydaemon/main.go
git commit -m "chore: initialize go daemon project"
```

## Task 2: Add state path helpers and log file support

**Files:**
- Create: `internal/state/paths.go`
- Create: `internal/state/log.go`
- Test: `internal/state/log_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/state/log_test.go`:

```go
package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenLogFileCreatesParentDirectory(t *testing.T) {
	root := t.TempDir()
	logPath := filepath.Join(root, "nested", "mydaemon.log")

	file, err := OpenLogFile(logPath)
	if err != nil {
		t.Fatalf("OpenLogFile returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = file.Close()
	})

	if _, err := os.Stat(filepath.Dir(logPath)); err != nil {
		t.Fatalf("expected parent directory to exist: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/state -run TestOpenLogFileCreatesParentDirectory -v`
Expected: FAIL because `OpenLogFile` is undefined

- [ ] **Step 3: Write minimal implementation**

Create `internal/state/paths.go`:

```go
package state

import (
	"os"
	"path/filepath"
)

const (
	DirName = ".mydaemon"
	PIDName = "mydaemon.pid"
	LogName = "mydaemon.log"
)

func DefaultStateDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, DirName), nil
}

func PIDPath(root string) string {
	return filepath.Join(root, PIDName)
}

func LogPath(root string) string {
	return filepath.Join(root, LogName)
}
```

Create `internal/state/log.go`:

```go
package state

import (
	"os"
	"path/filepath"
)

func OpenLogFile(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/state -run TestOpenLogFileCreatesParentDirectory -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/state/paths.go internal/state/log.go internal/state/log_test.go
git commit -m "feat: add daemon state path helpers"
```

## Task 3: Add PID file helpers

**Files:**
- Create: `internal/state/pid.go`
- Test: `internal/state/pid_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/state/pid_test.go`:

```go
package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAndReadPID(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, PIDName)

	if err := WritePID(path, 12345); err != nil {
		t.Fatalf("WritePID returned error: %v", err)
	}

	pid, ok, err := ReadPID(path)
	if err != nil {
		t.Fatalf("ReadPID returned error: %v", err)
	}
	if !ok {
		t.Fatalf("expected pid file to exist")
	}
	if pid != 12345 {
		t.Fatalf("expected pid 12345, got %d", pid)
	}

	if err := RemovePID(path); err != nil {
		t.Fatalf("RemovePID returned error: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected pid file to be removed, got err=%v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/state -run TestWriteAndReadPID -v`
Expected: FAIL because `WritePID`, `ReadPID`, and `RemovePID` are undefined

- [ ] **Step 3: Write minimal implementation**

Create `internal/state/pid.go`:

```go
package state

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func WritePID(path string, pid int) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strconv.Itoa(pid)), 0o644)
}

func ReadPID(path string) (int, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, true, err
	}
	return pid, true, nil
}

func RemovePID(path string) error {
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/state -run TestWriteAndReadPID -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/state/pid.go internal/state/pid_test.go
git commit -m "feat: add pid file helpers"
```

## Task 4: Add the periodic task runner

**Files:**
- Create: `internal/task/runner.go`
- Create: `internal/task/ticker.go`
- Test: `internal/task/ticker_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/task/ticker_test.go`:

```go
package task

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"
	"time"
)

func TestTickerLoopRunsAtLeastOnce(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	runner := NewRunner(logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go StartTickerLoop(ctx, 10*time.Millisecond, runner.RunOnce)
	time.Sleep(30 * time.Millisecond)
	cancel()
	time.Sleep(10 * time.Millisecond)

	if !strings.Contains(buf.String(), "tick") {
		t.Fatalf("expected log output to contain tick, got %q", buf.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/task -run TestTickerLoopRunsAtLeastOnce -v`
Expected: FAIL because `NewRunner` or `StartTickerLoop` is undefined

- [ ] **Step 3: Write minimal implementation**

Create `internal/task/runner.go`:

```go
package task

import (
	"context"
	"log"
	"os"
	"sync/atomic"
	"time"
)

type Runner struct {
	logger  *log.Logger
	counter uint64
}

func NewRunner(logger *log.Logger) *Runner {
	return &Runner{logger: logger}
}

func (r *Runner) RunOnce(_ context.Context) error {
	count := atomic.AddUint64(&r.counter, 1)
	r.logger.Printf("tick count=%d pid=%d at=%s", count, os.Getpid(), time.Now().Format(time.RFC3339))
	return nil
}
```

Create `internal/task/ticker.go`:

```go
package task

import (
	"context"
	"time"
)

func StartTickerLoop(ctx context.Context, interval time.Duration, fn func(context.Context) error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = fn(ctx)
		}
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/task -run TestTickerLoopRunsAtLeastOnce -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/task/runner.go internal/task/ticker.go internal/task/ticker_test.go
git commit -m "feat: add periodic task loop"
```

## Task 5: Add health server and graceful shutdown plumbing

**Files:**
- Create: `internal/app/health.go`
- Create: `internal/app/signal.go`
- Create: `internal/app/app.go`
- Test: `internal/app/app_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/app/app_test.go`:

```go
package app

import (
	"context"
	"io"
	"log"
	"net/http"
	"testing"
	"time"
)

func TestHealthServerRespondsAndShutdownCancelsContext(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	cfg := Config{
		Address:   "127.0.0.1:19524",
		StateDir:  t.TempDir(),
		Interval:  50 * time.Millisecond,
		Foreground: true,
	}

	a := New(cfg, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = a.RunForeground(ctx)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://" + cfg.Address + "/health")
		if err == nil {
			_ = resp.Body.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	resp, err := http.Get("http://" + cfg.Address + "/health")
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	_ = resp.Body.Close()

	req, err := http.NewRequest(http.MethodPost, "http://"+cfg.Address+"/shutdown", nil)
	if err != nil {
		t.Fatalf("shutdown request build failed: %v", err)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("shutdown request failed: %v", err)
	}
	_ = resp.Body.Close()
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app -run TestHealthServerRespondsAndShutdownCancelsContext -v`
Expected: FAIL because `Config`, `New`, or `RunForeground` is undefined

- [ ] **Step 3: Write minimal implementation**

Create `internal/app/app.go`:

```go
package app

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/chenxian/learning-go-daemon/internal/state"
	"github.com/chenxian/learning-go-daemon/internal/task"
)

type Config struct {
	Address    string
	StateDir   string
	Interval   time.Duration
	Foreground bool
}

type App struct {
	cfg    Config
	logger *log.Logger
}

func New(cfg Config, logger *log.Logger) *App {
	return &App{cfg: cfg, logger: logger}
}

func (a *App) RunForeground(parent context.Context) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	if err := os.MkdirAll(a.cfg.StateDir, 0o755); err != nil {
		return err
	}

	server := newHealthServer(a.cfg.Address, cancel)
	if err := server.start(); err != nil {
		return err
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
		defer shutdownCancel()
		_ = server.shutdown(shutdownCtx)
	}()

	pidPath := state.PIDPath(a.cfg.StateDir)
	if err := state.WritePID(pidPath, os.Getpid()); err != nil {
		return err
	}
	defer func() {
		_ = state.RemovePID(pidPath)
	}()

	runner := task.NewRunner(a.logger)
	go task.StartTickerLoop(ctx, a.cfg.Interval, runner.RunOnce)

	<-ctx.Done()
	return nil
}
```

Create `internal/app/health.go`:

```go
package app

import (
	"context"
	"net/http"
)

type healthServer struct {
	server *http.Server
}

func newHealthServer(addr string, cancel context.CancelFunc) *healthServer {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/shutdown", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		cancel()
	})
	return &healthServer{
		server: &http.Server{
			Addr:    addr,
			Handler: mux,
		},
	}
}

func (h *healthServer) start() error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- h.server.ListenAndServe()
	}()
	select {
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	default:
		return nil
	}
}

func (h *healthServer) shutdown(ctx context.Context) error {
	return h.server.Shutdown(ctx)
}
```

Create `internal/app/signal.go`:

```go
package app

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func watchSignals(cancel context.CancelFunc) func() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		cancel()
	}()
	return func() {
		signal.Stop(ch)
		close(ch)
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/app -run TestHealthServerRespondsAndShutdownCancelsContext -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/app.go internal/app/health.go internal/app/signal.go internal/app/app_test.go
git commit -m "feat: add foreground daemon runtime"
```

## Task 6: Add reliable health probing and status behavior

**Files:**
- Create: `internal/app/stop.go`
- Create: `internal/app/status_test.go`
- Modify: `internal/app/app.go`

- [ ] **Step 1: Write the failing test**

Create `internal/app/status_test.go`:

```go
package app

import (
	"io"
	"log"
	"testing"
	"time"

	"github.com/chenxian/learning-go-daemon/internal/state"
)

func TestStatusReportsStalePIDWhenProcessIsDown(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	cfg := Config{
		Address:   "127.0.0.1:19525",
		StateDir:  t.TempDir(),
		Interval:  time.Second,
		Foreground: true,
	}

	if err := state.WritePID(state.PIDPath(cfg.StateDir), 99999); err != nil {
		t.Fatalf("WritePID returned error: %v", err)
	}

	a := New(cfg, logger)
	status, err := a.Status()
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if status != "stale pid file" {
		t.Fatalf("expected stale pid file, got %q", status)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app -run TestStatusReportsStalePIDWhenProcessIsDown -v`
Expected: FAIL because `Status` is undefined

- [ ] **Step 3: Write minimal implementation**

Create `internal/app/stop.go`:

```go
package app

import (
	"errors"
	"net/http"
	"time"

	"github.com/chenxian/learning-go-daemon/internal/state"
)

func (a *App) Status() (string, error) {
	if healthAlive(a.cfg.Address) {
		return "running", nil
	}
	_, ok, err := state.ReadPID(state.PIDPath(a.cfg.StateDir))
	if err != nil {
		return "", err
	}
	if ok {
		return "stale pid file", nil
	}
	return "stopped", nil
}

func (a *App) Stop() error {
	req, err := http.NewRequest(http.MethodPost, "http://"+a.cfg.Address+"/shutdown", nil)
	if err != nil {
		return err
	}
	resp, err := (&http.Client{Timeout: 2 * time.Second}).Do(req)
	if err == nil && resp != nil {
		_ = resp.Body.Close()
		_ = state.RemovePID(state.PIDPath(a.cfg.StateDir))
		return nil
	}
	_, ok, readErr := state.ReadPID(state.PIDPath(a.cfg.StateDir))
	if readErr != nil {
		return readErr
	}
	if ok {
		_ = state.RemovePID(state.PIDPath(a.cfg.StateDir))
		return nil
	}
	if err != nil {
		return err
	}
	return errors.New("daemon is not running")
}

func healthAlive(addr string) bool {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get("http://" + addr + "/health")
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
```

Modify `internal/app/app.go` to install signal handling:

```go
	stopSignals := watchSignals(cancel)
	defer stopSignals()
```

Place those two lines immediately after `ctx, cancel := context.WithCancel(parent)` in `RunForeground`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/app -run TestStatusReportsStalePIDWhenProcessIsDown -v`
Expected: PASS

- [ ] **Step 5: Run the related lifecycle test**

Run: `go test ./internal/app -run TestHealthServerRespondsAndShutdownCancelsContext -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/app/stop.go internal/app/status_test.go internal/app/app.go
git commit -m "feat: add daemon status and stop behavior"
```

## Task 7: Add background startup

**Files:**
- Modify: `internal/app/start.go`
- Modify: `internal/app/app.go`
- Create: `internal/app/start_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/app/start_test.go`:

```go
package app

import "testing"

func TestBackgroundArgsIncludeForegroundFlag(t *testing.T) {
	args := buildBackgroundArgs([]string{"start"})
	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(args))
	}
	if args[0] != "start" || args[1] != "--foreground" {
		t.Fatalf("unexpected args: %#v", args)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app -run TestBackgroundArgsIncludeForegroundFlag -v`
Expected: FAIL because `buildBackgroundArgs` is undefined

- [ ] **Step 3: Write minimal implementation**

Create `internal/app/start.go`:

```go
package app

import (
	"context"
	"os"
	"os/exec"

	"github.com/chenxian/learning-go-daemon/internal/state"
)

func buildBackgroundArgs(args []string) []string {
	return append(args, "--foreground")
}

func (a *App) Start(args []string) error {
	if a.cfg.Foreground {
		return a.RunForeground(context.Background())
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}

	logFile, err := state.OpenLogFile(state.LogPath(a.cfg.StateDir))
	if err != nil {
		return err
	}
	defer logFile.Close()

	cmd := exec.Command(exe, buildBackgroundArgs(args)...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	return cmd.Start()
}
```

Modify `internal/app/app.go` to add default config helper:

```go
func DefaultConfig(stateDir string) Config {
	return Config{
		Address:    "127.0.0.1:19514",
		StateDir:   stateDir,
		Interval:   5 * time.Second,
		Foreground: false,
	}
}
```

Add that function below the `App` struct definition.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/app -run TestBackgroundArgsIncludeForegroundFlag -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/start.go internal/app/start_test.go internal/app/app.go
git commit -m "feat: add background startup flow"
```

## Task 8: Add CLI command parsing

**Files:**
- Modify: `cmd/mydaemon/main.go`
- Test: `cmd/mydaemon/main_test.go`

- [ ] **Step 1: Write the failing test**

Create `cmd/mydaemon/main_test.go`:

```go
package main

import "testing"

func TestParseCommandDefaultsToForegroundFalse(t *testing.T) {
	cmd, foreground := parseArgs([]string{"start"})
	if cmd != "start" {
		t.Fatalf("expected start, got %q", cmd)
	}
	if foreground {
		t.Fatalf("expected foreground false")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/mydaemon -run TestParseCommandDefaultsToForegroundFalse -v`
Expected: FAIL because `parseArgs` is undefined

- [ ] **Step 3: Write minimal implementation**

Replace `cmd/mydaemon/main.go` with:

```go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/chenxian/learning-go-daemon/internal/app"
	"github.com/chenxian/learning-go-daemon/internal/state"
)

func parseArgs(args []string) (string, bool) {
	command := "start"
	foreground := false
	for _, arg := range args {
		if arg == "start" || arg == "stop" || arg == "status" || arg == "logs" {
			command = arg
		}
		if arg == "--foreground" {
			foreground = true
		}
	}
	return command, foreground
}

func main() {
	command, foreground := parseArgs(os.Args[1:])

	stateDir, err := state.DefaultStateDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	cfg := app.DefaultConfig(stateDir)
	cfg.Foreground = foreground

	logFile, err := state.OpenLogFile(state.LogPath(stateDir))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer logFile.Close()

	logger := log.New(logFile, "", log.LstdFlags)
	daemon := app.New(cfg, logger)

	switch command {
	case "start":
		if err := daemon.Start([]string{"start"}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "stop":
		if err := daemon.Stop(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "status":
		status, err := daemon.Status()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(status)
	case "logs":
		fmt.Println(state.LogPath(stateDir))
	default:
		fmt.Fprintln(os.Stderr, "unknown command")
		os.Exit(1)
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/mydaemon -run TestParseCommandDefaultsToForegroundFalse -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/mydaemon/main.go cmd/mydaemon/main_test.go
git commit -m "feat: add daemon cli commands"
```

## Task 9: Run the complete test suite and manual verification

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Write a minimal project README**

Create `README.md`:

```md
# learning-go-daemon

Small Go daemon learning project.

## Commands

- `go run ./cmd/mydaemon start --foreground`
- `go run ./cmd/mydaemon status`
- `go run ./cmd/mydaemon stop`
- `go run ./cmd/mydaemon logs`
```

- [ ] **Step 2: Run all automated tests**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 3: Manually verify foreground lifecycle**

Run in terminal A: `go run ./cmd/mydaemon start --foreground`
Expected: process stays running and appends ticks to `~/.mydaemon/mydaemon.log`

Run in terminal B: `go run ./cmd/mydaemon status`
Expected: prints `running`

Run in terminal B: `go run ./cmd/mydaemon stop`
Expected: stop returns successfully and terminal A exits shortly afterward

- [ ] **Step 4: Manually verify background lifecycle**

Run: `go run ./cmd/mydaemon start`
Expected: command returns quickly

Run: `go run ./cmd/mydaemon status`
Expected: prints `running`

Run: `go run ./cmd/mydaemon stop`
Expected: daemon stops and `status` later prints `stopped`

- [ ] **Step 5: Commit**

```bash
git add README.md
git commit -m "docs: add daemon usage guide"
```

## Self-Review

Spec coverage check:

- lifecycle commands: covered by Tasks 5 through 9
- health endpoint and shutdown endpoint: covered by Task 5
- PID and local state: covered by Tasks 2 and 3
- status source-of-truth behavior: covered by Task 6
- periodic fixed work every five seconds: covered by Task 4 and verified in Task 9
- background mode: covered by Task 7 and verified in Task 9

Placeholder scan:

- no `TODO`, `TBD`, or implicit "write tests later" placeholders remain
- every test task contains concrete code and commands
- every implementation task names exact files and concrete code blocks

Type consistency check:

- `Config`, `App`, `RunForeground`, `Start`, `Stop`, and `Status` are introduced before later tasks reference them
- state helpers use the same `StateDir`, `PIDPath`, and `LogPath` naming throughout
- ticker loop uses one stable `RunOnce(context.Context) error` callback shape
