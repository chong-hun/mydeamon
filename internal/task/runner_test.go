package task

import (
	"context"
	"io"
	"log"
	"path/filepath"
	"testing"
)

type fakeExecutor struct {
	result ExecResult
	err    error
	calls  int
}

func (f *fakeExecutor) Run(context.Context, string, []string) (ExecResult, error) {
	f.calls++
	return f.result, f.err
}

func TestRunnerTransitionsIdleToCompletedOnExitZero(t *testing.T) {
	path := filepath.Join(t.TempDir(), "task-state.json")
	if err := SaveState(path, WorkState{Command: "date", Status: StatusIdle}); err != nil {
		t.Fatalf("SaveState returned error: %v", err)
	}

	exec := &fakeExecutor{result: ExecResult{ExitCode: 0, Stdout: "ok"}}
	runner := NewRunner(log.New(io.Discard, "", 0), path, exec)

	if err := runner.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	state, ok, err := LoadState(path)
	if err != nil || !ok {
		t.Fatalf("LoadState failed: ok=%v err=%v", ok, err)
	}
	if state.Status != StatusCompleted {
		t.Fatalf("expected completed, got %q", state.Status)
	}
	if state.LastExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", state.LastExitCode)
	}
	if state.LastStdout != "ok" {
		t.Fatalf("expected stdout ok, got %q", state.LastStdout)
	}
}

func TestRunnerDoesNotExecuteWhenBlocked(t *testing.T) {
	path := filepath.Join(t.TempDir(), "task-state.json")
	if err := SaveState(path, WorkState{Command: "date", Status: StatusBlocked}); err != nil {
		t.Fatalf("SaveState returned error: %v", err)
	}

	exec := &fakeExecutor{}
	runner := NewRunner(log.New(io.Discard, "", 0), path, exec)

	if err := runner.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if exec.calls != 0 {
		t.Fatalf("expected executor not to run, got %d calls", exec.calls)
	}
}

func TestRunnerTransitionsIdleToNeedsReviewOnExitTen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "task-state.json")
	if err := SaveState(path, WorkState{Command: "date", Status: StatusIdle}); err != nil {
		t.Fatalf("SaveState returned error: %v", err)
	}

	exec := &fakeExecutor{result: ExecResult{ExitCode: 10}}
	runner := NewRunner(log.New(io.Discard, "", 0), path, exec)

	if err := runner.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	state, ok, err := LoadState(path)
	if err != nil || !ok {
		t.Fatalf("LoadState failed: ok=%v err=%v", ok, err)
	}
	if state.Status != StatusNeedsReview {
		t.Fatalf("expected needs_review, got %q", state.Status)
	}
}

func TestApproveRejectResumeTransitions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "task-state.json")
	runner := NewRunner(log.New(io.Discard, "", 0), path, &fakeExecutor{})

	if err := SaveState(path, WorkState{Command: "date", Status: StatusNeedsReview}); err != nil {
		t.Fatalf("SaveState returned error: %v", err)
	}
	if err := runner.Approve(); err != nil {
		t.Fatalf("Approve returned error: %v", err)
	}
	state, _, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState returned error: %v", err)
	}
	if state.Status != StatusCompleted || state.LastReviewAction != "approved" {
		t.Fatalf("unexpected state after approve: %+v", state)
	}

	if err := SaveState(path, WorkState{Command: "date", Status: StatusNeedsReview}); err != nil {
		t.Fatalf("SaveState returned error: %v", err)
	}
	if err := runner.Reject(); err != nil {
		t.Fatalf("Reject returned error: %v", err)
	}
	state, _, err = LoadState(path)
	if err != nil {
		t.Fatalf("LoadState returned error: %v", err)
	}
	if state.Status != StatusBlocked || state.LastReviewAction != "rejected" {
		t.Fatalf("unexpected state after reject: %+v", state)
	}

	if err := runner.Resume(); err != nil {
		t.Fatalf("Resume returned error: %v", err)
	}
	state, _, err = LoadState(path)
	if err != nil {
		t.Fatalf("LoadState returned error: %v", err)
	}
	if state.Status != StatusIdle {
		t.Fatalf("unexpected state after resume: %+v", state)
	}
}
