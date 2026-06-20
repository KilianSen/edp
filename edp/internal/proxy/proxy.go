// Package proxy fronts test environments with a reverse proxy. It routes by
// Host header (host-based) or a /e/{name}/ path prefix (path-based) to the
// environment's container over the shared Docker network. While an env is
// deploying or not yet ready, it serves a "starting/redeploying" splash with an
// ETA instead of a connection error — or, when a splash UI is configured,
// redirects to that external UI and exposes per-env control endpoints it drives.
package proxy

import (
	"context"
	_ "embed"
	"encoding/json"
	"html/template"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"edp/internal/clearsite"
	"edp/internal/docker"
	"edp/internal/naming"
	"edp/internal/store"
)

//go:embed splash.html
var splashHTML string

// Deployer triggers a redeploy; the deploy engine satisfies it. Kept as an
// interface so the proxy doesn't depend on the deploy package directly.
type Deployer interface {
	Trigger(ctx context.Context, envID int64, trigger, reason string) (int64, error)
}

type Proxy struct {
	st         *store.Store
	dk         *docker.Client
	engine     Deployer
	clear      *clearsite.Flags
	trustProxy bool
	splashURL  string       // global external splash UI; per-env splash_url overrides
	corsOrigin string       // Access-Control-Allow-Origin for the /_edp/ control endpoints
	next       http.Handler // edp's own dashboard/API mux
	splash     *template.Template
}

func New(st *store.Store, dk *docker.Client, engine Deployer, clear *clearsite.Flags, splashURL, corsOrigin string, trustProxy bool, next http.Handler) *Proxy {
	if corsOrigin == "" {
		corsOrigin = "*"
	}
	return &Proxy{
		st:         st,
		dk:         dk,
		engine:     engine,
		clear:      clear,
		trustProxy: trustProxy,
		splashURL:  splashURL,
		corsOrigin: corsOrigin,
		next:       next,
		splash:     template.Must(template.New("splash").Parse(splashHTML)),
	}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	env, prefix := p.resolve(r)
	if env == nil {
		p.next.ServeHTTP(w, r) // not a proxied request → dashboard/API
		return
	}
	// Reserved control endpoints (status/redeploy) that an external splash UI
	// calls back into. Handled before the ready check so they work while the env
	// is down, and on the proxied origin so they're reachable for any routing mode.
	rel := r.URL.Path
	if prefix != "" {
		rel = strings.TrimPrefix(rel, prefix)
	}
	if action, ok := strings.CutPrefix(rel, "/_edp/"); ok {
		p.serveControl(w, r, env, action)
		return
	}
	if !p.ready(r.Context(), env) {
		p.serveSplash(w, r, env, prefix)
		return
	}
	p.reverse(w, r, env, prefix)
}

// serveControl handles the per-env control API the external splash UI uses:
//
//	GET  <env>/_edp/status?token=...    → JSON {name,status,ready,deploying,eta_ms,elapsed_ms}
//	POST <env>/_edp/redeploy?token=...  → trigger a redeploy, JSON {deployment_id}
//
// Auth reuses the env's webhook token (no admin credentials in the browser).
// CORS is enabled so the UI may be hosted on another origin.
func (p *Proxy) serveControl(w http.ResponseWriter, r *http.Request, env *store.Environment, action string) {
	p.setCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	token := r.URL.Query().Get("token")
	if token == "" {
		token = r.Header.Get("X-Edp-Token")
	}
	if env.WebhookToken == "" || token != env.WebhookToken {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	switch action {
	case "status":
		deploying, etaMs, elapsedMs := p.statusOf(r.Context(), env)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"name":       env.Name,
			"status":     env.Status,
			"ready":      p.ready(r.Context(), env),
			"deploying":  deploying,
			"eta_ms":     etaMs,
			"elapsed_ms": elapsedMs,
		})
	case "redeploy":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if p.engine == nil {
			http.Error(w, "redeploy unavailable", http.StatusServiceUnavailable)
			return
		}
		reason := firstToken(r.URL.Query().Get("reason"))
		if reason == "" {
			reason = "external-ui"
		}
		depID, err := p.engine.Trigger(r.Context(), env.ID, store.TriggerWebhook, reason)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int64{"deployment_id": depID})
	default:
		http.NotFound(w, r)
	}
}

func (p *Proxy) setCORS(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Access-Control-Allow-Origin", p.corsOrigin)
	h.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	h.Set("Access-Control-Allow-Headers", "Content-Type, X-Edp-Token")
	h.Add("Vary", "Origin")
}

// resolve finds the env a request targets, returning it and any path prefix to
// strip (for path-based routing). Returns nil when the request is for edp itself.
func (p *Proxy) resolve(r *http.Request) (*store.Environment, string) {
	host := p.externalHost(r)
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	if env, _ := p.st.EnvByProxyHost(r.Context(), host); env != nil {
		return env, ""
	}
	// path-based: /e/{name}/...
	if rest, ok := strings.CutPrefix(r.URL.Path, "/e/"); ok {
		name := rest
		if i := strings.IndexByte(rest, '/'); i >= 0 {
			name = rest[:i]
		}
		if env, _ := p.st.EnvByName(r.Context(), name); env != nil && env.ProxyPort != "" {
			return env, "/e/" + name
		}
	}
	return nil, ""
}

// ready reports whether the env has at least one running container.
func (p *Proxy) ready(ctx context.Context, env *store.Environment) bool {
	if env.Status == store.StatusRunning { // a deploy is in progress
		return false
	}
	infos, err := p.dk.ListByEnv(ctx, env.ID)
	if err != nil {
		return false
	}
	for _, c := range infos {
		if c.State == "running" {
			return true
		}
	}
	return false
}

