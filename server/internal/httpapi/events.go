package httpapi

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

type eventHub struct {
	mu          sync.Mutex
	subscribers map[chan string]struct{}
}

func newEventHub() *eventHub {
	return &eventHub{subscribers: make(map[chan string]struct{})}
}

func (h *eventHub) subscribe() (<-chan string, func()) {
	updates := make(chan string, 1)
	h.mu.Lock()
	h.subscribers[updates] = struct{}{}
	h.mu.Unlock()

	return updates, func() {
		h.mu.Lock()
		delete(h.subscribers, updates)
		h.mu.Unlock()
	}
}

func (h *eventHub) publish(event string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for subscriber := range h.subscribers {
		select {
		case subscriber <- event:
		default:
		}
	}
}

func (a *App) handleEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	controller := http.NewResponseController(w)
	writeEvent := func(payload string) bool {
		_ = controller.SetWriteDeadline(time.Now().Add(30 * time.Second))
		if _, err := fmt.Fprint(w, payload); err != nil {
			return false
		}
		return controller.Flush() == nil
	}
	if !writeEvent("event: ready\ndata: {}\n\n") {
		return
	}

	updates, unsubscribe := a.events.subscribe()
	defer unsubscribe()
	keepAlive := time.NewTicker(20 * time.Second)
	defer keepAlive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-updates:
			if !writeEvent(fmt.Sprintf("event: %s\ndata: {}\n\n", event)) {
				return
			}
		case <-keepAlive.C:
			if !writeEvent(": keepalive\n\n") {
				return
			}
		}
	}
}
