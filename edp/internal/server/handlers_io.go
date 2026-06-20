package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"edp/internal/store"
)

const exportVersion = 1

// importEnv is one environment in an import payload: the env fields plus its
// timed hooks. Embedding store.Environment lets the env fields unmarshal directly
// (no custom UnmarshalJSON exists, so the promoted MarshalJSON is irrelevant here).
type importEnv struct {
	store.Environment
	TimedHooks []store.TimedHook `json:"timed_hooks"`
}

type exportBundle struct {
	Version      int              `json:"version"`
	Environments []map[string]any `json:"environments"`
}

// exportEnvMap renders an env (and its hooks) as a portable map: secrets are
// dropped (Environment.MarshalJSON blanks git/registry creds; we also drop the
// webhook token), and instance-specific fields (id, status, timestamps) are removed.
func (s *Server) exportEnvMap(ctx context.Context, e *store.Environment) map[string]any {
	m := toMap(e)
	// Guard: credentials and the webhook token must never leave via export, even
	// though imports may carry them in. (Environment.MarshalJSON already blanks
	// git/registry creds; we strip explicitly here so the intent is local and
	// can't be undone by a future MarshalJSON change.)
	for _, k := range []string{"id", "status", "created_at", "updated_at",
		"webhook_token", "git_token", "registry_password"} {
		delete(m, k)
	}
	hooks, _ := s.st.ListTimedHooks(ctx, e.ID)
	out := make([]map[string]any, 0, len(hooks))
	for _, h := range hooks {
		hm := toMap(h)
		for _, k := range []string{"id", "env_id", "status", "created_at", "updated_at"} {
			delete(hm, k)
		}
		out = append(out, hm)
	}
	m["timed_hooks"] = out
	return m
}

func toMap(v any) map[string]any {
	b, _ := json.Marshal(v)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}

func writeDownload(w http.ResponseWriter, filename string, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func (s *Server) apiExportEnv(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	e, err := s.st.GetEnvironment(r.Context(), id)
	if err != nil || e == nil {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	bundle := exportBundle{Version: exportVersion, Environments: []map[string]any{s.exportEnvMap(r.Context(), e)}}
	writeDownload(w, "edp-env-"+sanitizeFilename(e.Name)+".json", bundle)
}

func (s *Server) apiExportAll(w http.ResponseWriter, r *http.Request) {
	envs, err := s.st.ListEnvironments(r.Context())
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	items := make([]map[string]any, 0, len(envs))
	for _, e := range envs {
		items = append(items, s.exportEnvMap(r.Context(), e))
	}
	writeDownload(w, "edp-environments.json", exportBundle{Version: exportVersion, Environments: items})
}

func (s *Server) apiImport(w http.ResponseWriter, r *http.Request) {
	data, err := importPayload(r)
	if err != nil {
		s.importResult(w, 0, nil, err)
		return
	}
	items, err := parseImport(data)
	if err != nil {
		s.importResult(w, 0, nil, err)
		return
	}
	var names []string
	for i := range items {
		e := items[i].Environment
		e.ID = 0
		e.Status = ""
		e.WebhookToken = randomToken()
		if e.Name == "" {
			e.Name = "imported"
		}
		e.Name = s.uniqueEnvName(r.Context(), e.Name)
		if err := s.st.CreateEnvironment(r.Context(), &e); err != nil {
			s.importResult(w, len(names), names, err)
			return
		}
		for j := range items[i].TimedHooks {
			h := items[i].TimedHooks[j]
			h.ID, h.EnvID, h.Status = 0, e.ID, ""
			_ = s.st.CreateTimedHook(r.Context(), &h)
		}
		s.maybeAutoDeploy(r.Context(), &e, "auto-deploy on import")
		names = append(names, e.Name)
	}
	s.importResult(w, len(names), names, nil)
}

func (s *Server) importResult(w http.ResponseWriter, n int, names []string, err error) {
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error(), "imported": fmt.Sprint(n)})
		return
	}
	writeJSON(w, 201, map[string]any{"imported": n, "names": names})
}

