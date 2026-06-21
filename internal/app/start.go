package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/chenxian/learning-go-daemon/internal/state"
)

const (
	backgroundStartTimeout      = 2 * time.Second
	backgroundStartPollInterval = 50 * time.Millisecond
)

func buildBackgroundArgs(args []string) []string {
	backgroundArgs := make([]string, 0, len(args)+1)
	backgroundArgs = append(backgroundArgs, args...)
	return append(backgroundArgs, "--foreground")
}

func (a *App) Start(args []string) error {
	if a.cfg.Foreground {
		return a.RunForeground(context.Background())
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}

	logFile, err := state.OpenLogFile(state.LogPath(a.cfg.StateDir))
	if err != nil {
		return err
	}
	defer func() {
		_ = logFile.Close()
	}()

	cmd := exec.Command(exe, buildBackgroundArgs(args)...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		return err
	}

	processDone := make(chan error, 1)
	go func() {
		processDone <- cmd.Wait()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), backgroundStartTimeout)
	defer cancel()

	ticker := time.NewTicker(backgroundStartPollInterval)
	defer ticker.Stop()

	if err := waitForBackgroundStart(ctx, a.cfg.Address, healthAlive, processDone, ticker.C); err != nil {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		select {
		case <-processDone:
		default:
		}
		return err
	}

	return nil
}

func waitForBackgroundStart(
	ctx context.Context,
	target string,
	healthCheck func(string) bool,
	processDone <-chan error,
	poll <-chan time.Time,
) error {
	if healthCheck(target) {
		return nil
	}

	for {
		select {
		case err := <-processDone:
			if err == nil {
				return fmt.Errorf("daemon at %s exited before becoming healthy", target)
			}
			return fmt.Errorf("daemon at %s exited before becoming healthy: %w", target, err)
		case <-ctx.Done():
			return fmt.Errorf("daemon at %s did not become healthy: %w", target, ctx.Err())
		case <-poll:
			if healthCheck(target) {
				return nil
			}
		}
	}
}
