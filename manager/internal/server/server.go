// Package server exposes edp-manager's HTTP surface: a manager-level auth +
// instance-registry API, aggregate (fan-out) read endpoints across all edp
// instances, and a per-instance pass-through proxy. When EDPM_UI_DIR is set it
// also serves the built dashboard (ui/) at "/". The JSON API is guarded by a
// Bearer token; per-instance edp tokens are held server-side, never in the browser.
package server

import (
	"context"
	"net/http"

	"edp-manager/internal/config"
	"edp-manager/internal/store"
)

type Server struct {
	cfg config.Config
	st  *store.Store

	apiToken string
}

func New(cfg config.Config, st *store.Store) (*Server, error) {
	s := &Server{cfg: cfg, st: st}
	if err := s.ensureSecrets(context.Background()); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// auth (unauthenticated entry points)
	mux.HandleFunc("GET /api/auth", s.apiAuthStatus)
	mux.HandleFunc("POST /api/login", s.apiLogin)

	// instance registry
	mux.HandleFunc("GET /api/instances", s.requireAPI(s.apiListInstances))
	mux.HandleFunc("POST /api/instances", s.requireAPI(s.apiCreateInstance))
	mux.HandleFunc("GET /api/instances/{id}", s.requireAPI(s.apiGetInstance))
	mux.HandleFunc("PUT /api/instances/{id}", s.requireAPI(s.apiUpdateInstance))
	mux.HandleFunc("DELETE /api/instances/{id}", s.requireAPI(s.apiDeleteInstance))
	mux.HandleFunc("POST /api/instances/{id}/test", s.requireAPI(s.apiTestInstance))

	// aggregate (fan-out across all instances)
	mux.HandleFunc("GET /api/environments", s.requireAPI(s.apiEnvironments))
	mux.HandleFunc("GET /api/overview", s.requireAPI(s.apiOverview))
	mux.HandleFunc("GET /api/status", s.requireAPI(s.apiStatus))
	mux.HandleFunc("GET /api/summary", s.requireAPI(s.apiSummary))

	// per-instance pass-through: /api/instances/{id}/edp/<edp-path>
	mux.HandleFunc("/api/instances/{id}/edp/{rest...}", s.requireAPI(s.proxyToInstance))

	// Optional bundled dashboard: when EDPM_UI_DIR is set, serve the built ui/ SPA
	// as the "/" fallback (behind the API routes above). Unset = headless, "/"
	// 404s. This is the only difference between the with-UI and headless images.
	if s.cfg.UIDir != "" {
		mux.Handle("/", spaHandler(s.cfg.UIDir))
	}

	// CORS lets a separately-hosted dashboard call the API cross-origin with its
	// Bearer token (the API isn't cookie-authed, so a permissive default is safe).
	return logRequests(corsMiddleware(s.cfg.CORSOrigins, mux))
}
