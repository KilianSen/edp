package server

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// spaHandler serves a built single-page app (edp-ui) from dir: real files when
// they exist, otherwise index.html so client-side routes (/env/123, …) resolve on
// reload. It is registered only when EDP_UI_DIR is set — that's the sole
// difference between the bundled-UI image and the headless one.
//
// It is mounted as the mux's "/" fallback, behind /api and /hooks (more specific
// patterns win) and behind the reverse proxy (which still fronts test-env
// traffic), so it only ever sees dashboard requests.
func spaHandler(dir string) http.Handler {
	index := filepath.Join(dir, "index.html")
	fileServer := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Resolve the request path under dir without escaping it. Use path (URL,
		// forward-slash) for the cleaned name so the no-store match below is
		// portable, then convert to an OS path for the filesystem lookup.
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
