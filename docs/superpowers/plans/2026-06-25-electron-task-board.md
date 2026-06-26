# Electron Task Board Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Linux-only Electron desktop app that can start and stop the local Go daemon and manage issue-like tasks through a local HTTP API.

**Architecture:** Keep Go as the authoritative backend. Add a new `internal/tasks` domain for persistent task storage and workflow transitions, expose it through loopback HTTP endpoints in `internal/app`, and add a thin Electron shell that owns tray, daemon lifecycle orchestration, and the dashboard UI. Keep the renderer simple with plain HTML, CSS, and browser-side JavaScript instead of introducing a frontend framework.

**Tech Stack:** Go 1.24, standard library (`net/http`, `encoding/json`, `testing`, `os/exec`), Electron, Node.js built-in test runner (`node --test`), git

---

## File Structure

Planned files and responsibilities:

- `cmd/mydaemon/main.go`
  - keep daemon lifecycle CLI commands and drop command-review verbs that no longer match the product
- `internal/state/paths.go`
  - add fixed paths for `tasks.json` and `runtime.json`
- `internal/tasks/model.go`
  - task types, status constants, priority constants, request payload types
- `internal/tasks/storage.go`
  - atomic file-backed load/save helpers for the task collection
- `internal/tasks/storage_test.go`
  - disk persistence coverage
- `internal/tasks/service.go`
  - task creation, editing, listing, lookup, and workflow transitions
- `internal/tasks/service_test.go`
  - workflow and validation coverage
- `internal/app/api.go`
  - local HTTP handlers for `/runtime` and `/tasks`
- `internal/app/api_test.go`
  - handler coverage for CRUD and transition endpoints
- `internal/app/app.go`
  - wire task service into the daemon runtime and remove command-runner ticker wiring
- `internal/app/start.go`
  - keep background startup behavior, now targeting the task-service daemon
- `internal/app/status_test.go`
  - retain lifecycle coverage after removing ticker-driven task behavior
- `README.md`
  - update product description and desktop run instructions
- `package.json`
  - Electron scripts and dependency metadata
- `desktop/main.js`
  - Electron main process, tray, BrowserWindow lifecycle
- `desktop/daemon-process.js`
  - health check, start, stop, and polling bridge to the Go daemon
- `desktop/daemon-process.test.js`
  - pure Node tests for daemon process orchestration helpers
- `desktop/preload.js`
  - safe IPC surface for the renderer
- `desktop/renderer/index.html`
  - dashboard markup
- `desktop/renderer/styles.css`
  - dashboard styles
- `desktop/renderer/app.js`
  - renderer state, fetch calls, filtering, modal behavior, and task actions
- `desktop/renderer/view-model.js`
  - pure helpers for filtering tasks and deriving allowed actions
- `desktop/renderer/view-model.test.js`
  - unit tests for renderer logic

## Task 1: Add the task domain model and file paths

**Files:**
- Create: `internal/tasks/model.go`
- Modify: `internal/state/paths.go`
- Test: `internal/tasks/storage_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/tasks/storage_test.go`:

```go
package tasks

import (
	"path/filepath"
	"testing"

	"github.com/chenxian/learning-go-daemon/internal/state"
)

func TestDefaultCollectionPathUsesStateHelper(t *testing.T) {
	root := t.TempDir()
	path := state.TasksPath(root)

	expected := filepath.Join(root, "tasks.json")
	if path != expected {
		t.Fatalf("expected %q, got %q", expected, path)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tasks -run TestDefaultCollectionPathUsesStateHelper -v`
Expected: FAIL because `internal/tasks` does not exist and `state.TasksPath` is undefined

- [ ] **Step 3: Write minimal implementation**

Update `internal/state/paths.go`:

```go
const (
	DirName     = ".mydaemon"
	PIDName     = "mydaemon.pid"
	LogName     = "mydaemon.log"
	TasksName   = "tasks.json"
	RuntimeName = "runtime.json"
)

func TasksPath(root string) string {
	return filepath.Join(root, TasksName)
}

func RuntimePath(root string) string {
	return filepath.Join(root, RuntimeName)
}
```

Create `internal/tasks/model.go`:

