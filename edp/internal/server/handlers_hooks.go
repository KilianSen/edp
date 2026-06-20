package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"edp/internal/store"
)

// ---- timed hooks API ----

func (s *Server) apiListHooks(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	hooks, err := s.st.ListTimedHooks(r.Context(), id)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, hooks)
}

func (s *Server) apiCreateHook(w http.ResponseWriter, r *http.Request) {
	envID, err := parseID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	h := &store.TimedHook{EnvID: envID, Enabled: true}
	if err := bindHook(r, h); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if h.Name == "" {
		writeJSON(w, 400, map[string]string{"error": "name is required"})
		return
	}
	if err := s.st.CreateTimedHook(r.Context(), h); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 201, h)
}

func (s *Server) apiGetHook(w http.ResponseWriter, r *http.Request) {
	h := s.loadHook(w, r)
	if h == nil {
		return
	}
	writeJSON(w, 200, h)
}

func (s *Server) apiUpdateHook(w http.ResponseWriter, r *http.Request) {
	h := s.loadHook(w, r)
	if h == nil {
		return
	}
	if err := bindHook(r, h); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if err := s.st.UpdateTimedHook(r.Context(), h); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, h)
}

func (s *Server) apiDeleteHook(w http.ResponseWriter, r *http.Request) {
	h := s.loadHook(w, r)
	if h == nil {
		return
	}
	if err := s.st.DeleteTimedHook(r.Context(), h.ID); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "deleted"})
}

func (s *Server) apiRunHook(w http.ResponseWriter, r *http.Request) {
	h := s.loadHook(w, r)
	if h == nil {
		return
	}
	runID, err := s.hooks.TriggerHook(r.Context(), h.ID, store.TriggerManual)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 202, map[string]int64{"run_id": runID})
}

func (s *Server) apiListHookRuns(w http.ResponseWriter, r *http.Request) {
	h := s.loadHook(w, r)
	if h == nil {
		return
	}
	runs, err := s.st.ListHookRuns(r.Context(), h.ID, 25)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, runs)
}

func (s *Server) apiGetHookRun(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	run, err := s.st.GetHookRun(r.Context(), id)
	if err != nil || run == nil {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, 200, run)
}

// loadHook fetches the hook named by the {id} path value, writing an error
// response and returning nil if absent.
func (s *Server) loadHook(w http.ResponseWriter, r *http.Request) *store.TimedHook {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return nil
	}
	h, err := s.st.GetTimedHook(r.Context(), id)
	if err != nil || h == nil {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return nil
	}
	return h
}

func bindHook(r *http.Request, h *store.TimedHook) error {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return json.NewDecoder(r.Body).Decode(h)
	}
	if err := r.ParseForm(); err != nil {
		return err
	}
	f := r.PostForm
	set(&h.Name, f, "name")
	set(&h.Schedule, f, "schedule")
	set(&h.Script, f, "script")
	h.Enabled = f.Get("enabled") != ""
	return nil
}

// ---- SSE: hook run log ----

func (s *Server) apiStreamHookLog(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	run, err := s.st.GetHookRun(r.Context(), id)
	if err != nil || run == nil {
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

	ch, cancel := s.hooks.Bus().Subscribe(id)
	defer cancel()

	if run.Log != "" {
		writeSSE(w, run.Log)
		flusher.Flush()
	}
	if run.Status != store.StatusRunning && run.Status != store.StatusQueued {
		fmt.Fprintf(w, "event: done\ndata: %s\n\n", run.Status)
		flusher.Flush()
		return
	}

	keepalive := time.NewTicker(25 * time.Second)
	defer keepalive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-keepalive.C:
			fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		case chunk, open := <-ch:
			if !open {
				if d, _ := s.st.GetHookRun(r.Context(), id); d != nil {
					fmt.Fprintf(w, "event: done\ndata: %s\n\n", d.Status)
				}
				flusher.Flush()
				return
			}
			writeSSE(w, chunk)
			flusher.Flush()
		}
	}
}

// ---- inbound webhook ----

// hookDeploy is the inbound webhook: POST /hooks/{id}?token=...
func (s *Server) hookDeploy(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	e, err := s.st.GetEnvironment(r.Context(), id)
	if err != nil || e == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	token := r.URL.Query().Get("token")
	if token == "" {
		token = r.Header.Get("X-Edp-Token")
	}
	if e.WebhookToken == "" || token != e.WebhookToken {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	reason := firstNonEmptyStr(r.URL.Query().Get("reason"), r.Header.Get("X-Edp-Reason"), "webhook")
	depID, err := s.engine.Trigger(r.Context(), id, store.TriggerWebhook, reason)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]int64{"deployment_id": depID})
}
