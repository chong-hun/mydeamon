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
