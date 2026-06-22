package task

import (
	"context"
	"fmt"
	"log"
	"time"
)

type Runner struct {
	logger    *log.Logger
	statePath string
	executor  Executor
}

func NewRunner(logger *log.Logger, statePath string, executor Executor) *Runner {
	return &Runner{
		logger:    logger,
		statePath: statePath,
		executor:  executor,
	}
}

func (r *Runner) RunOnce(ctx context.Context) error {
	state, ok, err := LoadState(r.statePath)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	if state.Status != StatusIdle {
		return nil
	}

	state.Status = StatusRunning
	state.LastStartAt = time.Now().Format(time.RFC3339)
	if err := SaveState(r.statePath, state); err != nil {
		return err
	}

	r.logger.Printf("command start: %s %v", state.Command, state.Args)
	result, err := r.executor.Run(ctx, state.Command, state.Args)
	if err != nil {
		state.Status = StatusBlocked
		state.LastErrorSummary = err.Error()
		state.LastFinishAt = time.Now().Format(time.RFC3339)
		return SaveState(r.statePath, state)
	}

	state.LastExitCode = result.ExitCode
	state.LastStdout = result.Stdout
	state.LastStderr = result.Stderr
	state.LastFinishAt = time.Now().Format(time.RFC3339)
	state.Status = classifyExitCode(result.ExitCode)

	r.logger.Printf("command end: exit=%d next_state=%s", result.ExitCode, state.Status)
	return SaveState(r.statePath, state)
}

func (r *Runner) Approve() error {
	return r.transitionReview(StatusNeedsReview, StatusCompleted, "approved")
}

func (r *Runner) Reject() error {
	return r.transitionReview(StatusNeedsReview, StatusBlocked, "rejected")
}

func (r *Runner) Resume() error {
	state, ok, err := LoadState(r.statePath)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("no task state found")
	}
	if state.Status != StatusBlocked {
		return fmt.Errorf("resume requires blocked state, got %s", state.Status)
	}
	state.Status = StatusIdle
	return SaveState(r.statePath, state)
}

func (r *Runner) transitionReview(from, to, action string) error {
	state, ok, err := LoadState(r.statePath)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("no task state found")
	}
	if state.Status != from {
		return fmt.Errorf("expected %s state, got %s", from, state.Status)
	}
	state.Status = to
	state.LastReviewAction = action
	return SaveState(r.statePath, state)
}
