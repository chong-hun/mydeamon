package task

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	StatusIdle        = "idle"
	StatusRunning     = "running"
	StatusNeedsReview = "needs_review"
	StatusBlocked     = "blocked"
	StatusCompleted   = "completed"
)

type WorkState struct {
	Command          string   `json:"command"`
	Args             []string `json:"args"`
	Status           string   `json:"status"`
	LastStartAt      string   `json:"last_start_at,omitempty"`
	LastFinishAt     string   `json:"last_finish_at,omitempty"`
	LastExitCode     int      `json:"last_exit_code"`
	LastStdout       string   `json:"last_stdout,omitempty"`
	LastStderr       string   `json:"last_stderr,omitempty"`
	LastErrorSummary string   `json:"last_error_summary,omitempty"`
	LastReviewAction string   `json:"last_review_action,omitempty"`
}

func SaveState(path string, state WorkState) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Chmod(0o644); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	return nil
}

func LoadState(path string) (WorkState, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return WorkState{}, false, nil
	}
	if err != nil {
		return WorkState{}, false, err
	}
	var state WorkState
	if err := json.Unmarshal(data, &state); err != nil {
		return WorkState{}, true, err
	}
	return state, true, nil
}

func RewriteRunningStateToBlocked(path string) error {
	state, ok, err := LoadState(path)
	if err != nil || !ok {
		return err
	}
	if state.Status != StatusRunning {
		return nil
	}
	state.Status = StatusBlocked
	state.LastErrorSummary = "daemon restarted while command was running"
	return SaveState(path, state)
}
