package server

import "net/http"

// corsMiddleware lets a browser app on another origin (the standalone edp-ui)
// call the API. Auth is a Bearer header, not a cookie, so a wildcard origin is
// safe — there are no ambient credentials for a hostile page to ride on.
//
// `origins` is the configured EDP_CORS_ORIGINS. When it is "*" (the default) we
// reflect the request's Origin so the header is concrete (some clients dislike a
// bare "*"); otherwise we echo it verbatim.
func corsMiddleware(origins string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			allow := origins
			if origins == "" || origins == "*" {
				allow = origin
			}
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", allow)
			h.Add("Vary", "Origin")
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			h.Set("Access-Control-Max-Age", "86400")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
