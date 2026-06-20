CREATE TABLE IF NOT EXISTS settings (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS environments (
  id                INTEGER PRIMARY KEY AUTOINCREMENT,
  name              TEXT    NOT NULL UNIQUE,
  source_type       TEXT    NOT NULL DEFAULT 'git',
  deploy_type       TEXT    NOT NULL DEFAULT 'container',
  repo_url          TEXT    NOT NULL DEFAULT '',
  git_ref           TEXT    NOT NULL DEFAULT '',
  git_token         TEXT    NOT NULL DEFAULT '',
  registry_image    TEXT    NOT NULL DEFAULT '',
  registry_username TEXT    NOT NULL DEFAULT '',
  registry_password TEXT    NOT NULL DEFAULT '',
  dockerfile_path   TEXT    NOT NULL DEFAULT '',
  build_context     TEXT    NOT NULL DEFAULT '.',
  image_name        TEXT    NOT NULL DEFAULT '',
  run_ports         TEXT    NOT NULL DEFAULT '',
  run_env           TEXT    NOT NULL DEFAULT '',
  run_volumes       TEXT    NOT NULL DEFAULT '',
  run_networks      TEXT    NOT NULL DEFAULT '',
  restart_policy    TEXT    NOT NULL DEFAULT 'unless-stopped',
  compose_path      TEXT    NOT NULL DEFAULT '',
  redeploy_schedule TEXT    NOT NULL DEFAULT '',
  volume_sweep      TEXT    NOT NULL DEFAULT 'none',
  setup_script      TEXT    NOT NULL DEFAULT '',
  cleanup_script    TEXT    NOT NULL DEFAULT '',
  prune_images      INTEGER NOT NULL DEFAULT 0,
  health_type       TEXT    NOT NULL DEFAULT 'none',
  health_target     TEXT    NOT NULL DEFAULT '',
  proxy_host        TEXT    NOT NULL DEFAULT '',
  proxy_port        TEXT    NOT NULL DEFAULT '',
  listen_port       TEXT    NOT NULL DEFAULT '',
  clear_site_data   TEXT    NOT NULL DEFAULT '',
  splash_url        TEXT    NOT NULL DEFAULT '',
  entrypoint        TEXT    NOT NULL DEFAULT '',
  command           TEXT    NOT NULL DEFAULT '',
  entrypoint_script TEXT    NOT NULL DEFAULT '',
  dockerfile_content TEXT   NOT NULL DEFAULT '',
  compose_override  TEXT    NOT NULL DEFAULT '',
  auto_deploy       INTEGER NOT NULL DEFAULT 0,
  webhook_token     TEXT    NOT NULL DEFAULT '',
  status            TEXT    NOT NULL DEFAULT 'idle',
  created_at        TEXT    NOT NULL,
  updated_at        TEXT    NOT NULL
);

CREATE TABLE IF NOT EXISTS deployments (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  env_id       INTEGER NOT NULL,
  trigger      TEXT    NOT NULL,
  status       TEXT    NOT NULL,
  started_at   TEXT,
  finished_at  TEXT,
  log          TEXT    NOT NULL DEFAULT '',
  commit_sha   TEXT    NOT NULL DEFAULT '',
  image_digest TEXT    NOT NULL DEFAULT '',
  reason       TEXT    NOT NULL DEFAULT '',
  duration_ms  INTEGER NOT NULL DEFAULT 0,
  ready_ms     INTEGER NOT NULL DEFAULT 0,
  FOREIGN KEY(env_id) REFERENCES environments(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_deployments_env ON deployments(env_id, id DESC);

-- Timed hooks: scheduled scripts that run against an env WITHOUT redeploying it.
CREATE TABLE IF NOT EXISTS timed_hooks (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  env_id     INTEGER NOT NULL,
  name       TEXT    NOT NULL,
  schedule   TEXT    NOT NULL DEFAULT '',
  script     TEXT    NOT NULL DEFAULT '',
  enabled    INTEGER NOT NULL DEFAULT 1,
  status     TEXT    NOT NULL DEFAULT 'idle',
  created_at TEXT    NOT NULL,
  updated_at TEXT    NOT NULL,
  UNIQUE(env_id, name),
  FOREIGN KEY(env_id) REFERENCES environments(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS hook_runs (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  hook_id     INTEGER NOT NULL,
  trigger     TEXT    NOT NULL,
  status      TEXT    NOT NULL,
  started_at  TEXT,
  finished_at TEXT,
  log         TEXT    NOT NULL DEFAULT '',
  FOREIGN KEY(hook_id) REFERENCES timed_hooks(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_hook_runs_hook ON hook_runs(hook_id, id DESC);
