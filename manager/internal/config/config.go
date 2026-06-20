// Package config holds runtime configuration for edp-manager, sourced from
// environment variables with sensible defaults.
package config

import (
	"os"
	"strings"
)

type Config struct {
	// Addr is the HTTP listen address, e.g. ":9090".
	Addr string
	// DataDir holds the SQLite DB and the encryption key (mounted volume).
	DataDir string
	// AdminPassword, when set, is authoritatively (re)applied on every boot — the
	// recovery path for a forgotten manager password.
	AdminPassword string
	// ConfigFile is an optional YAML file of edp instances to seed on boot
	// (idempotent: instances whose label already exists are left untouched).
	ConfigFile string
	// CORSOrigins is the Access-Control-Allow-Origin for the API, so a separately
	// hosted UI can call it. Default "*" (the API is Bearer-token authed).
	CORSOrigins string
	// FanoutTimeoutMs bounds how long a single instance may take during a fan-out
	// before it's reported as errored (the others still return).
	FanoutTimeoutMs int
	// UIDir, if set, is a directory of static dashboard files (a built ui/) that
	// the manager serves at "/" with SPA history fallback. Empty = headless (API
	// only); the "with UI" image (Dockerfile.ui) sets it. Single binary, one knob
	// — mirrors edp's EDP_UI_DIR.
	UIDir string
}

func getenv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func atoi(s string, def int) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return def
		}
		n = n*10 + int(r-'0')
	}
	if s == "" {
		return def
	}
	return n
}

func Load() Config {
	return Config{
		Addr:            getenv("EDPM_ADDR", ":9090"),
		DataDir:         getenv("EDPM_DATA_DIR", "/data"),
		AdminPassword:   getenv("EDPM_ADMIN_PASSWORD", ""),
		ConfigFile:      getenv("EDPM_CONFIG", ""),
		CORSOrigins:     getenv("EDPM_CORS_ORIGINS", "*"),
		FanoutTimeoutMs: atoi(getenv("EDPM_FANOUT_TIMEOUT_MS", "8000"), 8000),
		UIDir:           getenv("EDPM_UI_DIR", ""),
	}
}
