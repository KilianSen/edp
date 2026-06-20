package server

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// spaHandler serves a built single-page app (ui/) from dir: real files when they
// exist, otherwise index.html so client-side routes (/env/new, /i/1/env/2, …)
// resolve on reload. Registered only when EDPM_UI_DIR is set — the sole
// difference between the bundled-UI image and the headless one. Mirrors edp.
func spaHandler(dir string) http.Handler {
	index := filepath.Join(dir, "index.html")
	fileServer := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel := path.Clean("/" + strings.TrimPrefix(r.URL.Path, "/"))
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if st, err := os.Stat(full); err == nil && !st.IsDir() {
			// The shell and its runtime config must never be cached, so a UI update
			// or a re-pointed apiBase takes effect on the next load.
			if rel == "/index.html" || rel == "/config.js" {
				w.Header().Set("Cache-Control", "no-store")
			}
			fileServer.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFile(w, r, index)
	})
}
