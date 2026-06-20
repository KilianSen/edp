// Package server exposes edp's HTTP surface: the JSON API and the inbound
// webhook endpoint. The dashboard itself is a separate single-page app (edp-ui)
// that consumes this API with a Bearer token; nothing here renders HTML.
package server

import (
	"context"
	"net/http"

	"edp/internal/clearsite"
	"edp/internal/config"
	"edp/internal/deploy"
	"edp/internal/docker"
	"edp/internal/hooks"
	"edp/internal/logbus"
	"edp/internal/proxy"
	"edp/internal/statushub"
	"edp/internal/store"
)

type Server struct {
	cfg    config.Config
	st     *store.Store
	engine *deploy.Engine
	bus    *logbus.Bus
	dk     *docker.Client
	hub    *statushub.Hub
	hooks  *hooks.Runner
	clear  *clearsite.Flags

	apiToken string
}

func New(cfg config.Config, st *store.Store, engine *deploy.Engine, bus *logbus.Bus, dk *docker.Client, hub *statushub.Hub, hookRunner *hooks.Runner, clear *clearsite.Flags) (*Server, error) {
	s := &Server{cfg: cfg, st: st, engine: engine, bus: bus, dk: dk, hub: hub, hooks: hookRunner, clear: clear}
	if err := s.ensureSecrets(context.Background()); err != nil {
		return nil, err
	}
	return s, nil
}

// Handler builds the router. The JSON API requires a Bearer token; /hooks uses
// per-env tokens; /api/login exchanges the admin password for the API token.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// auth: unauthenticated login + first-run status (the SPA's entry point)
	mux.HandleFunc("GET /api/auth", s.apiAuthStatus)
	mux.HandleFunc("POST /api/login", s.apiLogin)

	// JSON API
	mux.HandleFunc("GET /api/environments", s.requireAPI(s.apiListEnvs))
	mux.HandleFunc("POST /api/environments", s.requireAPI(s.apiCreateEnv))
	mux.HandleFunc("GET /api/environments/{id}", s.requireAPI(s.apiGetEnv))
	mux.HandleFunc("PUT /api/environments/{id}", s.requireAPI(s.apiUpdateEnv))
	mux.HandleFunc("POST /api/environments/{id}", s.requireAPI(s.apiUpdateEnv)) // form-friendly
	mux.HandleFunc("DELETE /api/environments/{id}", s.requireAPI(s.apiDeleteEnv))
	mux.HandleFunc("POST /api/environments/{id}/deploy", s.requireAPI(s.apiDeploy))
	mux.HandleFunc("POST /api/environments/{id}/rotate-token", s.requireAPI(s.apiRotateToken))
	mux.HandleFunc("GET /api/environments/{id}/deployments", s.requireAPI(s.apiListDeployments))
	mux.HandleFunc("GET /api/deployments/{id}", s.requireAPI(s.apiGetDeployment))
	mux.HandleFunc("GET /api/deployments/{id}/logs/stream", s.requireAPI(s.apiStreamLogs))
	mux.HandleFunc("GET /api/overview", s.requireAPI(s.apiOverview))
	mux.HandleFunc("GET /api/status", s.requireAPI(s.apiStatus))
	mux.HandleFunc("GET /api/events/stream", s.requireAPI(s.apiEventsStream))

	// import / export
	mux.HandleFunc("GET /api/export", s.requireAPI(s.apiExportAll))
	mux.HandleFunc("GET /api/environments/{id}/export", s.requireAPI(s.apiExportEnv))
	mux.HandleFunc("POST /api/import", s.requireAPI(s.apiImport))

	// timed hooks
	mux.HandleFunc("GET /api/environments/{id}/hooks", s.requireAPI(s.apiListHooks))
	mux.HandleFunc("POST /api/environments/{id}/hooks", s.requireAPI(s.apiCreateHook))
	mux.HandleFunc("GET /api/hooks/{id}", s.requireAPI(s.apiGetHook))
	mux.HandleFunc("PUT /api/hooks/{id}", s.requireAPI(s.apiUpdateHook))
	mux.HandleFunc("POST /api/hooks/{id}", s.requireAPI(s.apiUpdateHook)) // form-friendly
	mux.HandleFunc("DELETE /api/hooks/{id}", s.requireAPI(s.apiDeleteHook))
	mux.HandleFunc("POST /api/hooks/{id}/delete", s.requireAPI(s.apiDeleteHook)) // form-friendly
	mux.HandleFunc("POST /api/hooks/{id}/run", s.requireAPI(s.apiRunHook))
	mux.HandleFunc("GET /api/hooks/{id}/runs", s.requireAPI(s.apiListHookRuns))
	mux.HandleFunc("GET /api/hook-runs/{id}", s.requireAPI(s.apiGetHookRun))
	mux.HandleFunc("GET /api/hook-runs/{id}/logs/stream", s.requireAPI(s.apiStreamHookLog))

	// webhook
	mux.HandleFunc("POST /hooks/{id}", s.hookDeploy)

	// Optional bundled dashboard: when EDP_UI_DIR is set, serve the built edp-ui
	// SPA as the "/" fallback (behind the API/webhook routes above and the reverse
	// proxy). Unset = headless, "/" 404s. This is what the with-UI image enables.
	if s.cfg.UIDir != "" {
		mux.Handle("/", spaHandler(s.cfg.UIDir))
	}

	// CORS lets the standalone edp-ui (a different origin) call the API with its
	// Bearer token. The reverse proxy sits in front: it routes proxied test-env
	// traffic, and falls through to this API mux for everything else.
	return proxy.New(s.st, s.dk, s.engine, s.clear, s.cfg.SplashURL, s.cfg.CORSOrigins, s.cfg.TrustProxy, logRequests(corsMiddleware(s.cfg.CORSOrigins, mux)))
}
