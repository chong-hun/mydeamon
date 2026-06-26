package tasks

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

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
	if !reflect.DeepEqual(got.Tasks[0], input.Tasks[0]) {
		t.Fatalf("expected task %#v, got %#v", input.Tasks[0], got.Tasks[0])
	}
}
