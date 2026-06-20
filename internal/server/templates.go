package server

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static
var staticRoot embed.FS

var staticFS, _ = fs.Sub(staticRoot, "static")

// pages are rendered by cloning the base layout and parsing each page on top,
// so every page provides its own "content" block.
var pageFiles = []string{"login.html", "dashboard.html", "env_form.html", "env_detail.html", "hook_form.html", "hook_detail.html", "env_import.html"}

func (s *Server) loadTemplates() error {
	set := map[string]*template.Template{}
	for _, p := range pageFiles {
		t, err := template.New("base.html").Funcs(tmplFuncs).ParseFS(templatesFS, "templates/base.html", "templates/"+p)
		if err != nil {
			return fmt.Errorf("parse %s: %w", p, err)
		}
		set[p] = t
	}
	s.pages = set
	return nil
}

func (s *Server) render(w http.ResponseWriter, page string, data any) {
	t := s.pages[page]
	if t == nil {
		http.Error(w, "template not found: "+page, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "base.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

var tmplFuncs = template.FuncMap{
	"short": func(s string) string {
		if len(s) > 12 {
			return s[:12]
		}
		return s
	},
	"list":     func(items ...string) []string { return items },
	"contains": strings.Contains,
	// durMs renders a millisecond duration compactly: "840ms", "8.4s", "1m 5s".
	"durMs": func(ms int64) string {
		if ms <= 0 {
			return ""
		}
		if ms < 1000 {
			return fmt.Sprintf("%dms", ms)
		}
		s := float64(ms) / 1000
		if s < 60 {
			return fmt.Sprintf("%.1fs", s)
		}
		return fmt.Sprintf("%dm %ds", int(s)/60, int(s)%60)
	},
}
