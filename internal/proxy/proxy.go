// Package proxy fronts test environments with a reverse proxy. It routes by
// Host header (host-based) or a /e/{name}/ path prefix (path-based) to the
// environment's container over the shared Docker network. While an env is
// deploying or not yet ready, it serves a "starting/redeploying" splash with an
// ETA instead of a connection error.
package proxy

import (
	"context"
	_ "embed"
	"html/template"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"edp/internal/clearsite"
	"edp/internal/docker"
	"edp/internal/naming"
	"edp/internal/store"
)

//go:embed splash.html
var splashHTML string

type Proxy struct {
	st         *store.Store
	dk         *docker.Client
	clear      *clearsite.Flags
	trustProxy bool
	next       http.Handler // edp's own dashboard/API mux
	splash     *template.Template
}

func New(st *store.Store, dk *docker.Client, clear *clearsite.Flags, trustProxy bool, next http.Handler) *Proxy {
	return &Proxy{
		st:         st,
		dk:         dk,
		clear:      clear,
		trustProxy: trustProxy,
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
	if !p.ready(r.Context(), env) {
		p.serveSplash(w, r, env)
		return
	}
	p.reverse(w, r, env, prefix)
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
		p.serveSplash(w, r, env)
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

func (p *Proxy) serveSplash(w http.ResponseWriter, r *http.Request, env *store.Environment) {
	etaMs := p.st.EstimateDurationMs(r.Context(), env.ID)
	deploying := env.Status == store.StatusRunning

	// elapsed since the in-flight deploy started (for the progress bar)
	var elapsedMs int64
	if dep, _ := p.st.LatestDeployment(r.Context(), env.ID); dep != nil && dep.StartedAt != nil && deploying {
		elapsedMs = time.Since(*dep.StartedAt).Milliseconds()
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
