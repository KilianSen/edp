package server

import (
	"fmt"
	"net/http"
	"strings"
)

// apiStreamLogs streams a deployment's log over Server-Sent Events. It first
// replays the log already stored, then tails live chunks from the bus until the
// deployment finishes (bus channel closed) or the client disconnects.
func (s *Server) apiStreamLogs(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	dep, err := s.st.GetDeployment(r.Context(), id)
	if err != nil || dep == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Subscribe before replaying stored log to avoid missing chunks in the gap.
	ch, cancel := s.bus.Subscribe(id)
	defer cancel()

	if dep.Log != "" {
		writeSSE(w, dep.Log)
		flusher.Flush()
	}

	// If already finished, just send a done marker.
	if dep.Status != "running" && dep.Status != "queued" {
		fmt.Fprintf(w, "event: done\ndata: %s\n\n", dep.Status)
		flusher.Flush()
		return
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case chunk, open := <-ch:
			if !open {
				// reload final status
				if d, _ := s.st.GetDeployment(r.Context(), id); d != nil {
					fmt.Fprintf(w, "event: done\ndata: %s\n\n", d.Status)
				} else {
					fmt.Fprint(w, "event: done\ndata: ended\n\n")
				}
				flusher.Flush()
				return
			}
			writeSSE(w, chunk)
			flusher.Flush()
		}
	}
}

// writeSSE emits text as one or more SSE "data:" lines (newlines split records).
func writeSSE(w http.ResponseWriter, text string) {
	for _, line := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		fmt.Fprintf(w, "data: %s\n", line)
	}
	fmt.Fprint(w, "\n")
}
