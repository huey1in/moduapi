// Package embedding is an experimental public composition API, not a stable API.
// The caller owns the HTTP listener, TLS, admission and graceful HTTP draining.
package embedding

import (
	"context"
	"net/http"
	"sync"

	"github.com/huey1in/moduapi/core/config"
	"github.com/huey1in/moduapi/internal/app"
)

// Runtime owns the installed component processes. Do not copy it after Start.
type Runtime struct {
	app    *app.App
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex
	closed bool
	active sync.WaitGroup
	once   sync.Once
}

// Start validates configuration and starts/negotiates all discovered bundles.
// Configuration is immutable after Start. ctx bounds startup, not runtime life;
// the owner must call Close even if ctx is later cancelled.
func Start(ctx context.Context, c config.Config) (*Runtime, error) {
	a, err := app.Start(ctx, c, nil)
	if err != nil {
		return nil, err
	}
	base, cancel := context.WithCancel(context.Background())
	return &Runtime{app: a, ctx: base, cancel: cancel}, nil
}

func (r *Runtime) Ready() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return !r.closed && r.app.Registry.Ready()
}

// ServeHTTP composes into the host's HTTP server without starting a listener.
func (r *Runtime) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		http.Error(w, "kernel closed", 503)
		return
	}
	r.active.Add(1)
	r.mu.Unlock()
	defer r.active.Done()
	ctx, cancel := context.WithCancel(req.Context())
	stop := context.AfterFunc(r.ctx, cancel)
	defer stop()
	defer cancel()
	r.app.Gateway.ServeHTTP(w, req.WithContext(ctx))
}

// Close rejects admission, cancels active requests, stops and reaps components.
// Call http.Server.Shutdown first if active HTTP requests should finish normally.
// Like process.Stop, Close waits for process reaping even when ctx has expired.
func (r *Runtime) Close(ctx context.Context) {
	r.once.Do(func() {
		r.mu.Lock()
		r.closed = true
		r.cancel()
		r.mu.Unlock()
		r.app.Manager.Close(ctx)
		r.active.Wait()
	})
}
