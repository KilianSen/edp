// Package logbus is a tiny in-memory pub/sub for streaming deployment log
// lines to live subscribers (the dashboard's SSE connections). It does not
// persist anything; the store keeps the durable copy.
package logbus

import "sync"

type Bus struct {
	mu   sync.Mutex
	subs map[int64]map[chan string]struct{}
}

func New() *Bus {
	return &Bus{subs: make(map[int64]map[chan string]struct{})}
}

// Subscribe returns a channel that receives log chunks for the given deployment
// ID, plus a cancel func the caller must invoke when done.
func (b *Bus) Subscribe(deployID int64) (<-chan string, func()) {
	ch := make(chan string, 256)
	b.mu.Lock()
	if b.subs[deployID] == nil {
		b.subs[deployID] = make(map[chan string]struct{})
	}
	b.subs[deployID][ch] = struct{}{}
	b.mu.Unlock()

	cancel := func() {
		b.mu.Lock()
		if m := b.subs[deployID]; m != nil {
			if _, ok := m[ch]; ok {
				delete(m, ch)
				close(ch)
			}
			if len(m) == 0 {
				delete(b.subs, deployID)
			}
		}
		b.mu.Unlock()
	}
	return ch, cancel
}

// Publish sends a chunk to all subscribers of a deployment. Non-blocking: if a
// subscriber's buffer is full the chunk is dropped for that subscriber (it can
// reload the full log from the store).
func (b *Bus) Publish(deployID int64, chunk string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs[deployID] {
		select {
		case ch <- chunk:
		default:
		}
	}
}

// Close signals all subscribers of a deployment that the stream has ended by
// closing their channels.
func (b *Bus) Close(deployID int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs[deployID] {
		close(ch)
	}
	delete(b.subs, deployID)
}
