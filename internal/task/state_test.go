package task

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveAndLoadStateRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "task-state.json")
	want := WorkState{
		Command:      "date",
		Args:         []string{"+%F %T"},
		Status:       StatusIdle,
		LastExitCode: 0,
	}

	if err := SaveState(path, want); err != nil {
		t.Fatalf("SaveState returned error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if !strings.Contains(string(data), "\"last_exit_code\": 0") {
		t.Fatalf("expected saved JSON to include last_exit_code=0, got %q", string(data))
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
	if got.LastExitCode != want.LastExitCode {
		t.Fatalf("got LastExitCode=%d, want %d", got.LastExitCode, want.LastExitCode)
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

func TestLoadStateInvalidJSONReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "task-state.json")
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	_, ok, err := LoadState(path)
	if err == nil {
		t.Fatal("expected LoadState to return error for invalid JSON")
	}
	if !ok {
		t.Fatal("expected ok=true when state file exists")
	}
}

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
	if state.LastErrorSummary == "" {
		t.Fatal("expected recovery error summary to be set")
	}
}
