// Package config holds runtime configuration for edp, sourced from environment
// variables with sensible defaults so the container "just works".
package config

import (
	"os"
	"strings"
)

type Config struct {
	// Addr is the listen address for the HTTP server, e.g. ":8080".
	Addr string
	// DataDir holds the SQLite database (mounted volume in production).
	DataDir string
	// WorkspaceDir holds git checkouts / build contexts (mounted volume).
	WorkspaceDir string
	// AdminPassword, if set on first boot, seeds the admin login.
	AdminPassword string
	// DockerBin / ComposeArgs let us override how we shell out to Docker.
	DockerBin string
	// PythonBin is the interpreter used to run lifecycle hooks.
	PythonBin string
	// Image is the edp image used for TCP port-forward sidecars. If empty, edp
	// auto-detects it from its own container.
	Image string
	// TrustProxy makes edp honor X-Forwarded-Proto/Host (and mark the session
	// cookie Secure on https). Enable only when edp sits behind a trusted reverse
	// proxy such as Nginx Proxy Manager that sets those headers.
	TrustProxy bool
	// Import is an export bundle to load on startup — either inline JSON or a path
	// to a JSON file. Environments whose name already exists are left untouched.
	Import string
	// ReapOnExit tears down all of this instance's managed containers, compose
	// stacks, forwarders, and the shared network when edp shuts down. Default off:
	// a routine restart/update of edp must NOT destroy running test envs. Enable
	// only when envs are reprovisioned on boot (e.g. EDP_IMPORT + auto_deploy).
	ReapOnExit bool
	// SplashURL is an external "deploying/starting" UI to redirect to while a
	// proxied env is not ready, instead of edp's built-in splash. Per-env
	// splash_url overrides it. The external UI drives redeploy/status via the
	// per-env control endpoints (see the proxy package). Empty = built-in splash.
	SplashURL string
	// CORSOrigins is the Access-Control-Allow-Origin value for the JSON API
	// (/api/*) and the per-env proxy control endpoints (/_edp/*), so a
	// separately-hosted dashboard (edp-ui) or splash UI can call them from
	// another origin. Default "*" (auth is a Bearer token / per-env token, not a
	// cookie, so there are no ambient credentials to abuse).
	CORSOrigins string
	// UIDir, if set, is a directory of static dashboard files (a built edp-ui)
	// that edp serves at "/" with SPA history fallback. Empty = headless (API
	// only); the "with UI" container image sets it. This is the single knob that
	// distinguishes the bundled-UI image from the headless one — same binary.
	UIDir string
}

func getenv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func isTrue(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func Load() Config {
	return Config{
		Addr:          getenv("EDP_ADDR", ":8080"),
		DataDir:       getenv("EDP_DATA_DIR", "/data"),
		WorkspaceDir:  getenv("EDP_WORKSPACE_DIR", "/workspace"),
		AdminPassword: getenv("EDP_ADMIN_PASSWORD", ""),
		DockerBin:     getenv("EDP_DOCKER_BIN", "docker"),
		PythonBin:     getenv("EDP_PYTHON_BIN", "python3"),
		Image:         getenv("EDP_IMAGE", ""),
		TrustProxy:    isTrue(getenv("EDP_TRUST_PROXY", "")),
		Import:        getenv("EDP_IMPORT", ""),
		ReapOnExit:    isTrue(getenv("EDP_REAP_ON_EXIT", "")),
		SplashURL:     getenv("EDP_SPLASH_URL", ""),
		CORSOrigins:   getenv("EDP_CORS_ORIGINS", "*"),
		UIDir:         getenv("EDP_UI_DIR", ""),
	}
}
