package server

import (
	"net/http"
	"strconv"

	"edp/internal/store"
)

func itoa(i int64) string { return strconv.FormatInt(i, 10) }

// envView decorates an Environment with live runtime info for templates.
type envView struct {
	*store.Environment
	ContainerState string
	ContainerInfo  string
	Last           *store.Deployment
	EstimateMs     int64 // ETA for the next deploy, from past successful runs
}

// Deploying reports whether the env's latest deployment is in flight.
func (v envView) Deploying() bool {
	return v.Last != nil && (v.Last.Status == store.StatusRunning || v.Last.Status == store.StatusQueued)
}

// hookView decorates a TimedHook with its most recent run for templates.
type hookView struct {
	*store.TimedHook
	Last *store.HookRun
}

func (s *Server) buildEnvView(r *http.Request, e *store.Environment) envView {
	v := envView{Environment: e}
	v.ContainerState, v.ContainerInfo = s.containerState(r.Context(), e.ID)
	v.Last, _ = s.st.LatestDeployment(r.Context(), e.ID)
	v.EstimateMs = s.st.EstimateDurationMs(r.Context(), e.ID)
	return v
}

// envViews builds the decorated view list for all environments.
func (s *Server) envViews(r *http.Request) ([]envView, error) {
	envs, err := s.st.ListEnvironments(r.Context())
	if err != nil {
		return nil, err
	}
	views := make([]envView, 0, len(envs))
	for _, e := range envs {
		views = append(views, s.buildEnvView(r, e))
	}
	return views, nil
}

// partialEnvRows renders just the dashboard table rows, for htmx live refresh.
func (s *Server) partialEnvRows(w http.ResponseWriter, r *http.Request) {
	views, err := s.envViews(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if t := s.pages["dashboard.html"]; t != nil {
		_ = t.ExecuteTemplate(w, "env_rows", map[string]any{"Envs": views})
	}
}

func (s *Server) pageDashboard(w http.ResponseWriter, r *http.Request) {
	views, err := s.envViews(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, "dashboard.html", map[string]any{"Envs": views})
}

func (s *Server) pageEnvForm(w http.ResponseWriter, r *http.Request) {
	e := defaultEnv()
	editing := false
	if idStr := r.PathValue("id"); idStr != "" {
		id, err := parseID(idStr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		got, err := s.st.GetEnvironment(r.Context(), id)
		if err != nil || got == nil {
			http.NotFound(w, r)
			return
		}
		e = got
		editing = true
	} else {
		applyPreset(e, r.URL.Query().Get("preset")) // ?preset=git|image|compose
	}
	s.render(w, "env_form.html", map[string]any{"Env": e, "Editing": editing})
}

// applyPreset seeds a new env's source/deploy type from a quick-start choice.
func applyPreset(e *store.Environment, preset string) {
	switch preset {
	case "image":
		e.SourceType, e.DeployType = store.SourceRegistry, store.DeployContainer
	case "compose":
		e.SourceType, e.DeployType = store.SourceGit, store.DeployCompose
	default: // "git" or unset
		e.SourceType, e.DeployType = store.SourceGit, store.DeployContainer
	}
}

func (s *Server) pageEnvDetail(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	e, err := s.st.GetEnvironment(r.Context(), id)
	if err != nil || e == nil {
		http.NotFound(w, r)
		return
	}
	deps, _ := s.st.ListDeployments(r.Context(), id, 25)
	view := s.buildEnvView(r, e)

	hookRows, _ := s.st.ListTimedHooks(r.Context(), id)
	hooks := make([]hookView, 0, len(hookRows))
	for _, h := range hookRows {
		hv := hookView{TimedHook: h}
		hv.Last, _ = s.st.LatestHookRun(r.Context(), h.ID)
		hooks = append(hooks, hv)
	}

	// optional ?deploy=<id> to focus a specific deployment's live log
	var focus *store.Deployment
	if d := r.URL.Query().Get("deploy"); d != "" {
		if did, err := parseID(d); err == nil {
			focus, _ = s.st.GetDeployment(r.Context(), did)
		}
	} else if len(deps) > 0 {
		focus = deps[0]
	}

	base := s.extBase(r)
	webhookURL := base + "/hooks/" + itoa(id) + "?token=" + e.WebhookToken

	s.render(w, "env_detail.html", map[string]any{
		"View":        view,
		"Deployments": deps,
		"Focus":       focus,
		"WebhookURL":  webhookURL,
		"BaseURL":     base,
		"Host":        s.extHost(r),
		"Hooks":       hooks,
	})
}

// hookDeploy is the inbound webhook: POST /hooks/{id}?token=...
func (s *Server) hookDeploy(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	e, err := s.st.GetEnvironment(r.Context(), id)
	if err != nil || e == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	token := r.URL.Query().Get("token")
	if token == "" {
		token = r.Header.Get("X-Edp-Token")
	}
	if e.WebhookToken == "" || token != e.WebhookToken {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	reason := firstNonEmptyStr(r.URL.Query().Get("reason"), r.Header.Get("X-Edp-Reason"), "webhook")
	depID, err := s.engine.Trigger(r.Context(), id, store.TriggerWebhook, reason)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]int64{"deployment_id": depID})
}
