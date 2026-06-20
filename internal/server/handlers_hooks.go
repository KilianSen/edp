package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"edp/internal/store"
)

// ---- API ----

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
	if wantsHTML(r) {
		http.Redirect(w, r, "/env/"+itoa(envID), http.StatusSeeOther)
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
	if wantsHTML(r) {
		http.Redirect(w, r, "/timed-hooks/"+itoa(h.ID), http.StatusSeeOther)
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
	if wantsHTML(r) {
		http.Redirect(w, r, "/env/"+itoa(h.EnvID), http.StatusSeeOther)
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
	if isHTMX(r) {
		w.Header().Set("HX-Trigger", "refresh")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if wantsHTML(r) {
		http.Redirect(w, r, "/timed-hooks/"+itoa(h.ID)+"?run="+itoa(runID), http.StatusSeeOther)
		return
	}
	writeJSON(w, 202, map[string]int64{"run_id": runID})
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

// ---- pages ----

func (s *Server) pageHookForm(w http.ResponseWriter, r *http.Request) {
	// /env/{id}/hooks/new  -> create;  /timed-hooks/{id}/edit -> edit
	if strings.Contains(r.URL.Path, "/env/") {
		envID, err := parseID(r.PathValue("id"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		env, err := s.st.GetEnvironment(r.Context(), envID)
		if err != nil || env == nil {
			http.NotFound(w, r)
			return
		}
		s.render(w, "hook_form.html", map[string]any{
			"Hook":    &store.TimedHook{EnvID: envID, Enabled: true},
			"EnvName": env.Name,
			"Editing": false,
		})
		return
	}
	h := s.loadHookPage(w, r)
	if h == nil {
		return
	}
	env, _ := s.st.GetEnvironment(r.Context(), h.EnvID)
	name := ""
	if env != nil {
		name = env.Name
	}
	s.render(w, "hook_form.html", map[string]any{"Hook": h, "EnvName": name, "Editing": true})
}

func (s *Server) pageHookDetail(w http.ResponseWriter, r *http.Request) {
	h := s.loadHookPage(w, r)
	if h == nil {
		return
	}
	runs, _ := s.st.ListHookRuns(r.Context(), h.ID, 25)
	env, _ := s.st.GetEnvironment(r.Context(), h.EnvID)
	envName := ""
	if env != nil {
		envName = env.Name
	}

	var focus *store.HookRun
	if v := r.URL.Query().Get("run"); v != "" {
		if rid, err := parseID(v); err == nil {
			focus, _ = s.st.GetHookRun(r.Context(), rid)
		}
	} else if len(runs) > 0 {
		focus = runs[0]
	}

	s.render(w, "hook_detail.html", map[string]any{
		"Hook":    h,
		"EnvName": envName,
		"Runs":    runs,
		"Focus":   focus,
	})
}

func (s *Server) loadHookPage(w http.ResponseWriter, r *http.Request) *store.TimedHook {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return nil
	}
	h, err := s.st.GetTimedHook(r.Context(), id)
	if err != nil || h == nil {
		http.NotFound(w, r)
		return nil
	}
	return h
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
