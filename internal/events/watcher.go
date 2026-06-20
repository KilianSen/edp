// Package events bridges the Docker event stream into the status hub so the
// dashboard learns about container state changes the moment they happen, instead
// of only when a page is rendered.
package events

import (
	"context"
	"log"
	"time"

	"edp/internal/docker"
	"edp/internal/statushub"
)

type Watcher struct {
	dk  *docker.Client
	hub *statushub.Hub
}

func New(dk *docker.Client, hub *statushub.Hub) *Watcher {
	return &Watcher{dk: dk, hub: hub}
}

// Run streams Docker events until ctx is cancelled, reconnecting on error (the
// daemon may restart, or `docker events` may exit). Each container event tagged
// with an edp.env label is broadcast as a change signal.
func (w *Watcher) Run(ctx context.Context) {
	for ctx.Err() == nil {
		err := w.dk.StreamEvents(ctx, func(ev docker.Event) {
			if id := ev.EnvID(); id != "" {
				w.hub.Broadcast(id)
			}
		})
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			log.Printf("events: stream ended (%v), reconnecting", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}
