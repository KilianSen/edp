// Package statushub is a single-topic pub/sub used to push "something changed"
// signals (an env id) to dashboard SSE connections.
package statushub

import "sync"

type Hub struct {
	mu   sync.Mutex
	subs map[chan string]struct{}
}

func New() *Hub { return &Hub{subs: make(map[chan string]struct{})} }

func (h *Hub) Subscribe() (<-chan string, func()) {
	ch := make(chan string, 64)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		if _, ok := h.subs[ch]; ok {
			delete(h.subs, ch)
			close(ch)
		}
		h.mu.Unlock()
	}
}

// Broadcast delivers msg (an env id, or "" for "refresh everything") to all
// subscribers, dropping it for any slow subscriber rather than blocking.
func (h *Hub) Broadcast(msg string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- msg:
		default:
		}
	}
}
