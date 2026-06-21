package task

import (
	"context"
	"log"
	"time"
)

func StartTickerLoop(ctx context.Context, interval time.Duration, fn func(context.Context) error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	runTickerLoop(ctx, ticker.C, fn)
}

func runTickerLoop(ctx context.Context, ticks <-chan time.Time, fn func(context.Context) error) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticks:
			if ctx.Err() != nil {
				return
			}
			if err := fn(ctx); err != nil {
				log.Printf("task ticker callback error: %v", err)
			}
		}
	}
}
