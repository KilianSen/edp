package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"edp/internal/store"
)

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) apiListEnvs(w http.ResponseWriter, r *http.Request) {
	envs, err := s.st.ListEnvironments(r.Context())
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, envs)
}

func (s *Server) apiGetEnv(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	env, err := s.st.GetEnvironment(r.Context(), id)
	if err != nil || env == nil {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, 200, env)
}

func (s *Server) apiCreateEnv(w http.ResponseWriter, r *http.Request) {
	e := defaultEnv()
	if err := bindEnv(r, e); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if e.Name == "" {
		writeJSON(w, 400, map[string]string{"error": "name is required"})
		return
	}
	if e.WebhookToken == "" {
		e.WebhookToken = randomToken()
	}
	if err := s.st.CreateEnvironment(r.Context(), e); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	s.maybeAutoDeploy(r.Context(), e, "auto-deploy on create")
	if wantsHTML(r) {
		http.Redirect(w, r, "/env/"+itoa(e.ID), http.StatusSeeOther)
		return
	}
	writeJSON(w, 201, e)
}

// maybeAutoDeploy triggers an initial deploy if the env opted into auto-deploy.
func (s *Server) maybeAutoDeploy(ctx context.Context, e *store.Environment, reason string) {
	if e.AutoDeploy {
		if _, err := s.engine.Trigger(ctx, e.ID, store.TriggerManual, reason); err != nil {
			log.Printf("auto-deploy env %d: %v", e.ID, err)
		}
	}
}

func (s *Server) apiUpdateEnv(w http.ResponseWriter, r *http.Request) {
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
	if err := bindEnv(r, e); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if err := s.st.UpdateEnvironment(r.Context(), e); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if wantsHTML(r) {
		http.Redirect(w, r, "/env/"+itoa(e.ID), http.StatusSeeOther)
		return
	}
	writeJSON(w, 200, e)
}

func (s *Server) apiDeleteEnv(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if err := s.st.DeleteEnvironment(r.Context(), id); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "deleted"})
}

func (s *Server) apiDeploy(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	reason := firstNonEmptyStr(strings.TrimSpace(r.FormValue("reason")), "manual")
	depID, err := s.engine.Trigger(r.Context(), id, store.TriggerManual, reason)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if isHTMX(r) {
		w.Header().Set("HX-Trigger", "refresh") // nudge the dashboard to refresh rows
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if wantsHTML(r) {
		http.Redirect(w, r, "/env/"+itoa(id)+"?deploy="+itoa(depID), http.StatusSeeOther)
		return
	}
	writeJSON(w, 202, map[string]int64{"deployment_id": depID})
}

func (s *Server) apiRotateToken(w http.ResponseWriter, r *http.Request) {
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
	e.WebhookToken = randomToken()
	if err := s.st.UpdateEnvironment(r.Context(), e); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if wantsHTML(r) {
		http.Redirect(w, r, "/env/"+itoa(id), http.StatusSeeOther)
		return
	}
	writeJSON(w, 200, map[string]string{"webhook_token": e.WebhookToken})
}

func (s *Server) apiListDeployments(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	deps, err := s.st.ListDeployments(r.Context(), id, 50)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, deps)
}

func (s *Server) apiGetDeployment(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	dep, err := s.st.GetDeployment(r.Context(), id)
	if err != nil || dep == nil {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, 200, dep)
}

// ---- binding ----

func defaultEnv() *store.Environment {
	return &store.Environment{
		SourceType:    store.SourceGit,
		DeployType:    store.DeployContainer,
		BuildContext:  ".",
		RestartPolicy: "unless-stopped",
		VolumeSweep:   store.SweepNone,
		WebhookToken:  randomToken(),
	}
}

// bindEnv populates e from either a JSON body or a posted form.
func bindEnv(r *http.Request, e *store.Environment) error {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return json.NewDecoder(r.Body).Decode(e)
	}
	if err := r.ParseForm(); err != nil {
		return err
	}
	f := r.PostForm
	set(&e.Name, f, "name")
	set(&e.SourceType, f, "source_type")
	set(&e.DeployType, f, "deploy_type")
	set(&e.RepoURL, f, "repo_url")
	set(&e.GitRef, f, "git_ref")
	set(&e.GitToken, f, "git_token")
	set(&e.RegistryImage, f, "registry_image")
	set(&e.RegistryUsername, f, "registry_username")
	set(&e.RegistryPassword, f, "registry_password")
	set(&e.DockerfilePath, f, "dockerfile_path")
	set(&e.BuildContext, f, "build_context")
	set(&e.ImageName, f, "image_name")
	set(&e.Entrypoint, f, "entrypoint")
	set(&e.Command, f, "command")
	set(&e.EntrypointScript, f, "entrypoint_script")
	set(&e.DockerfileContent, f, "dockerfile_content")
	set(&e.ComposeOverride, f, "compose_override")
	set(&e.RunPorts, f, "run_ports")
	set(&e.RunEnv, f, "run_env")
	set(&e.RunVolumes, f, "run_volumes")
	set(&e.RunNetworks, f, "run_networks")
	set(&e.RestartPolicy, f, "restart_policy")
	set(&e.ComposePath, f, "compose_path")
	set(&e.RedeploySchedule, f, "redeploy_schedule")
	set(&e.VolumeSweep, f, "volume_sweep")
	set(&e.SetupScript, f, "setup_script")
	set(&e.CleanupScript, f, "cleanup_script")
	set(&e.HealthType, f, "health_type")
	set(&e.HealthTarget, f, "health_target")
	set(&e.ProxyHost, f, "proxy_host")
	set(&e.ProxyPort, f, "proxy_port")
	set(&e.ListenPort, f, "listen_port")
	e.ClearSiteData = strings.Join(f["clear_site_data"], ",") // multiple checkboxes
	e.PruneImages = f.Get("prune_images") != ""
	e.AutoDeploy = f.Get("auto_deploy") != ""
	return nil
}

func set(dst *string, f map[string][]string, key string) {
	if v, ok := f[key]; ok && len(v) > 0 {
		*dst = strings.TrimSpace(v[0])
	}
}

func wantsHTML(r *http.Request) bool {
	ct := r.Header.Get("Content-Type")
	return strings.HasPrefix(ct, "application/x-www-form-urlencoded") || strings.HasPrefix(ct, "multipart/form-data")
}

func isHTMX(r *http.Request) bool { return r.Header.Get("HX-Request") == "true" }

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
