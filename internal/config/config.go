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
	}
}
