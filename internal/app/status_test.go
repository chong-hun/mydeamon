package app

import (
	"context"
	"io"
	"log"
	"net/http"
	"testing"
	"time"

	"github.com/chenxian/learning-go-daemon/internal/state"
)

func TestStatusReportsStalePIDWhenProcessIsDown(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	cfg := Config{
		Address:    "127.0.0.1:19525",
		StateDir:   t.TempDir(),
		Interval:   time.Second,
		Foreground: true,
	}

	if err := state.WritePID(state.PIDPath(cfg.StateDir), 99999); err != nil {
		t.Fatalf("WritePID returned error: %v", err)
	}

	a := New(cfg, logger)
	status, err := a.Status()
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if status != "stale pid file" {
		t.Fatalf("expected stale pid file, got %q", status)
	}
}

func TestStatusReportsRunningForSeparateClientWithDynamicAddress(t *testing.T) {
	liveApp, done := startTestApp(t)
	clientApp := newClientApp(liveApp.cfg, liveApp.logger)

	status, err := clientApp.Status()
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if status != "running" {
		t.Fatalf("expected running, got %q", status)
	}

	shutdownTestApp(t, liveApp, done)
}

func TestStopShutsDownLiveDaemonForSeparateClientWithDynamicAddress(t *testing.T) {
	liveApp, done := startTestApp(t)
	clientApp := newClientApp(liveApp.cfg, liveApp.logger)

	if err := clientApp.Stop(); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("RunForeground returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("RunForeground did not exit after Stop")
	}

	status, err := clientApp.Status()
	if err != nil {
		t.Fatalf("Status returned error after Stop: %v", err)
	}
	if status != "stopped" {
		t.Fatalf("expected stopped after Stop, got %q", status)
	}

	_, ok, err := state.ReadPID(state.PIDPath(clientApp.cfg.StateDir))
	if err != nil {
		t.Fatalf("ReadPID returned error: %v", err)
	}
	if ok {
		t.Fatal("expected PID file to be removed after Stop")
	}
}

func TestStopDoesNotRemovePIDWhenShutdownRequestFails(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	cfg := Config{
		Address:    "127.0.0.1:19526",
		StateDir:   t.TempDir(),
		Interval:   time.Second,
		Foreground: true,
	}

	pidPath := state.PIDPath(cfg.StateDir)
	if err := state.WritePID(pidPath, 99999); err != nil {
		t.Fatalf("WritePID returned error: %v", err)
	}

	a := New(cfg, logger)
	if err := a.Stop(); err == nil {
		t.Fatal("expected Stop to return an error")
	}

	_, ok, err := state.ReadPID(pidPath)
	if err != nil {
		t.Fatalf("ReadPID returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected PID file to remain after failed Stop")
	}
}

func startTestApp(t *testing.T) (*App, <-chan error) {
	t.Helper()

	logger := log.New(io.Discard, "", 0)
	cfg := Config{
		Address:    "127.0.0.1:0",
		StateDir:   t.TempDir(),
		Interval:   50 * time.Millisecond,
		Foreground: true,
	}

	a := New(cfg, logger)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	done := make(chan error, 1)
	go func() {
		done <- a.RunForeground(ctx)
	}()

	waitForHealth(t, a)
	return a, done
}

func newClientApp(cfg Config, logger *log.Logger) *App {
	return New(cfg, logger)
}

func waitForHealth(t *testing.T, a *App) {
	t.Helper()

	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		address := a.healthAddress()
		if address == "" {
			time.Sleep(20 * time.Millisecond)
			continue
		}

		resp, err := client.Get("http://" + address + "/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			_ = resp.Body.Close()
			return
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatal("health server did not become ready")
}

func shutdownTestApp(t *testing.T, a *App, done <-chan error) {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, "http://"+a.healthAddress()+"/shutdown", nil)
	if err != nil {
		t.Fatalf("shutdown request build failed: %v", err)
	}

	resp, err := (&http.Client{Timeout: time.Second}).Do(req)
	if err != nil {
		t.Fatalf("shutdown request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		t.Fatalf("shutdown status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	_ = resp.Body.Close()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("RunForeground returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("RunForeground did not exit after shutdown")
	}
}
