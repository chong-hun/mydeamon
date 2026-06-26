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
