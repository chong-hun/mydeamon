package task

import (
	"context"
	"log"
	"os"
	"sync/atomic"
	"time"
)

type Runner struct {
	logger  *log.Logger
	counter uint64
}

func NewRunner(logger *log.Logger) *Runner {
	return &Runner{logger: logger}
}

func (r *Runner) RunOnce(_ context.Context) error {
	count := atomic.AddUint64(&r.counter, 1)
	r.logger.Printf("tick count=%d pid=%d at=%s", count, os.Getpid(), time.Now().Format(time.RFC3339))
	return nil
}
