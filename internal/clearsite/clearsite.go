// Package clearsite coordinates one-shot browser-data clearing after a redeploy.
// The deploy engine marks an environment when it redeploys; the reverse proxy
// consumes that mark once, emitting a Clear-Site-Data response header so the
// browser drops the env's cookies/cache/storage and starts from a clean slate.
package clearsite

import (
	"strings"
	"sync"
)

// allowed are the Clear-Site-Data directives we accept. "storage" covers
// localStorage, sessionStorage and IndexedDB; "executionContexts" forces a reload.
var allowed = map[string]bool{
	"cache": true, "cookies": true, "storage": true, "executionContexts": true,
}

type Flags struct {
	mu      sync.Mutex
	pending map[int64]string // envID -> comma-separated directives
}

func New() *Flags { return &Flags{pending: map[int64]string{}} }

// Mark records that env should have its browser data cleared on the next
// proxied navigation. directives is the env's stored comma list; empty is a no-op.
func (f *Flags) Mark(envID int64, directives string) {
	if strings.TrimSpace(directives) == "" {
		return
	}
	f.mu.Lock()
	f.pending[envID] = directives
	f.mu.Unlock()
}

// Take returns the formatted Clear-Site-Data header value for env and clears the
// pending mark, so the header is sent only once per redeploy. Returns "" if none.
func (f *Flags) Take(envID int64) string {
	f.mu.Lock()
	d, ok := f.pending[envID]
	if ok {
		delete(f.pending, envID)
	}
	f.mu.Unlock()
	if !ok {
		return ""
	}
	return Header(d)
}

// Header turns a comma list of directives into a Clear-Site-Data header value:
//
//	"cache","cookies" -> `"cache", "cookies"`
func Header(directives string) string {
	var parts []string
	for _, d := range strings.Split(directives, ",") {
		d = strings.TrimSpace(d)
		if allowed[d] {
			parts = append(parts, `"`+d+`"`)
		}
	}
	return strings.Join(parts, ", ")
}
