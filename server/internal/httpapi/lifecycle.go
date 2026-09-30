package httpapi

import (
	"context"
	"net/http"
	"time"
)

type managedHandler struct {
	http.Handler
	app *App
}

// BeginShutdown removes readiness and cancels long-lived streams and workers.
func (h *managedHandler) BeginShutdown() {
	h.app.workerMu.Lock()
	defer h.app.workerMu.Unlock()
	h.app.stopping = true
	h.app.lifecycleCancel()
}

// Shutdown waits for workers before the caller closes the database.
func (h *managedHandler) Shutdown(ctx context.Context) error {
	h.BeginShutdown()
	done := make(chan struct{})
	go func() { h.app.workers.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (a *App) startWorker(work func(context.Context)) {
	a.workerMu.Lock()
	defer a.workerMu.Unlock()
	if a.stopping {
		return
	}
	a.workers.Add(1)
	go func() { defer a.workers.Done(); work(a.lifecycleContext) }()
}

func (a *App) handleReadiness(w http.ResponseWriter, r *http.Request) {
	a.workerMu.Lock()
	stopping := a.stopping
	a.workerMu.Unlock()
	if stopping {
		writeError(w, http.StatusServiceUnavailable, "server stopping")
		return
	}
	// Startup validates migrations and recovery keys before creating this handler.
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := a.store.Ping(ctx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
