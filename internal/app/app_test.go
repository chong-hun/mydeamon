package app

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestHealthServerStartReturnsBindError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	server := newHealthServer(ln.Addr().String(), func() {})
	if err := server.start(); err == nil {
		t.Fatal("expected bind error, got nil")
	}
}

func TestHealthServerRespondsAndShutdownCancelsContext(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	cfg := Config{
		Address:    "127.0.0.1:0",
		StateDir:   t.TempDir(),
		Interval:   50 * time.Millisecond,
		Foreground: true,
	}

	a := New(cfg, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- a.RunForeground(ctx)
	}()

	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(2 * time.Second)
	var address string
	for time.Now().Before(deadline) {
		address = a.healthAddress()
		if address == "" {
			time.Sleep(20 * time.Millisecond)
			continue
		}

		resp, err := client.Get("http://" + address + "/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			_ = resp.Body.Close()
			break
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		time.Sleep(20 * time.Millisecond)
	}
	if address == "" {
		t.Fatal("health server did not publish an address")
	}

	resp, err := client.Get("http://" + address + "/health")
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		t.Fatalf("health status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	_ = resp.Body.Close()

	req, err := http.NewRequest(http.MethodPost, "http://"+address+"/shutdown", nil)
	if err != nil {
		t.Fatalf("shutdown request build failed: %v", err)
	}
	resp, err = client.Do(req)
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
