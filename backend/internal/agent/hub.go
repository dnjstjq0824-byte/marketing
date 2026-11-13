package agent

import (
	"sync"
	"time"
)

// Hub wakes long-polling claim waiters when any job is created (D26).
// Notify never blocks; Wait sleeps without holding any DB connection.
type Hub struct {
	mu sync.Mutex
	ch chan struct{}
}

// NewHub builds a hub.
func NewHub() *Hub { return &Hub{ch: make(chan struct{})} }

// Notify wakes all current waiters.
func (h *Hub) Notify() {
	h.mu.Lock()
	defer h.mu.Unlock()
	close(h.ch)
	h.ch = make(chan struct{})
}

// Wait sleeps until a Notify or the timeout, whichever comes first.
func (h *Hub) Wait(timeout time.Duration) {
	h.mu.Lock()
	ch := h.ch
	h.mu.Unlock()
	if timeout <= 0 {
		return
	}
	select {
	case <-ch:
	case <-time.After(timeout):
	}
}