```go
package tasks

import "time"

const (
	StatusTodo        = "todo"
	StatusInProgress  = "in_progress"
	StatusNeedsReview = "needs_review"
	StatusBlocked     = "blocked"
	StatusDone        = "done"
)

const (
	PriorityLow    = "low"
	PriorityMedium = "medium"
	PriorityHigh   = "high"
)

type Task struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Priority    string    `json:"priority"`
	Tags        []string  `json:"tags"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Collection struct {
	Tasks []Task `json:"tasks"`
}

type CreateInput struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Priority    string   `json:"priority"`
	Tags        []string `json:"tags"`
}

type UpdateInput struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Priority    string   `json:"priority"`
	Tags        []string `json:"tags"`
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tasks -run TestDefaultCollectionPathUsesStateHelper -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/state/paths.go internal/tasks/model.go internal/tasks/storage_test.go
git commit -m "feat: add task model and state paths"
```

## Task 2: Implement file-backed task storage

**Files:**
- Create: `internal/tasks/storage.go`
- Test: `internal/tasks/storage_test.go`

- [ ] **Step 1: Write the failing test**

Extend `internal/tasks/storage_test.go`:

```go
func TestStoreSaveAndLoadRoundTripsTasks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	store := NewStore(path)

	createdAt := time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC)
	input := Collection{
		Tasks: []Task{{
			ID:          "task_001",
			Title:       "Draft daemon API",
			Description: "Define /tasks endpoints",
			Status:      StatusTodo,
			Priority:    PriorityHigh,
			Tags:        []string{"backend", "api"},
			CreatedAt:   createdAt,
			UpdatedAt:   createdAt,
		}},
	}

	if err := store.Save(input); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(got.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(got.Tasks))
	}
	if got.Tasks[0].Title != "Draft daemon API" {
		t.Fatalf("unexpected title: %q", got.Tasks[0].Title)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tasks -run TestStoreSaveAndLoadRoundTripsTasks -v`
Expected: FAIL because `NewStore`, `Save`, and `Load` are undefined

- [ ] **Step 3: Write minimal implementation**

Create `internal/tasks/storage.go`:

```go
package tasks

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Store struct {
	path string
}

func NewStore(path string) *Store {
	return &Store{path: path}
}

func (s *Store) Load() (Collection, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return Collection{}, nil
	}
	if err != nil {
		return Collection{}, err
	}

	var collection Collection
	if err := json.Unmarshal(data, &collection); err != nil {
		return Collection{}, err
	}
	return collection, nil
}

func (s *Store) Save(collection Collection) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(collection, "", "  ")
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(s.path), "tasks-*.json")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.path)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tasks -run TestStoreSaveAndLoadRoundTripsTasks -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tasks/storage.go internal/tasks/storage_test.go
git commit -m "feat: add file-backed task storage"
```

## Task 3: Add task service validation and workflow transitions

**Files:**
- Create: `internal/tasks/service.go`
- Test: `internal/tasks/service_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/tasks/service_test.go`:

```go
package tasks

import (
	"path/filepath"
	"testing"
	"time"
)

