package tasks

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestServiceCreateTaskSetsDefaults(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "tasks.json"))
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	svc := NewService(store, func() time.Time { return now }, func() string { return "task_001" })

	task, err := svc.Create(CreateInput{
		Title:       "  Review tray UX  ",
		Description: "  Validate quick actions  ",
		Priority:    PriorityMedium,
		Tags:        []string{"desktop"},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if task.ID != "task_001" {
		t.Fatalf("expected task_001, got %q", task.ID)
	}
	if task.Title != "Review tray UX" {
		t.Fatalf("expected trimmed title, got %q", task.Title)
	}
	if task.Description != "Validate quick actions" {
		t.Fatalf("expected trimmed description, got %q", task.Description)
	}
	if task.Status != StatusTodo {
		t.Fatalf("expected todo, got %q", task.Status)
	}
	if task.CreatedAt != now {
		t.Fatalf("expected created at %v, got %v", now, task.CreatedAt)
	}
	if task.UpdatedAt != now {
		t.Fatalf("expected updated at %v, got %v", now, task.UpdatedAt)
	}
}

func TestServiceCompleteRejectsInvalidTransition(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "tasks.json"))
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	svc := NewService(store, func() time.Time { return now }, func() string { return "task_001" })

	if _, err := svc.Create(CreateInput{Title: "Draft API", Priority: PriorityHigh}); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	err := svc.Complete("task_001")
	if err == nil {
		t.Fatal("expected invalid transition error")
	}
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected invalid transition error, got %v", err)
	}
}

func TestServiceCreateRejectsEmptyTitle(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "tasks.json"))
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	svc := NewService(store, func() time.Time { return now }, func() string { return "task_001" })

	_, err := svc.Create(CreateInput{
		Title:    "   ",
		Priority: PriorityLow,
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestServiceCreateRejectsInvalidPriority(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "tasks.json"))
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	svc := NewService(store, func() time.Time { return now }, func() string { return "task_001" })

	_, err := svc.Create(CreateInput{
		Title:    "Draft API",
		Priority: "urgent",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestServiceGetNotFound(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "tasks.json"))
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	svc := NewService(store, func() time.Time { return now }, func() string { return "task_001" })

	_, err := svc.Get("missing")
	if err == nil {
		t.Fatal("expected not found error")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestServiceUpdateRejectsInvalidPriority(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "tasks.json"))
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	svc := NewService(store, func() time.Time { return now }, func() string { return "task_001" })

	if _, err := svc.Create(CreateInput{Title: "Draft API", Priority: PriorityHigh}); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	_, err := svc.Update("task_001", UpdateInput{
		Title:    "Draft API",
		Priority: "urgent",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestServiceUpdateNotFound(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "tasks.json"))
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	svc := NewService(store, func() time.Time { return now }, func() string { return "task_001" })

	_, err := svc.Update("missing", UpdateInput{
		Title:    "Draft API",
		Priority: PriorityMedium,
	})
	if err == nil {
		t.Fatal("expected not found error")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestServiceStartRejectsMissingTask(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "tasks.json"))
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	svc := NewService(store, func() time.Time { return now }, func() string { return "task_001" })

	err := svc.Start("missing")
	if err == nil {
		t.Fatal("expected not found error")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestServiceWorkflowHappyPath(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "tasks.json"))
	currentTime := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	svc := NewService(store, func() time.Time { return currentTime }, func() string { return "task_001" })

	created, err := svc.Create(CreateInput{
		Title:       "Draft API",
		Description: "Initial daemon endpoints",
		Priority:    PriorityHigh,
		Tags:        []string{"backend"},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if created.Status != StatusTodo {
		t.Fatalf("expected todo after create, got %q", created.Status)
	}

	tasks, err := svc.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	currentTime = currentTime.Add(time.Minute)
	updated, err := svc.Update("task_001", UpdateInput{
		Title:       "Draft task API",
		Description: "Refine daemon endpoints",
		Priority:    PriorityMedium,
		Tags:        []string{"backend", "api"},
	})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if updated.Title != "Draft task API" {
		t.Fatalf("expected updated title, got %q", updated.Title)
	}
	if updated.Priority != PriorityMedium {
		t.Fatalf("expected medium priority, got %q", updated.Priority)
	}
	if !updated.UpdatedAt.After(created.UpdatedAt) {
		t.Fatalf("expected updated timestamp to advance")
	}

	currentTime = currentTime.Add(time.Minute)
	if err := svc.Start("task_001"); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	started, err := svc.Get("task_001")
	if err != nil {
		t.Fatalf("Get after Start returned error: %v", err)
	}
	if !started.UpdatedAt.After(updated.UpdatedAt) {
		t.Fatalf("expected transition timestamp to advance")
	}
	currentTime = currentTime.Add(time.Minute)
	if err := svc.Block("task_001"); err != nil {
		t.Fatalf("Block returned error: %v", err)
	}
	currentTime = currentTime.Add(time.Minute)
	if err := svc.MoveToTodo("task_001"); err != nil {
		t.Fatalf("MoveToTodo returned error: %v", err)
	}
	currentTime = currentTime.Add(time.Minute)
	if err := svc.Start("task_001"); err != nil {
		t.Fatalf("Start after move to todo returned error: %v", err)
	}
	currentTime = currentTime.Add(time.Minute)
	if err := svc.Block("task_001"); err != nil {
		t.Fatalf("Block second pass returned error: %v", err)
	}
	currentTime = currentTime.Add(time.Minute)
	if err := svc.Resume("task_001"); err != nil {
		t.Fatalf("Resume returned error: %v", err)
	}
	currentTime = currentTime.Add(time.Minute)
	if err := svc.Review("task_001"); err != nil {
		t.Fatalf("Review returned error: %v", err)
	}
	currentTime = currentTime.Add(time.Minute)
	if err := svc.Reopen("task_001"); err != nil {
		t.Fatalf("Reopen returned error: %v", err)
	}
	currentTime = currentTime.Add(time.Minute)
	if err := svc.Review("task_001"); err != nil {
		t.Fatalf("Review second pass returned error: %v", err)
	}
	currentTime = currentTime.Add(time.Minute)
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
	if task.Title != "Draft task API" {
		t.Fatalf("expected updated title to persist, got %q", task.Title)
	}
	if task.Priority != PriorityMedium {
		t.Fatalf("expected updated priority to persist, got %q", task.Priority)
	}
	if len(task.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(task.Tags))
	}
}
