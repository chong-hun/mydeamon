package app

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestBackgroundArgsIncludeForegroundFlag(t *testing.T) {
	args := buildBackgroundArgs([]string{"start"})
	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(args))
	}
	if args[0] != "start" || args[1] != "--foreground" {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestBackgroundArgsDoNotMutateCallerBackingArray(t *testing.T) {
	backing := []string{"start", "keep", "tail"}
	original := backing[:1]

	args := buildBackgroundArgs(original)

	if len(original) != 1 {
		t.Fatalf("expected original len to remain 1, got %d", len(original))
	}
	if backing[1] != "keep" {
		t.Fatalf("expected backing array to remain unchanged, got %#v", backing)
	}
	if args[0] != "start" || args[1] != "--foreground" {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestWaitForBackgroundStartReturnsWhenHealthy(t *testing.T) {
	poll := make(chan time.Time, 1)
	poll <- time.Now()
	calls := 0

	err := waitForBackgroundStart(
		context.Background(),
		"127.0.0.1:19514",
		func(target string) bool {
			calls++
			return calls == 2 && target == "127.0.0.1:19514"
		},
		make(chan error),
		poll,
	)
	if err != nil {
		t.Fatalf("waitForBackgroundStart returned error after poll: %v", err)
	}
}

func TestWaitForBackgroundStartReturnsErrorWhenProcessExits(t *testing.T) {
	processDone := make(chan error, 1)
	processDone <- context.Canceled

	err := waitForBackgroundStart(
		context.Background(),
		"127.0.0.1:19514",
		func(string) bool { return false },
		processDone,
		make(chan time.Time),
	)
	if err == nil {
		t.Fatal("expected error when process exits early")
	}
	if !strings.Contains(err.Error(), "exited before becoming healthy") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWaitForBackgroundStartReturnsErrorWhenContextExpires(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := waitForBackgroundStart(
		ctx,
		"127.0.0.1:19514",
		func(string) bool { return false },
		make(chan error),
		make(chan time.Time),
	)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "did not become healthy") {
		t.Fatalf("unexpected error: %v", err)
	}
}