func TestServiceCreateTaskSetsDefaults(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "tasks.json"))
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	svc := NewService(store, func() time.Time { return now }, func() string { return "task_001" })

	task, err := svc.Create(CreateInput{
		Title:       "Review tray UX",
		Description: "Validate quick actions",
		Priority:    PriorityMedium,
		Tags:        []string{"desktop"},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if task.Status != StatusTodo {
		t.Fatalf("expected todo, got %q", task.Status)
	}
	if task.ID != "task_001" {
		t.Fatalf("expected task_001, got %q", task.ID)
	}
}

func TestServiceCompleteRejectsInvalidTransition(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "tasks.json"))
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	svc := NewService(store, func() time.Time { return now }, func() string { return "task_001" })

	if _, err := svc.Create(CreateInput{Title: "Draft API", Priority: PriorityHigh}); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if err := svc.Complete("task_001"); err == nil {
		t.Fatal("expected invalid transition error")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tasks -run 'TestService(CreateTaskSetsDefaults|CompleteRejectsInvalidTransition)' -v`
Expected: FAIL because `NewService`, `Create`, and `Complete` are undefined

- [ ] **Step 3: Write minimal implementation**

Create `internal/tasks/service.go`:

```go
package tasks

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

type Service struct {
	store  *Store
	now    func() time.Time
	nextID func() string
}

func NewService(store *Store, now func() time.Time, nextID func() string) *Service {
	return &Service{store: store, now: now, nextID: nextID}
}

func (s *Service) List() ([]Task, error) {
	collection, err := s.store.Load()
	if err != nil {
		return nil, err
	}
	return collection.Tasks, nil
}

func (s *Service) Create(input CreateInput) (Task, error) {
	if strings.TrimSpace(input.Title) == "" {
		return Task{}, errors.New("title is required")
	}
	if !slices.Contains([]string{PriorityLow, PriorityMedium, PriorityHigh}, input.Priority) {
		return Task{}, fmt.Errorf("invalid priority %q", input.Priority)
	}

	collection, err := s.store.Load()
	if err != nil {
		return Task{}, err
	}

	now := s.now().UTC()
	task := Task{
		ID:          s.nextID(),
		Title:       strings.TrimSpace(input.Title),
		Description: strings.TrimSpace(input.Description),
		Status:      StatusTodo,
		Priority:    input.Priority,
		Tags:        input.Tags,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	collection.Tasks = append(collection.Tasks, task)
	if err := s.store.Save(collection); err != nil {
		return Task{}, err
	}
	return task, nil
}

func (s *Service) Complete(id string) error {
	return s.transition(id, StatusNeedsReview, StatusDone)
}

func (s *Service) transition(id, from, to string) error {
	collection, err := s.store.Load()
	if err != nil {
		return err
	}

	for i := range collection.Tasks {
		if collection.Tasks[i].ID != id {
			continue
		}
		if collection.Tasks[i].Status != from {
			return fmt.Errorf("cannot move task from %s to %s", collection.Tasks[i].Status, to)
		}
		collection.Tasks[i].Status = to
		collection.Tasks[i].UpdatedAt = s.now().UTC()
		return s.store.Save(collection)
	}
	return fmt.Errorf("task %s not found", id)
}
```

- [ ] **Step 4: Expand the implementation to full workflow coverage**

Update `internal/tasks/service.go` with the remaining methods:

```go
func (s *Service) Get(id string) (Task, error) {
	collection, err := s.store.Load()
	if err != nil {
		return Task{}, err
	}
	for _, task := range collection.Tasks {
		if task.ID == id {
			return task, nil
		}
	}
	return Task{}, fmt.Errorf("task %s not found", id)
}

func (s *Service) Update(id string, input UpdateInput) (Task, error) {
	if strings.TrimSpace(input.Title) == "" {
		return Task{}, errors.New("title is required")
	}
	if !slices.Contains([]string{PriorityLow, PriorityMedium, PriorityHigh}, input.Priority) {
		return Task{}, fmt.Errorf("invalid priority %q", input.Priority)
	}

	collection, err := s.store.Load()
	if err != nil {
		return Task{}, err
	}

	for i := range collection.Tasks {
		if collection.Tasks[i].ID != id {
			continue
		}
		collection.Tasks[i].Title = strings.TrimSpace(input.Title)
		collection.Tasks[i].Description = strings.TrimSpace(input.Description)
		collection.Tasks[i].Priority = input.Priority
		collection.Tasks[i].Tags = input.Tags
		collection.Tasks[i].UpdatedAt = s.now().UTC()
		if err := s.store.Save(collection); err != nil {
			return Task{}, err
		}
		return collection.Tasks[i], nil
	}

	return Task{}, fmt.Errorf("task %s not found", id)
}

func (s *Service) Start(id string) error      { return s.transition(id, StatusTodo, StatusInProgress) }
func (s *Service) Block(id string) error      { return s.transition(id, StatusInProgress, StatusBlocked) }
func (s *Service) Review(id string) error     { return s.transition(id, StatusInProgress, StatusNeedsReview) }
func (s *Service) Reopen(id string) error     { return s.transition(id, StatusNeedsReview, StatusInProgress) }
func (s *Service) Complete(id string) error   { return s.transition(id, StatusNeedsReview, StatusDone) }
func (s *Service) Resume(id string) error     { return s.transition(id, StatusBlocked, StatusInProgress) }
func (s *Service) MoveToTodo(id string) error { return s.transition(id, StatusBlocked, StatusTodo) }
```

Add coverage to `internal/tasks/service_test.go`:

```go
func TestServiceWorkflowHappyPath(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "tasks.json"))
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	svc := NewService(store, func() time.Time { return now }, func() string { return "task_001" })

	if _, err := svc.Create(CreateInput{Title: "Draft API", Priority: PriorityHigh}); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if err := svc.Start("task_001"); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if err := svc.Review("task_001"); err != nil {
		t.Fatalf("Review returned error: %v", err)
	}
	if err := svc.Complete("task_001"); err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	task, err := svc.Get("task_001")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if task.Status != StatusDone {
		t.Fatalf("expected done, got %q", task.Status)
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/tasks -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/tasks/service.go internal/tasks/service_test.go
git commit -m "feat: add task service and workflow rules"
```

## Task 4: Expose the task API from the daemon and remove command-runner wiring

**Files:**
- Create: `internal/app/api.go`
- Create: `internal/app/api_test.go`
- Modify: `internal/app/app.go`
- Modify: `cmd/mydaemon/main.go`
- Modify: `README.md`

- [ ] **Step 1: Write the failing API test**

Create `internal/app/api_test.go`:

```go
package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenxian/learning-go-daemon/internal/tasks"
)

func TestTasksAPIListsAndCreatesTasks(t *testing.T) {
	store := tasks.NewStore(filepath.Join(t.TempDir(), "tasks.json"))
	svc := tasks.NewService(store, time.Now, func() string { return "task_001" })
	server := newTaskHTTPHandler(svc, time.Now)

	createBody, _ := json.Marshal(tasks.CreateInput{
		Title:       "Build dashboard",
		Description: "Add task list and detail view",
		Priority:    tasks.PriorityHigh,
		Tags:        []string{"desktop"},
	})
	req := httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/tasks", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("Build dashboard")) {
		t.Fatalf("expected task title in response: %s", rec.Body.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app -run TestTasksAPIListsAndCreatesTasks -v`
Expected: FAIL because `newTaskHTTPHandler` is undefined

- [ ] **Step 3: Write minimal implementation**

Create `internal/app/api.go`:

```go
package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/chenxian/learning-go-daemon/internal/tasks"
)

func newTaskHTTPHandler(service *tasks.Service, now func() time.Time) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/runtime", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":     "running",
			"started_at": now().UTC(),
		})
	})

	mux.HandleFunc("/tasks", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			items, err := service.List()
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"tasks": items})
		case http.MethodPost:
			var input tasks.CreateInput
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			item, err := service.Create(input)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusCreated, item)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/tasks/", func(w http.ResponseWriter, r *http.Request) {
		id, action := splitTaskPath(r.URL.Path)
		switch {
		case action == "" && r.Method == http.MethodGet:
			item, err := service.Get(id)
			if err != nil {
				writeError(w, http.StatusNotFound, err)
				return
			}
			writeJSON(w, http.StatusOK, item)
		case action == "" && r.Method == http.MethodPatch:
			var input tasks.UpdateInput
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			item, err := service.Update(id, input)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusOK, item)
		case r.Method == http.MethodPost:
			if err := runTaskAction(service, id, action); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	return mux
}