func (p *Proxy) reverse(w http.ResponseWriter, r *http.Request, env *store.Environment, prefix string) {
	// One-shot browser-data clear after a redeploy: on the first top-level
	// navigation, emit Clear-Site-Data so the browser starts from a clean slate.
	if p.clear != nil && isNavigation(r) {
		if hdr := p.clear.Take(env.ID); hdr != "" {
			w.Header().Set("Clear-Site-Data", hdr)
		}
	}

	target := &url.URL{Scheme: "http", Host: naming.ContainerName(env.Name) + ":" + env.ProxyPort}
	rp := httputil.NewSingleHostReverseProxy(target)
	inner := rp.Director
	scheme, host := p.externalScheme(r), p.externalHost(r)
	rp.Director = func(req *http.Request) {
		inner(req)
		// tell the test app the external scheme/host the browser used (through
		// any upstream proxy), so its own redirects/links are correct.
		req.Header.Set("X-Forwarded-Proto", scheme)
		req.Header.Set("X-Forwarded-Host", host)
	}
	if prefix != "" {
		r.URL.Path = strings.TrimPrefix(r.URL.Path, prefix)
		if r.URL.Path == "" {
			r.URL.Path = "/"
		}
	}
	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		// target went away mid-request → show the splash rather than a raw 502
		p.serveSplash(w, r, env, prefix)
	}
	rp.ServeHTTP(w, r)
}

// externalHost / externalScheme report what the client used to reach edp,
// honoring X-Forwarded-* when edp itself sits behind a trusted proxy (NPM).
func (p *Proxy) externalHost(r *http.Request) string {
	if p.trustProxy {
		if h := firstToken(r.Header.Get("X-Forwarded-Host")); h != "" {
			return h
		}
	}
	return r.Host
}

func (p *Proxy) externalScheme(r *http.Request) string {
	if p.trustProxy {
		if s := firstToken(r.Header.Get("X-Forwarded-Proto")); s != "" {
			return s
		}
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func firstToken(v string) string {
	if i := strings.IndexByte(v, ','); i >= 0 {
		v = v[:i]
	}
	return strings.TrimSpace(v)
}

// isNavigation reports whether the request is a top-level page load (vs an
// asset/XHR), so a one-shot clear lands on a real navigation and the optional
// "executionContexts" reload is meaningful.
func isNavigation(r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	if dest := r.Header.Get("Sec-Fetch-Dest"); dest != "" {
		return dest == "document"
	}
	return strings.Contains(r.Header.Get("Accept"), "text/html")
}

// statusOf reports whether a deploy is in flight, the ETA for it (from past
// successful runs), and how long the current deploy has been running.
func (p *Proxy) statusOf(ctx context.Context, env *store.Environment) (deploying bool, etaMs, elapsedMs int64) {
	etaMs = p.st.EstimateDurationMs(ctx, env.ID)
	deploying = env.Status == store.StatusRunning
	if dep, _ := p.st.LatestDeployment(ctx, env.ID); dep != nil && dep.StartedAt != nil && deploying {
		elapsedMs = time.Since(*dep.StartedAt).Milliseconds()
	}
	return
}

// splashFor returns the external splash UI for an env (its own, else the global
// default), or "" to use the built-in embedded splash.
func (p *Proxy) splashFor(env *store.Environment) string {
	if env.SplashURL != "" {
		return env.SplashURL
	}
	return p.splashURL
}

func (p *Proxy) serveSplash(w http.ResponseWriter, r *http.Request, env *store.Environment, prefix string) {
	deploying, etaMs, elapsedMs := p.statusOf(r.Context(), env)

	// When an external splash UI is configured, redirect top-level navigations to
	// it (with context + a return URL + the control base/token). Asset/XHR
	// requests still get the lightweight 503 below so they don't bounce to HTML.
	if base := p.splashFor(env); base != "" && isNavigation(r) {
		p.redirectToSplash(w, r, env, prefix, base, deploying, etaMs, elapsedMs)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusServiceUnavailable)
	_ = p.splash.Execute(w, map[string]any{
		"Name":      env.Name,
		"Deploying": deploying,
		"EtaMs":     etaMs,
		"ElapsedMs": elapsedMs,
	})
}

// redirectToSplash sends the browser to the external splash UI, passing the env
// name, deploy state/ETA, the original URL to return to once ready, and the
// control base + token the UI uses to poll status and trigger a redeploy.
func (p *Proxy) redirectToSplash(w http.ResponseWriter, r *http.Request, env *store.Environment, prefix, base string, deploying bool, etaMs, elapsedMs int64) {
	scheme, host := p.externalScheme(r), p.externalHost(r)
	state := "starting"
	if deploying {
		state = "deploying"
	}
	q := url.Values{}
	q.Set("env", env.Name)
	q.Set("state", state)
	q.Set("eta", strconv.FormatInt(etaMs, 10))
	q.Set("elapsed", strconv.FormatInt(elapsedMs, 10))
	q.Set("return", scheme+"://"+host+r.URL.RequestURI())
	q.Set("ctl", scheme+"://"+host+prefix) // control endpoints live at <ctl>/_edp/*
	q.Set("token", env.WebhookToken)

	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	w.Header().Set("Referrer-Policy", "no-referrer") // keep the token out of the Referer header
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, base+sep+q.Encode(), http.StatusFound)
}
