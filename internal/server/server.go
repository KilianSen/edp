// Package server exposes edp's HTTP surface: the dashboard, the JSON API, and
// the inbound webhook endpoint.
package server

import (
	"context"
	"html/template"
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
	pages  map[string]*template.Template

	sessionSecret []byte
	apiToken      string
}

func New(cfg config.Config, st *store.Store, engine *deploy.Engine, bus *logbus.Bus, dk *docker.Client, hub *statushub.Hub, hookRunner *hooks.Runner, clear *clearsite.Flags) (*Server, error) {
	s := &Server{cfg: cfg, st: st, engine: engine, bus: bus, dk: dk, hub: hub, hooks: hookRunner, clear: clear}
	if err := s.loadTemplates(); err != nil {
		return nil, err
	}
	if err := s.ensureSecrets(context.Background()); err != nil {
		return nil, err
	}
	return s, nil
}

// Handler builds the router. Web pages require a session cookie; /api requires a
// session cookie or Bearer token; /hooks uses per-env tokens.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// static assets
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// auth / pages
	mux.HandleFunc("GET /login", s.pageLogin)
	mux.HandleFunc("POST /login", s.doLogin)
	mux.HandleFunc("POST /logout", s.doLogout)
	mux.HandleFunc("GET /", s.requireWeb(s.pageDashboard))
	mux.HandleFunc("GET /env/new", s.requireWeb(s.pageEnvForm))
	mux.HandleFunc("GET /env/{id}", s.requireWeb(s.pageEnvDetail))
	mux.HandleFunc("GET /env/{id}/edit", s.requireWeb(s.pageEnvForm))

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
	mux.HandleFunc("GET /api/status", s.requireAPI(s.apiStatus))
	mux.HandleFunc("GET /api/events/stream", s.requireAPI(s.apiEventsStream))
	mux.HandleFunc("GET /partials/envs", s.requireWeb(s.partialEnvRows))

	// import / export
	mux.HandleFunc("GET /api/export", s.requireAPI(s.apiExportAll))
	mux.HandleFunc("GET /api/environments/{id}/export", s.requireAPI(s.apiExportEnv))
	mux.HandleFunc("POST /api/import", s.requireAPI(s.apiImport))
	mux.HandleFunc("GET /env/import", s.requireWeb(s.pageImport))

	// timed hooks
	mux.HandleFunc("GET /api/environments/{id}/hooks", s.requireAPI(s.apiListHooks))
	mux.HandleFunc("POST /api/environments/{id}/hooks", s.requireAPI(s.apiCreateHook))
	mux.HandleFunc("GET /api/hooks/{id}", s.requireAPI(s.apiGetHook))
	mux.HandleFunc("PUT /api/hooks/{id}", s.requireAPI(s.apiUpdateHook))
	mux.HandleFunc("POST /api/hooks/{id}", s.requireAPI(s.apiUpdateHook)) // form-friendly
	mux.HandleFunc("DELETE /api/hooks/{id}", s.requireAPI(s.apiDeleteHook))
	mux.HandleFunc("POST /api/hooks/{id}/delete", s.requireAPI(s.apiDeleteHook)) // form-friendly
	mux.HandleFunc("POST /api/hooks/{id}/run", s.requireAPI(s.apiRunHook))
	mux.HandleFunc("GET /api/hook-runs/{id}", s.requireAPI(s.apiGetHookRun))
	mux.HandleFunc("GET /api/hook-runs/{id}/logs/stream", s.requireAPI(s.apiStreamHookLog))
	mux.HandleFunc("GET /env/{id}/hooks/new", s.requireWeb(s.pageHookForm))
	mux.HandleFunc("GET /timed-hooks/{id}/edit", s.requireWeb(s.pageHookForm))
	mux.HandleFunc("GET /timed-hooks/{id}", s.requireWeb(s.pageHookDetail))

	// webhook
	mux.HandleFunc("POST /hooks/{id}", s.hookDeploy)

	// The reverse proxy sits in front: it routes proxied test-env traffic, and
	// falls through to the dashboard/API mux for everything else.
	return proxy.New(s.st, s.dk, s.clear, s.cfg.TrustProxy, logRequests(mux))
}