func splitTaskPath(path string) (string, string) {
	trimmed := strings.TrimPrefix(path, "/tasks/")
	parts := strings.Split(trimmed, "/")
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

func runTaskAction(service *tasks.Service, id, action string) error {
	switch action {
	case "start":
		return service.Start(id)
	case "block":
		return service.Block(id)
	case "review":
		return service.Review(id)
	case "reopen":
		return service.Reopen(id)
	case "complete":
		return service.Complete(id)
	case "resume":
		return service.Resume(id)
	case "todo":
		return service.MoveToTodo(id)
	default:
		return errors.New("unknown task action")
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
```

Modify `internal/app/app.go` to wire the service and drop ticker wiring:

```go
type App struct {
	cfg         Config
	logger      *log.Logger
	taskService *tasks.Service
	mu          sync.RWMutex
	health      *healthServer
}

func New(cfg Config, logger *log.Logger) *App {
	store := tasks.NewStore(state.TasksPath(cfg.StateDir))
	service := tasks.NewService(store, time.Now, func() string {
		return fmt.Sprintf("task_%d", time.Now().UnixMilli())
	})
	return &App{cfg: cfg, logger: logger, taskService: service}
}
```

Update `cmd/mydaemon/main.go` to keep only lifecycle CLI commands:

```go
case "start", "stop", "status", "logs":
	// keep existing command handling
default:
	return parsedArgs{}, fmt.Errorf("unknown argument: %s", arg)
```

Update `README.md` command list:

```md
## Commands

- `go run ./cmd/mydaemon start --foreground`
- `go run ./cmd/mydaemon status`
- `go run ./cmd/mydaemon stop`
- `go run ./cmd/mydaemon logs`

## Desktop Goal

The daemon is the local backend for a Linux Electron task board.
```

- [ ] **Step 4: Integrate the new API handler into the health server**

Update `internal/app/health.go`:

```go
func newHealthServer(addr string, cancel context.CancelFunc, tasksHandler http.Handler) *healthServer {
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
	mux.Handle("/runtime", tasksHandler)
	mux.Handle("/tasks", tasksHandler)
	mux.Handle("/tasks/", tasksHandler)
	return &healthServer{server: &http.Server{Addr: addr, Handler: mux}}
}
```

Update `internal/app/app.go` runtime startup:

```go
server := newHealthServer(a.cfg.Address, cancel, newTaskHTTPHandler(a.taskService, time.Now))
if err := server.start(); err != nil {
	return err
}
```

- [ ] **Step 5: Run tests to verify the backend API passes**

Run: `go test ./internal/app ./internal/tasks ./internal/state -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add cmd/mydaemon/main.go internal/app/api.go internal/app/api_test.go internal/app/app.go internal/app/health.go README.md
git commit -m "feat: expose local task api from daemon"
```

## Task 5: Bootstrap the Electron shell and daemon process bridge

**Files:**
- Create: `package.json`
- Create: `desktop/main.js`
- Create: `desktop/daemon-process.js`
- Create: `desktop/daemon-process.test.js`
- Create: `desktop/preload.js`

- [ ] **Step 1: Write the failing Node test**

Create `desktop/daemon-process.test.js`:

```js
const test = require('node:test');
const assert = require('node:assert/strict');
const { buildStartCommand, buildTaskURL } = require('./daemon-process');

test('buildStartCommand starts mydaemon in background mode', () => {
  assert.deepEqual(buildStartCommand('/tmp/mydaemon'), {
    command: '/tmp/mydaemon',
    args: ['start'],
  });
});

test('buildTaskURL points at the local daemon', () => {
  assert.equal(buildTaskURL('127.0.0.1:19514', '/tasks'), 'http://127.0.0.1:19514/tasks');
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test desktop/daemon-process.test.js`
Expected: FAIL because `desktop/daemon-process.js` does not exist

- [ ] **Step 3: Write minimal implementation**

Create `package.json`:

```json
{
  "name": "learning-go-daemon",
  "version": "0.1.0",
  "private": true,
  "main": "desktop/main.js",
  "scripts": {
    "electron": "electron .",
    "test:desktop": "node --test desktop/*.test.js desktop/renderer/*.test.js"
  },
  "devDependencies": {
    "electron": "^37.0.0"
  }
}
```

Create `desktop/daemon-process.js`:

```js
const { execFile } = require('node:child_process');
const { promisify } = require('node:util');

const execFileAsync = promisify(execFile);

function buildStartCommand(binaryPath) {
  return { command: binaryPath, args: ['start'] };
}

function buildTaskURL(address, path) {
  return `http://${address}${path}`;
}

async function startDaemon(binaryPath) {
  const { command, args } = buildStartCommand(binaryPath);
  await execFileAsync(command, args);
}

module.exports = {
  buildStartCommand,
  buildTaskURL,
  startDaemon,
};
```

Create `desktop/preload.js`:

```js
const { contextBridge, ipcRenderer } = require('electron');

contextBridge.exposeInMainWorld('desktopAPI', {
  loadTasks: () => ipcRenderer.invoke('tasks:list'),
  createTask: (payload) => ipcRenderer.invoke('tasks:create', payload),
  transitionTask: (id, action) => ipcRenderer.invoke('tasks:action', { id, action }),
  updateTask: (id, payload) => ipcRenderer.invoke('tasks:update', { id, payload }),
  daemonStatus: () => ipcRenderer.invoke('daemon:status'),
});
```

Create `desktop/main.js`:

```js
const path = require('node:path');
const { app, BrowserWindow, Tray, Menu } = require('electron');

let mainWindow = null;
let tray = null;

function trayIconDataURL() {
  return 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAABAAAAAQCAQAAAC1+jfqAAAApElEQVR42mNgoBvg4uJiwA2YGBh+g4GB4T8DA8P/BgaG/zMwMPyfgYHhP4PBgf8MDAz/JyAg+E8QwMDA8J8B4n8GBob/DAwM/ycgIPgPEMDBwYGBgYHhPwMDw38GBob/MzAw/J+BgWEgkB8SExP9T0BA8B8mJiY+AwPDf4aGhv8MDAz/JyAg+E8QwMDA8J8B4n8GBob/DAwM/ycgIPgPAwMDAwPjPwYGhv8MDAz/JwAA3twTM8VnZwoAAAAASUVORK5CYII=';
}

function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1100,
    height: 760,
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
    },
  });
  mainWindow.loadFile(path.join(__dirname, 'renderer/index.html'));
}

app.whenReady().then(() => {
  createWindow();
  tray = new Tray(trayIconDataURL());
  tray.setContextMenu(Menu.buildFromTemplate([
    { label: 'Open Dashboard', click: () => mainWindow.show() },
    { type: 'separator' },
    { label: 'Quit App', click: () => app.quit() },
  ]));
});
```

- [ ] **Step 4: Run the desktop unit test to verify it passes**

Run: `node --test desktop/daemon-process.test.js`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add package.json desktop/main.js desktop/daemon-process.js desktop/daemon-process.test.js desktop/preload.js
git commit -m "feat: bootstrap electron shell and daemon bridge"
```

## Task 6: Build the renderer dashboard and list-detail task workflow

**Files:**
- Create: `desktop/renderer/index.html`
- Create: `desktop/renderer/styles.css`
- Create: `desktop/renderer/app.js`
- Create: `desktop/renderer/view-model.js`
- Create: `desktop/renderer/view-model.test.js`

- [ ] **Step 1: Write the failing renderer helper test**

Create `desktop/renderer/view-model.test.js`:

```js
const test = require('node:test');
const assert = require('node:assert/strict');
const { filterTasks, allowedActions } = require('./view-model');

test('filterTasks matches title text and status', () => {
  const tasks = [
    { id: '1', title: 'Draft daemon API', description: '', status: 'todo', priority: 'high', tags: [] },
    { id: '2', title: 'Review tray UX', description: '', status: 'done', priority: 'medium', tags: [] },
  ];

  const result = filterTasks(tasks, { query: 'draft', status: 'todo', priority: 'all' });
  assert.equal(result.length, 1);
  assert.equal(result[0].id, '1');
});

test('allowedActions returns workflow actions for in_progress', () => {
  assert.deepEqual(allowedActions('in_progress'), ['review', 'block']);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test desktop/renderer/view-model.test.js`
Expected: FAIL because `desktop/renderer/view-model.js` does not exist

- [ ] **Step 3: Write minimal implementation**

Create `desktop/renderer/view-model.js`:

```js
function filterTasks(tasks, filters) {
  const query = filters.query.trim().toLowerCase();
  return tasks.filter((task) => {
    const matchesQuery =
      query === '' ||
      task.title.toLowerCase().includes(query) ||
      task.description.toLowerCase().includes(query);
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
```

Create `desktop/renderer/index.html`:

```html
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>mydaemon</title>
    <link rel="stylesheet" href="./styles.css">
  </head>
  <body>
    <header class="topbar">
      <h1>mydaemon</h1>
      <div class="toolbar">
        <input id="search" placeholder="Search tasks">
        <select id="status-filter">
          <option value="all">All statuses</option>
          <option value="todo">Todo</option>
          <option value="in_progress">In progress</option>
          <option value="needs_review">Needs review</option>
          <option value="blocked">Blocked</option>
          <option value="done">Done</option>
        </select>
        <select id="priority-filter">
          <option value="all">All priorities</option>
          <option value="low">Low</option>
          <option value="medium">Medium</option>
          <option value="high">High</option>
        </select>
        <button id="new-task-button">New Task</button>
      </div>
    </header>
    <main class="layout">
      <aside class="task-list" id="task-list"></aside>
      <section class="task-detail" id="task-detail"></section>
    </main>
    <dialog id="create-dialog">
      <form method="dialog" id="create-form">
        <label>Title <input name="title" required></label>
        <label>Description <textarea name="description"></textarea></label>
        <label>Priority
          <select name="priority">
            <option value="low">Low</option>
            <option value="medium" selected>Medium</option>
            <option value="high">High</option>
          </select>
        </label>
        <label>Tags <input name="tags" placeholder="backend, api"></label>
        <menu>
          <button value="cancel">Cancel</button>
          <button id="create-submit" value="default">Create</button>
        </menu>
      </form>
    </dialog>
    <script src="./view-model.js"></script>
    <script src="./app.js"></script>
  </body>
</html>
```

Create `desktop/renderer/styles.css`:

```css
:root {
  --bg: #f4efe6;
  --panel: #fffaf4;
  --line: #d7c9b3;
  --text: #231d15;
  --accent: #135c49;
  --muted: #6f6557;
}

body {
  margin: 0;
  font-family: "IBM Plex Sans", sans-serif;
  background: radial-gradient(circle at top left, #fff6df, var(--bg));
  color: var(--text);
}

.layout {
  display: grid;
  grid-template-columns: 360px 1fr;
  min-height: calc(100vh - 72px);
}
```

Create `desktop/renderer/app.js`:

```js
const { filterTasks, allowedActions } = window.viewModel;

const state = {
  tasks: [],
  selectedTaskID: null,
  filters: { query: '', status: 'all', priority: 'all' },
};

async function refreshTasks() {
  const response = await window.desktopAPI.loadTasks();
  state.tasks = response.tasks;
  if (!state.selectedTaskID && state.tasks.length > 0) {
    state.selectedTaskID = state.tasks[0].id;
  }
  render();
}

function render() {
  const items = filterTasks(state.tasks, state.filters);
  const selected = state.tasks.find((task) => task.id === state.selectedTaskID) || null;
  renderList(items);
  renderDetail(selected);
}

function renderList(items) {
  const list = document.getElementById('task-list');
  list.innerHTML = items.map((task) => `
    <button class="task-row" data-task-id="${task.id}">
      <strong>${task.title}</strong>
      <span>${task.status}</span>
      <span>${task.priority}</span>
      <span>${task.tags.join(', ')}</span>
    </button>
  `).join('');

  for (const button of list.querySelectorAll('[data-task-id]')) {
    button.addEventListener('click', () => {
      state.selectedTaskID = button.dataset.taskId;
      render();
    });
  }
}

function renderDetail(task) {
  const panel = document.getElementById('task-detail');
  if (!task) {
    panel.innerHTML = '<p>Select a task to view details.</p>';
    return;
  }

  const actions = allowedActions(task.status)
    .map((action) => `<button data-action="${action}">${action}</button>`)
    .join('');

  panel.innerHTML = `
    <h2>${task.title}</h2>
    <p>${task.description || ''}</p>
    <p>Status: ${task.status}</p>
    <p>Priority: ${task.priority}</p>
    <p>Tags: ${task.tags.join(', ')}</p>
    <div class="actions">${actions}</div>
  `;

  for (const button of panel.querySelectorAll('[data-action]')) {
    button.addEventListener('click', async () => {
      await window.desktopAPI.transitionTask(task.id, button.dataset.action);
      await refreshTasks();
    });
  }
}

function bindFilters() {
  document.getElementById('search').addEventListener('input', (event) => {
    state.filters.query = event.target.value;
    render();
  });
  document.getElementById('status-filter').addEventListener('change', (event) => {
    state.filters.status = event.target.value;
    render();
  });
  document.getElementById('priority-filter').addEventListener('change', (event) => {
    state.filters.priority = event.target.value;
    render();
  });
}

function bindCreateDialog() {
  const dialog = document.getElementById('create-dialog');
  const form = document.getElementById('create-form');

  document.getElementById('new-task-button').addEventListener('click', () => {
    dialog.showModal();
  });

  form.addEventListener('submit', async (event) => {
    event.preventDefault();
    const formData = new FormData(form);
    await window.desktopAPI.createTask({
      title: String(formData.get('title') || ''),
      description: String(formData.get('description') || ''),
      priority: String(formData.get('priority') || 'medium'),
      tags: String(formData.get('tags') || '')
        .split(',')
        .map((item) => item.trim())
        .filter(Boolean),
    });
    form.reset();
    dialog.close();
    await refreshTasks();
  });
}

window.addEventListener('DOMContentLoaded', async () => {
  bindFilters();
  bindCreateDialog();
  await refreshTasks();
});
```

- [ ] **Step 4: Connect the renderer to IPC actions**

Expand `desktop/main.js`:

```js
const {
  createTask,
  listTasks,
  updateTask,
  runTaskAction,
  ensureDaemonRunning,
  getDaemonStatus,
} = require('./daemon-process');
const { ipcMain } = require('electron');

ipcMain.handle('tasks:list', async () => listTasks());
ipcMain.handle('tasks:create', async (_event, payload) => createTask(payload));
ipcMain.handle('tasks:update', async (_event, { id, payload }) => updateTask(id, payload));
ipcMain.handle('tasks:action', async (_event, { id, action }) => runTaskAction(id, action));
ipcMain.handle('daemon:status', async () => getDaemonStatus());

app.whenReady().then(async () => {
  await ensureDaemonRunning();
  createWindow();
  createTray();
});
```

Expand `desktop/daemon-process.js`:

```js
const LOCAL_ADDRESS = '127.0.0.1:19514';

async function fetchJSON(address, path, options = {}) {
  const response = await fetch(buildTaskURL(address, path), options);
  if (!response.ok) {
    throw new Error(`request failed with status ${response.status}`);
  }
  return response.status === 204 ? null : response.json();
}

async function getDaemonStatus() {
  try {
    await fetchJSON(LOCAL_ADDRESS, '/health');
    return { status: 'running', address: LOCAL_ADDRESS };
  } catch {
    return { status: 'stopped', address: LOCAL_ADDRESS };
  }
}

async function ensureDaemonRunning(binaryPath) {
  const status = await getDaemonStatus();
  if (status.status === 'running') {
    return status;
  }
  await startDaemon(binaryPath);
  return getDaemonStatus();
}

async function listTasks() {
  return fetchJSON(LOCAL_ADDRESS, '/tasks');
}

async function createTask(payload) {
  return fetchJSON(LOCAL_ADDRESS, '/tasks', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
}

async function updateTask(id, payload) {
  return fetchJSON(LOCAL_ADDRESS, `/tasks/${id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
}

async function runTaskAction(id, action) {
  return fetchJSON(LOCAL_ADDRESS, `/tasks/${id}/${action}`, {
    method: 'POST',
  });
}
```

- [ ] **Step 5: Run the renderer unit tests**

Run: `node --test desktop/renderer/view-model.test.js desktop/daemon-process.test.js`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add desktop/main.js desktop/daemon-process.js desktop/renderer/index.html desktop/renderer/styles.css desktop/renderer/app.js desktop/renderer/view-model.js desktop/renderer/view-model.test.js
git commit -m "feat: add electron dashboard ui"
```

## Task 7: Verify the end-to-end desktop flow and document it

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Add desktop run instructions**

Update `README.md`:

```md
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
```

- [ ] **Step 2: Run the automated test suites**

Run: `go test ./...`
Expected: PASS

Run: `node --test desktop/*.test.js desktop/renderer/*.test.js`
Expected: PASS

- [ ] **Step 3: Perform manual Linux verification**

Run:

```bash
go run ./cmd/mydaemon stop || true
npm install
npm run electron
```

Verify:

- the app window opens
- the daemon starts if it was stopped
- the tray menu shows `Open Dashboard`, daemon status, `Start Daemon`, `Stop Daemon`, and `Quit App`
- creating a task in the modal adds it to the list
- selecting a task updates the detail panel
- `Start`, `Move to Review`, `Block`, `Reopen`, `Complete`, and `Resume` only appear when valid for the selected status
- closing and reopening the Electron app preserves tasks on disk

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs: add desktop workflow instructions"
```

## Spec Coverage Check

- Linux-only Electron shell: Task 5 and Task 6
- tray plus dashboard layout: Task 5 and Task 6
- Electron-managed daemon start and stop: Task 5 and Task 6
- issue-like task creation and editing: Task 3, Task 4, and Task 6
- file-backed persistence: Task 2
- workflow transitions: Task 3 and Task 6
- loopback HTTP API: Task 4
- local-only polling architecture: Task 4 and Task 6
- README and manual Linux verification: Task 7

## Placeholder Scan

- No `TBD`, `TODO`, or “implement later” markers remain.
- Each task names exact files and concrete commands.
- Each code-writing step includes concrete code blocks rather than abstract instructions.

## Type Consistency Check

- Task statuses are consistently named `todo`, `in_progress`, `needs_review`, `blocked`, and `done`.
- The backend uses `CreateInput` and `UpdateInput`, and the API layer references those exact types.
- Renderer actions use `start`, `review`, `block`, `reopen`, `complete`, `resume`, and `todo`; the backend action router in Task 4 must map those names directly to the service methods from Task 3.
