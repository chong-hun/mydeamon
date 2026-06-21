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

func TestReadPIDMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), PIDName)

	pid, ok, err := ReadPID(path)
	if err != nil {
		t.Fatalf("ReadPID returned error: %v", err)
	}
	if ok {
		t.Fatalf("expected ok=false for missing file")
	}
	if pid != 0 {
		t.Fatalf("expected pid 0 for missing file, got %d", pid)
	}
}

func TestReadPIDMalformedFileReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), PIDName)
	if err := os.WriteFile(path, []byte("abc"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	_, ok, err := ReadPID(path)
	if err == nil {
		t.Fatal("expected error for malformed pid file")
	}
	if !ok {
		t.Fatalf("expected ok=true for existing file")
	}
}

func TestReadPIDRejectsNonPositivePID(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
	}{
		{name: "zero", content: "0"},
		{name: "negative", content: "-7"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), PIDName)
			if err := os.WriteFile(path, []byte(tc.content), 0o644); err != nil {
				t.Fatalf("WriteFile returned error: %v", err)
			}

			_, ok, err := ReadPID(path)
			if err == nil {
				t.Fatal("expected error for non-positive pid")
			}
			if !ok {
				t.Fatalf("expected ok=true for existing file")
			}
		})
	}
}

func TestRemovePIDIsIdempotentWhenMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), PIDName)

	if err := RemovePID(path); err != nil {
		t.Fatalf("first RemovePID returned error: %v", err)
	}

	if err := RemovePID(path); err != nil {
		t.Fatalf("second RemovePID returned error: %v", err)
	}
}
