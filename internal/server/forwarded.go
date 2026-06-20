package server

import (
	"net/http"
	"strings"
)

// extScheme returns the scheme the client actually used to reach edp. When
// EDP_TRUST_PROXY is set it honors X-Forwarded-Proto (e.g. NPM terminating TLS).
func (s *Server) extScheme(r *http.Request) string {
	if s.cfg.TrustProxy {
		if p := firstToken(r.Header.Get("X-Forwarded-Proto")); p != "" {
			return p
		}
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

// extHost returns the host the client used (X-Forwarded-Host behind a trusted
// proxy, otherwise the request Host).
func (s *Server) extHost(r *http.Request) string {
	if s.cfg.TrustProxy {
		if h := firstToken(r.Header.Get("X-Forwarded-Host")); h != "" {
			return h
		}
	}
	return r.Host
}

// extBase is the external origin (scheme://host) for building absolute URLs.
func (s *Server) extBase(r *http.Request) string {
	return s.extScheme(r) + "://" + s.extHost(r)
}

func (s *Server) externalIsHTTPS(r *http.Request) bool { return s.extScheme(r) == "https" }

// firstToken returns the first comma-separated value, trimmed (XFF can be a list).
func firstToken(v string) string {
	if i := strings.IndexByte(v, ','); i >= 0 {
		v = v[:i]
	}
	return strings.TrimSpace(v)
}
