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
