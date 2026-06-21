package app

import (
	"context"
	"net"
	"net/http"
	"sync"
)

type healthServer struct {
	server *http.Server

	mu       sync.RWMutex
	listener net.Listener
	addr     string
}

func newHealthServer(addr string, cancel context.CancelFunc) *healthServer {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/shutdown", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		cancel()
	})
	return &healthServer{
		server: &http.Server{
			Addr:    addr,
			Handler: mux,
		},
	}
}

func (h *healthServer) start() error {
	ln, err := net.Listen("tcp", h.server.Addr)
	if err != nil {
		return err
	}

	h.mu.Lock()
	h.listener = ln
	h.addr = ln.Addr().String()
	h.mu.Unlock()

	go func() {
		_ = h.server.Serve(ln)
	}()

	return nil
}

func (h *healthServer) shutdown(ctx context.Context) error {
	return h.server.Shutdown(ctx)
}

func (h *healthServer) address() string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.addr
}
