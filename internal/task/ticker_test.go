package task

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"
	"time"
)

func TestTickerLoopRunsAtLeastOnce(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	runner := NewRunner(logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ran := make(chan struct{}, 1)
	done := make(chan struct{})
	ticks := make(chan time.Time, 1)

	go func() {
		runTickerLoop(ctx, ticks, func(ctx context.Context) error {
			err := runner.RunOnce(ctx)
			select {
			case ran <- struct{}{}:
			default:
			}
			cancel()
			return err
		})
		close(done)
	}()

	ticks <- time.Now()
	waitForSignal(t, ran, "runner to execute at least once")
	waitForSignal(t, done, "ticker loop to stop after cancellation")

	if !strings.Contains(buf.String(), "tick") {
		t.Fatalf("expected log output to contain tick, got %q", buf.String())
	}
}

func TestTickerLoopStopsWithoutExtraRunAfterCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	ticks := make(chan time.Time, 2)
	calls := make(chan struct{}, 2)

	go func() {
		runTickerLoop(ctx, ticks, func(context.Context) error {
			calls <- struct{}{}
			return nil
		})
		close(done)
	}()

	cancel()
	ticks <- time.Now()
	waitForSignal(t, done, "ticker loop to stop after cancellation")

	select {
	case <-calls:
		t.Fatal("expected no callback run after cancellation")
	default:
	}
}

func TestTickerLoopLogsCallbackErrors(t *testing.T) {
	var buf bytes.Buffer
	oldWriter := log.Writer()
	oldFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	defer log.SetOutput(oldWriter)
	defer log.SetFlags(oldFlags)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	expectedErr := errors.New("boom")
	ticks := make(chan time.Time, 1)

	go func() {
		runTickerLoop(ctx, ticks, func(context.Context) error {
			cancel()
			return expectedErr
		})
		close(done)
	}()

	ticks <- time.Now()
	waitForSignal(t, done, "ticker loop to stop after callback error")

	if !strings.Contains(buf.String(), "task ticker callback error: boom") {
		t.Fatalf("expected callback error log, got %q", buf.String())
	}
}

func waitForSignal(t *testing.T, ch <-chan struct{}, description string) {
	t.Helper()

	select {
	case <-ch:
	case <-time.After(250 * time.Millisecond):
		t.Fatalf("timed out waiting for %s", description)
	}
}
