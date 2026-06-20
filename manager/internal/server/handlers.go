package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"edp-manager/internal/aggregate"
	"edp-manager/internal/edpclient"
	"edp-manager/internal/store"
)

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// ---- instance registry CRUD ----

func (s *Server) apiListInstances(w http.ResponseWriter, r *http.Request) {
	insts, err := s.st.ListInstances(r.Context())
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, insts)
}

func (s *Server) apiCreateInstance(w http.ResponseWriter, r *http.Request) {
	var in store.Instance
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if in.Label == "" || in.BaseURL == "" {
		writeJSON(w, 400, map[string]string{"error": "label and base_url are required"})
		return
	}
	if err := s.st.CreateInstance(r.Context(), &in); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 201, in)
}

func (s *Server) apiGetInstance(w http.ResponseWriter, r *http.Request) {
	inst := s.loadInstance(w, r)
	if inst == nil {
		return
	}
	writeJSON(w, 200, inst)
}

func (s *Server) apiUpdateInstance(w http.ResponseWriter, r *http.Request) {
	inst := s.loadInstance(w, r)
	if inst == nil {
		return
	}
	// Decode onto the existing instance: an omitted api_token leaves it unchanged.
	if err := json.NewDecoder(r.Body).Decode(inst); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if err := s.st.UpdateInstance(r.Context(), inst); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, inst)
}

func (s *Server) apiDeleteInstance(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if err := s.st.DeleteInstance(r.Context(), id); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "deleted"})
}

// apiTestInstance pings an instance to confirm it's reachable and the token works.
func (s *Server) apiTestInstance(w http.ResponseWriter, r *http.Request) {
	inst := s.loadInstance(w, r)
	if inst == nil {
		return
	}
	if err := edpclient.New(inst.BaseURL, inst.APIToken).Ping(r.Context()); err != nil {
		writeJSON(w, 200, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) loadInstance(w http.ResponseWriter, r *http.Request) *store.Instance {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return nil
	}
	inst, err := s.st.GetInstance(r.Context(), id)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return nil
	}
	if inst == nil {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return nil
	}
	return inst
}

// ---- aggregate (fan-out) ----

// fanout runs a GET against every instance's <path> and returns the merged,
// instance-tagged result with per-instance errors.
func (s *Server) fanout(w http.ResponseWriter, r *http.Request, path string) {
	insts, err := s.st.ListInstances(r.Context())
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	res := aggregate.Fanout(r.Context(), insts, path, time.Duration(s.cfg.FanoutTimeoutMs)*time.Millisecond)
	writeJSON(w, 200, res)
}

func (s *Server) apiEnvironments(w http.ResponseWriter, r *http.Request) {
	s.fanout(w, r, "/api/environments")
}

func (s *Server) apiOverview(w http.ResponseWriter, r *http.Request) {
	s.fanout(w, r, "/api/overview")
}

func (s *Server) apiStatus(w http.ResponseWriter, r *http.Request) {
	s.fanout(w, r, "/api/status")
}

// apiSummary rolls every instance up into a fleet health overview: reachable
// up/down per instance plus environment counts by status, and fleet totals.
func (s *Server) apiSummary(w http.ResponseWriter, r *http.Request) {
	insts, err := s.st.ListInstances(r.Context())
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	// Summarize off the flat env list (status is top-level there); /api/overview
	// nests the env under an "env" key, which would make status counting miss.
	res := aggregate.Summarize(r.Context(), insts, "/api/environments", time.Duration(s.cfg.FanoutTimeoutMs)*time.Millisecond)
	writeJSON(w, 200, res)
}

// ---- per-instance pass-through ----

// proxyToInstance forwards /api/instances/{id}/edp/<path> to that instance's
// /<path>, injecting its token. This is the drill-into-one-instance channel for
// per-env detail, actions (deploy), and SSE log streaming — things a merged
// view can't express. SSE flows through because the proxy flushes immediately.
func (s *Server) proxyToInstance(w http.ResponseWriter, r *http.Request) {
	inst := s.loadInstance(w, r)
	if inst == nil {
		return
	}
	stripPrefix := "/api/instances/" + strings.TrimPrefix(r.PathValue("id"), "/") + "/edp"
	rp, err := edpclient.New(inst.BaseURL, inst.APIToken).ReverseProxy(stripPrefix)
	if err != nil {
		writeJSON(w, 502, map[string]string{"error": err.Error()})
		return
	}
	rp.ServeHTTP(w, r)
}
