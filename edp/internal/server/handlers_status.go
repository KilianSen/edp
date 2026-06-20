package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"edp/internal/store"
)

// containerState summarizes the live Docker state for an env's containers.
func (s *Server) containerState(ctx context.Context, envID int64) (state, info string) {
	infos, err := s.dk.ListByEnv(ctx, envID)
	if err != nil {
		return "unknown", ""
	}
	switch len(infos) {
	case 0:
		return "none", ""
	case 1:
		return infos[0].State, infos[0].Status
	default:
		running := 0
		for _, c := range infos {
			if c.State == "running" {
				running++
			}
		}
		return "stack", fmt.Sprintf("%d/%d up", running, len(infos))
	}
}

type statusItem struct {
	ID    int64  `json:"id"`
	State string `json:"state"`
	Info  string `json:"info"`
}

// overviewItem decorates an environment with the live runtime info the dashboard
// needs in one shot: container state, the latest deployment, and the ETA derived
// from past successful deploys (so the SPA doesn't N+1 the deployments endpoint).
type overviewItem struct {
	Env            *store.Environment `json:"env"`
	ContainerState string             `json:"container_state"`
	ContainerInfo  string             `json:"container_info"`
	Last           *store.Deployment  `json:"last"`
	EstimateMs     int64              `json:"estimate_ms"`
}

// apiOverview returns the decorated environment list backing the dashboard.
func (s *Server) apiOverview(w http.ResponseWriter, r *http.Request) {
	envs, err := s.st.ListEnvironments(r.Context())
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	items := make([]overviewItem, 0, len(envs))
	for _, e := range envs {
		st, info := s.containerState(r.Context(), e.ID)
		last, _ := s.st.LatestDeployment(r.Context(), e.ID)
		items = append(items, overviewItem{
			Env:            e,
			ContainerState: st,
			ContainerInfo:  info,
			Last:           last,
			EstimateMs:     s.st.EstimateDurationMs(r.Context(), e.ID),
		})
	}
	writeJSON(w, 200, items)
}

// apiStatus returns a snapshot of every env's live container state.
func (s *Server) apiStatus(w http.ResponseWriter, r *http.Request) {
	envs, err := s.st.ListEnvironments(r.Context())
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	items := make([]statusItem, 0, len(envs))
	for _, e := range envs {
		st, info := s.containerState(r.Context(), e.ID)
		items = append(items, statusItem{ID: e.ID, State: st, Info: info})
	}
	writeJSON(w, 200, items)
}

// apiEventsStream pushes a "changed" SSE event (carrying the env id) whenever a
// managed container's state changes, so dashboards update live.
func (s *Server) apiEventsStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch, cancel := s.hub.Subscribe()
	defer cancel()

	// initial nudge so a freshly-connected client pulls a snapshot
	fmt.Fprint(w, "event: changed\ndata: \n\n")
	flusher.Flush()

	keepalive := time.NewTicker(25 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-keepalive.C:
			fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		case id, open := <-ch:
			if !open {
				return
			}
			fmt.Fprintf(w, "event: changed\ndata: %s\n\n", id)
			flusher.Flush()
		}
	}
}
