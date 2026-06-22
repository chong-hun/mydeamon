package task

import (
	"bytes"
	"context"
	"os/exec"
)

type ExecResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

type Executor interface {
	Run(context.Context, string, []string) (ExecResult, error)
}

type OSExecutor struct{}

func (OSExecutor) Run(ctx context.Context, command string, args []string) (ExecResult, error) {
	cmd := exec.CommandContext(ctx, command, args...)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			return ExecResult{}, err
		}
		exitCode = exitErr.ExitCode()
	}

	return ExecResult{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}, nil
}

func classifyExitCode(code int) string {
	switch code {
	case 0:
		return StatusCompleted
	case 10:
		return StatusNeedsReview
	default:
		return StatusBlocked
	}
}
