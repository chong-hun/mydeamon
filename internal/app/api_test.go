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

func TestTasksAPIGetsTaskByID(t *testing.T) {
	store := tasks.NewStore(filepath.Join(t.TempDir(), "tasks.json"))
	svc := tasks.NewService(store, time.Now, func() string { return "task_001" })
	server := newTaskHTTPHandler(svc, time.Now)

	if _, err := svc.Create(tasks.CreateInput{
		Title:    "Build dashboard",
		Priority: tasks.PriorityHigh,
	}); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/tasks/task_001", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("\"id\":\"task_001\"")) {
		t.Fatalf("expected task id in response: %s", rec.Body.String())
	}
}

func TestTasksAPIPatchesTaskByID(t *testing.T) {
	store := tasks.NewStore(filepath.Join(t.TempDir(), "tasks.json"))
	svc := tasks.NewService(store, time.Now, func() string { return "task_001" })
	server := newTaskHTTPHandler(svc, time.Now)

	if _, err := svc.Create(tasks.CreateInput{
		Title:    "Build dashboard",
		Priority: tasks.PriorityHigh,
	}); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	updateBody, _ := json.Marshal(tasks.UpdateInput{
		Title:       "Ship dashboard",
		Description: "Wire detail view",
		Priority:    tasks.PriorityMedium,
		Tags:        []string{"desktop", "api"},
	})
	req := httptest.NewRequest(http.MethodPatch, "/tasks/task_001", bytes.NewReader(updateBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("Ship dashboard")) {
		t.Fatalf("expected updated title in response: %s", rec.Body.String())
	}
}

func TestTasksAPIStartAndInvalidTransition(t *testing.T) {
	store := tasks.NewStore(filepath.Join(t.TempDir(), "tasks.json"))
	svc := tasks.NewService(store, time.Now, func() string { return "task_001" })
	server := newTaskHTTPHandler(svc, time.Now)

	if _, err := svc.Create(tasks.CreateInput{
		Title:    "Build dashboard",
		Priority: tasks.PriorityHigh,
	}); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/tasks/task_001/start", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/tasks/task_001/start", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}

func TestTasksAPIRejectsUnknownAction(t *testing.T) {
	store := tasks.NewStore(filepath.Join(t.TempDir(), "tasks.json"))
	svc := tasks.NewService(store, time.Now, func() string { return "task_001" })
	server := newTaskHTTPHandler(svc, time.Now)

	if _, err := svc.Create(tasks.CreateInput{
		Title:    "Build dashboard",
		Priority: tasks.PriorityHigh,
	}); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/tasks/task_001/launch", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestTasksAPIReturnsNotFoundForMissingTask(t *testing.T) {
	store := tasks.NewStore(filepath.Join(t.TempDir(), "tasks.json"))
	svc := tasks.NewService(store, time.Now, func() string { return "task_001" })
	server := newTaskHTTPHandler(svc, time.Now)

	req := httptest.NewRequest(http.MethodGet, "/tasks/missing", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestTasksAPIRejectsBadJSON(t *testing.T) {
	store := tasks.NewStore(filepath.Join(t.TempDir(), "tasks.json"))
	svc := tasks.NewService(store, time.Now, func() string { return "task_001" })
	server := newTaskHTTPHandler(svc, time.Now)

	req := httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewBufferString("{"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestTasksAPIRejectsMalformedActionPath(t *testing.T) {
	store := tasks.NewStore(filepath.Join(t.TempDir(), "tasks.json"))
	svc := tasks.NewService(store, time.Now, func() string { return "task_001" })
	server := newTaskHTTPHandler(svc, time.Now)

	if _, err := svc.Create(tasks.CreateInput{
		Title:    "Build dashboard",
		Priority: tasks.PriorityHigh,
	}); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/tasks/task_001/start/extra", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
