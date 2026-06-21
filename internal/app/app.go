package app

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/chenxian/learning-go-daemon/internal/state"
	"github.com/chenxian/learning-go-daemon/internal/task"
)

type Config struct {
	Address    string
	StateDir   string
	Interval   time.Duration
	Foreground bool
}

type App struct {
	cfg    Config
	logger *log.Logger

	mu     sync.RWMutex
	health *healthServer
}

func DefaultConfig(stateDir string) Config {
	return Config{
		Address:    "127.0.0.1:19514",
		StateDir:   stateDir,
		Interval:   5 * time.Second,
		Foreground: false,
	}
}

func New(cfg Config, logger *log.Logger) *App {
	return &App{cfg: cfg, logger: logger}
}

func (a *App) RunForeground(parent context.Context) error {
	ctx, cancel := context.WithCancel(parent)
	stopSignals := watchSignals(cancel)
	defer cancel()
	defer stopSignals()

	if err := os.MkdirAll(a.cfg.StateDir, 0o755); err != nil {
		return err
	}

	server := newHealthServer(a.cfg.Address, cancel)
	if err := server.start(); err != nil {
		return err
	}
	a.setHealthServer(server)

	pidPath := state.PIDPath(a.cfg.StateDir)
	if err := state.WritePID(pidPath, os.Getpid()); err != nil {
		a.setHealthServer(nil)
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
		defer shutdownCancel()
		_ = server.shutdown(shutdownCtx)
		return err
	}
	if err := os.WriteFile(healthAddressPath(a.cfg.StateDir), []byte(server.address()), 0o644); err != nil {
		_ = state.RemovePID(pidPath)
		a.setHealthServer(nil)
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
		defer shutdownCancel()
		_ = server.shutdown(shutdownCtx)
		return err
	}
	defer func() {
		_ = state.RemovePID(pidPath)
	}()
	defer func() {
		_ = os.Remove(healthAddressPath(a.cfg.StateDir))
	}()
	defer func() {
		a.setHealthServer(nil)
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
		defer shutdownCancel()
		_ = server.shutdown(shutdownCtx)
	}()

	runner := task.NewRunner(a.logger)
	go task.StartTickerLoop(ctx, a.cfg.Interval, runner.RunOnce)

	<-ctx.Done()
	return nil
}

func (a *App) setHealthServer(server *healthServer) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.health = server
}

func (a *App) healthAddress() string {
	a.mu.RLock()
	server := a.health
	a.mu.RUnlock()
	if server == nil {
		return ""
	}

	return server.address()
}

func healthAddressPath(stateDir string) string {
	return filepath.Join(stateDir, "mydaemon.addr")
}
