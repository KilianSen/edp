package store

import (
	"encoding/json"
	"time"
)

// Environment is a managed test environment definition.
type Environment struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	SourceType string `json:"source_type"` // git | registry
	DeployType string `json:"deploy_type"` // container | compose

	// git source
	RepoURL  string `json:"repo_url"`
	GitRef   string `json:"git_ref"`
	GitToken string `json:"git_token,omitempty"`

	// registry source
	RegistryImage    string `json:"registry_image"`
	RegistryUsername string `json:"registry_username"`
	RegistryPassword string `json:"registry_password,omitempty"`

	// build / run (container)
	DockerfilePath string `json:"dockerfile_path"`
	BuildContext   string `json:"build_context"`
	ImageName      string `json:"image_name"`
	RunPorts       string `json:"run_ports"`    // "8080:80,9000:9000"
	RunEnv         string `json:"run_env"`      // KEY=VAL per line
	RunVolumes     string `json:"run_volumes"`  // "name:/path" per line
	RunNetworks    string `json:"run_networks"` // comma separated
	RestartPolicy  string `json:"restart_policy"`

	// overrides — replace what the repo/image ships
	Entrypoint        string `json:"entrypoint"`         // docker run --entrypoint
	Command           string `json:"command"`            // args appended after the image
	EntrypointScript  string `json:"entrypoint_script"`  // inline script run as the entrypoint: <interpreter> -c <script>
	DockerfileContent string `json:"dockerfile_content"` // inline Dockerfile, built instead of the repo's
	ComposeOverride   string `json:"compose_override"`   // inline compose file merged via an extra -f

	// compose
	ComposePath string `json:"compose_path"`

	// lifecycle
	RedeploySchedule string `json:"redeploy_schedule"` // cron expr or Go duration; empty = none
	VolumeSweep      string `json:"volume_sweep"`      // none | named | all
	SetupScript      string `json:"setup_script"`
	CleanupScript    string `json:"cleanup_script"`
	PruneImages      bool   `json:"prune_images"`
	AutoDeploy       bool   `json:"auto_deploy"` // deploy automatically when created/imported/loaded

	// health check: how edp decides an env is "ready" after deploy
	HealthType   string `json:"health_type"`   // none | http | exec
	HealthTarget string `json:"health_target"` // URL (http) or command (exec)

	// reverse proxy: edp fronts the env's container on the shared network
	ProxyHost string `json:"proxy_host"` // Host header to match (host-based routing)
	ProxyPort string `json:"proxy_port"` // container's internal port to target; empty = proxy off
	// port proxying: edp listens on ListenPort and TCP-forwards to ProxyPort
	ListenPort string `json:"listen_port"` // host-facing TCP port; empty = no forward
	// ClearSiteData: comma list of Clear-Site-Data directives to emit once after
	// a redeploy (cache,cookies,storage,executionContexts); empty = off.
	ClearSiteData string `json:"clear_site_data"`

	WebhookToken string    `json:"webhook_token"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// MarshalJSON omits credentials from API output (they are still accepted on
// input via the default unmarshal, and remain available to server-rendered
// templates that read the struct fields directly).
func (e Environment) MarshalJSON() ([]byte, error) {
	type alias Environment
	c := alias(e)
	c.GitToken = ""
	c.RegistryPassword = ""
	return json.Marshal(c)
}

// Deployment is one run of the deploy engine for an environment.
type Deployment struct {
	ID          int64      `json:"id"`
	EnvID       int64      `json:"env_id"`
	Trigger     string     `json:"trigger"` // manual | webhook | schedule
	Reason      string     `json:"reason"`  // human-supplied or derived reason for the deploy
	Status      string     `json:"status"`  // queued | running | success | failed
	StartedAt   *time.Time `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at"`
	Log         string     `json:"log"`
	CommitSHA   string     `json:"commit_sha"`
	ImageDigest string     `json:"image_digest"`
	DurationMs  int64      `json:"duration_ms"` // total deploy time
	ReadyMs     int64      `json:"ready_ms"`    // time from start until health passed (0 if no health check)
}

const (
	HealthNone = "none"
	HealthHTTP = "http"
	HealthExec = "exec"
)

// TimedHook is a scheduled script that runs against an environment without
// triggering a redeploy (e.g. periodic DB cleanup, cache warming, health pings).
type TimedHook struct {
	ID        int64     `json:"id"`
	EnvID     int64     `json:"env_id"`
	Name      string    `json:"name"`
	Schedule  string    `json:"schedule"` // duration or cron; empty = manual only
	Script    string    `json:"script"`
	Enabled   bool      `json:"enabled"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// HookRun is one execution of a TimedHook.
type HookRun struct {
	ID         int64      `json:"id"`
	HookID     int64      `json:"hook_id"`
	Trigger    string     `json:"trigger"` // manual | schedule
	Status     string     `json:"status"`
	StartedAt  *time.Time `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
	Log        string     `json:"log"`
}

const (
	SourceGit        = "git"
	SourceRegistry   = "registry"
	SourceDockerfile = "dockerfile" // build from inline dockerfile_content, no repo

	DeployContainer = "container"
	DeployCompose   = "compose"

	SweepNone  = "none"
	SweepNamed = "named"
	SweepAll   = "all"

	TriggerManual   = "manual"
	TriggerWebhook  = "webhook"
	TriggerSchedule = "schedule"

	StatusQueued  = "queued"
	StatusRunning = "running"
	StatusSuccess = "success"
	StatusFailed  = "failed"
)
