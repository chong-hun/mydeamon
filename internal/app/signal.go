package app

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func watchSignals(cancel context.CancelFunc) func() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		cancel()
	}()
	return func() {
		signal.Stop(ch)
		close(ch)
	}
}
