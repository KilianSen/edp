// An edp instance registered with the manager. The api_token is write-only:
// the API never returns it.
export interface Instance {
  id: number;
  label: string;
  base_url: string;
  api_token?: string;
  created_at?: string;
  updated_at?: string;
}

// One instance failing during a fan-out (the rest still return).
export interface InstanceError {
  instance_id: number;
  instance_label: string;
  error: string;
}

// A merged fan-out row: an edp object (env/status/overview) plus the tags the
// manager adds identifying which instance it came from. Fields are open because
// the manager passes edp's payloads through generically.
export type Tagged = Record<string, unknown> & {
  instance_id: number;
  instance_label: string;
};

export interface Fanout {
  items: Tagged[];
  errors: InstanceError[];
}

// Fleet health: per-instance reachability + environment counts by status.
export interface InstanceHealth {
  instance_id: number;
  instance_label: string;
  base_url: string;
  reachable: boolean;
  error?: string;
  environments: number;
  by_status: Record<string, number>;
}

export interface Totals {
  instances: number;
  reachable: number;
  environments: number;
  by_status: Record<string, number>;
}

export interface Summary {
  instances: InstanceHealth[];
  totals: Totals;
}

// ---- edp domain types (mirror edp/internal/store/models.go) ----
// Credentials (git_token, registry_password) are write-only; edp never returns them.

export interface Environment {
  id: number;
  name: string;
  source_type: string; // git | registry | dockerfile
  deploy_type: string; // container | compose
  repo_url: string;
  git_ref: string;
  registry_image: string;
  registry_username: string;
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
  compose_path: string;
  redeploy_schedule: string;
  volume_sweep: string;
  prune_images: boolean;
  auto_deploy: boolean;
  health_type: string;
  health_target: string;
  proxy_host: string;
  proxy_port: string;
  listen_port: string;
  splash_url: string;
  webhook_token: string;
  status: string;
  created_at: string;
  updated_at: string;

  // write-only on edp (accepted on save, never returned) — present for the form
  git_token?: string;
  registry_password?: string;
  dockerfile_content?: string;
  compose_override?: string;
  setup_script?: string;
  cleanup_script?: string;
  clear_site_data?: string;
}

// blankEnv matches edp's defaultEnv(), for the create form.
export function blankEnv(): Environment {
  return {
    id: 0,
    name: "",
    source_type: "git",
    deploy_type: "container",
    repo_url: "",
    git_ref: "",
    registry_image: "",
    registry_username: "",
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
    compose_path: "",
    redeploy_schedule: "",
    volume_sweep: "none",
    prune_images: false,
    auto_deploy: false,
    health_type: "none",
    health_target: "",
    proxy_host: "",
    proxy_port: "",
    listen_port: "",
    splash_url: "",
    webhook_token: "",
    status: "",
    created_at: "",
    updated_at: "",
    git_token: "",
    registry_password: "",
    dockerfile_content: "",
    compose_override: "",
    setup_script: "",
    cleanup_script: "",
    clear_site_data: "",
  };
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

// edp's decorated env list item (server.overviewItem), as returned by /api/overview.
export interface OverviewItem {
  env: Environment;
  container_state: string;
  container_info: string;
  last: Deployment | null;
  estimate_ms: number;
}

// An overview item with the manager's instance tags merged in.
export type ManagedOverview = OverviewItem & {
  instance_id: number;
  instance_label: string;
};

export interface OverviewFanout {
  items: ManagedOverview[];
  errors: InstanceError[];
}