// importPayload returns the raw JSON, accepting either a raw JSON body or a
// posted form field "json" (so a plain HTML form / curl --data-urlencode works).
func importPayload(r *http.Request) ([]byte, error) {
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "application/x-www-form-urlencoded") || strings.HasPrefix(ct, "multipart/form-data") {
		if err := r.ParseForm(); err != nil {
			return nil, err
		}
		return []byte(r.PostForm.Get("json")), nil
	}
	return io.ReadAll(io.LimitReader(r.Body, 8<<20))
}

// parseImport accepts a bundle {version,environments:[…]}, a bare array, or a
// single env object.
func parseImport(data []byte) ([]importEnv, error) {
	var bundle struct {
		Environments []importEnv `json:"environments"`
	}
	if json.Unmarshal(data, &bundle) == nil && len(bundle.Environments) > 0 {
		return bundle.Environments, nil
	}
	var arr []importEnv
	if json.Unmarshal(data, &arr) == nil && len(arr) > 0 {
		return arr, nil
	}
	var one importEnv
	if err := json.Unmarshal(data, &one); err != nil {
		return nil, fmt.Errorf("not valid JSON: %w", err)
	}
	if one.Name == "" {
		return nil, fmt.Errorf("no environments found in import")
	}
	return []importEnv{one}, nil
}

// uniqueEnvName appends -2, -3, … until the name is free.
func (s *Server) uniqueEnvName(ctx context.Context, name string) string {
	base := name
	for n := 2; ; n++ {
		if e, _ := s.st.EnvByName(ctx, name); e == nil {
			return name
		}
		name = fmt.Sprintf("%s-%d", base, n)
	}
}

// Bootstrap loads environments declared in EDP_IMPORT (inline JSON or a file
// path) on startup. It is idempotent: an env whose name already exists is left
// untouched, so the variable can stay set across restarts. Credentials and timed
// hooks present in the bundle are applied.
func (s *Server) Bootstrap(ctx context.Context) {
	raw := strings.TrimSpace(s.cfg.Import)
	if raw == "" {
		return
	}
	data := []byte(raw)
	if !looksLikeJSON(raw) { // treat as a file path
		b, err := os.ReadFile(raw)
		if err != nil {
			log.Printf("bootstrap: read EDP_IMPORT file %q: %v", raw, err)
			return
		}
		data = b
	}
	items, err := parseImport(data)
	if err != nil {
		log.Printf("bootstrap: %v", err)
		return
	}
	created, skipped := 0, 0
	for i := range items {
		e := items[i].Environment
		if e.Name == "" {
			continue
		}
		if existing, _ := s.st.EnvByName(ctx, e.Name); existing != nil {
			skipped++
			continue
		}
		e.ID, e.Status, e.WebhookToken = 0, "", randomToken()
		if err := s.st.CreateEnvironment(ctx, &e); err != nil {
			log.Printf("bootstrap: create %q: %v", e.Name, err)
			continue
		}
		for j := range items[i].TimedHooks {
			h := items[i].TimedHooks[j]
			h.ID, h.EnvID, h.Status = 0, e.ID, ""
			_ = s.st.CreateTimedHook(ctx, &h)
		}
		s.maybeAutoDeploy(ctx, &e, "auto-deploy on startup")
		created++
		log.Printf("bootstrap: loaded environment %q", e.Name)
	}
	if created > 0 || skipped > 0 {
		log.Printf("bootstrap: %d environment(s) loaded, %d already present", created, skipped)
	}
}

func looksLikeJSON(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[")
}

func sanitizeFilename(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			out = append(out, r)
		} else {
			out = append(out, '-')
		}
	}
	if len(out) == 0 {
		return "env"
	}
	return string(out)
}
