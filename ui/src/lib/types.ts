// Mirrors edp/internal/store/models.go. Credentials (git_token, registry_password)
// are accepted on write but never returned by the API, so they are write-only here.

export interface Environment {
  id: number;
  name: string;
  source_type: string; // git | registry | dockerfile
  deploy_type: string; // container | compose

  repo_url: string;
  git_ref: string;
  git_token?: string; // write-only

  registry_image: string;
  registry_username: string;
  registry_password?: string; // write-only

  dockerfile_path: string;
  build_context: string;
  image_name: string;
  run_ports: string;
  run_env: string;
  run_volumes: string;
  run_networks: string;
  restart_policy: string;

  entrypoint: string;
  command: string;
  entrypoint_script: string;
  dockerfile_content: string;
  compose_override: string;

  compose_path: string;

  redeploy_schedule: string;
  volume_sweep: string; // none | named | all
  setup_script: string;
  cleanup_script: string;
  prune_images: boolean;
  auto_deploy: boolean;

  health_type: string; // none | http | exec
  health_target: string;

  proxy_host: string;
  proxy_port: string;
  listen_port: string;
  clear_site_data: string; // comma list
  splash_url: string;

  webhook_token: string;
  status: string;
  created_at: string;
  updated_at: string;
}

export interface Deployment {
  id: number;
  env_id: number;
  trigger: string; // manual | webhook | schedule
  reason: string;
  status: string; // queued | running | success | failed
  started_at: string | null;
  finished_at: string | null;
  log: string;
  commit_sha: string;
  image_digest: string;
  duration_ms: number;
  ready_ms: number;
}

export interface TimedHook {
  id: number;
  env_id: number;
  name: string;
  schedule: string;
  script: string;
  enabled: boolean;
  status: string;
  created_at: string;
  updated_at: string;
}

export interface HookRun {
  id: number;
  hook_id: number;
  trigger: string; // manual | schedule
  status: string;
  started_at: string | null;
  finished_at: string | null;
  log: string;
}

export interface StatusItem {
  id: number;
  state: string;
  info: string;
}

// Mirrors server.overviewItem — the decorated env list backing the dashboard.
export interface OverviewItem {
  env: Environment;
  container_state: string;
  container_info: string;
  last: Deployment | null;
  estimate_ms: number;
}

// A blank environment matching the API's defaultEnv(), for the create form.
export function blankEnv(): Environment {
  return {
    id: 0,
    name: "",
    source_type: "git",
    deploy_type: "container",
    repo_url: "",
    git_ref: "",
    git_token: "",
    registry_image: "",
    registry_username: "",
    registry_password: "",
    dockerfile_path: "",
    build_context: ".",
    image_name: "",
    run_ports: "",
    run_env: "",
    run_volumes: "",
    run_networks: "",
    restart_policy: "unless-stopped",
    entrypoint: "",
    command: "",
    entrypoint_script: "",
    dockerfile_content: "",
    compose_override: "",
    compose_path: "",
    redeploy_schedule: "",
    volume_sweep: "none",
    setup_script: "",
    cleanup_script: "",
    prune_images: false,
    auto_deploy: false,
    health_type: "none",
    health_target: "",
    proxy_host: "",
    proxy_port: "",
    listen_port: "",
    clear_site_data: "",
    splash_url: "",
    webhook_token: "",
    status: "",
    created_at: "",
    updated_at: "",
  };
}
