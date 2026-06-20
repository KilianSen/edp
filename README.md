# edp — Easy Deploy Platform

A single container that manages the lifecycle of **test environments** — build or
pull, deploy, redeploy, sweep volumes — with a web dashboard, an inbound webhook,
and a REST API. It is **not** a Portainer replacement; it's a focused, low-overhead
test-env deployer.

Each environment is defined once (target repo, build-or-pull, run config, redeploy
timer, setup/cleanup hooks, volume-sweep policy) and can be redeployed from the
dashboard, a webhook, or the API. edp controls the host's Docker daemon via the
mounted socket and labels everything it creates with `edp.managed=true` and
`edp.env=<id>`.

## Features

- **Two source modes** — git clone + `docker build`, or pull a prebuilt image from a registry.
- **Two env shapes** — a single container, or a `docker compose` stack.
- **Lifecycle hooks** — `setup` (before) and `cleanup` (after) **Python** scripts; `EDP_*` vars (env name, repo dir, …) are injected into `os.environ`.
- **Volume sweeps** — `none` / `named` / `all`, so test DBs can come up fresh on each deploy.
- **Redeploy timer** — per-env schedule as a Go duration (`30m`, `6h`) or cron (`0 */6 * * *`).
- **Health checks** — per-env `http` (GET, expect 2xx) or `exec` (`docker exec` into the live container); polled after deploy to define "ready" and time the start. (Single-container envs; compose stacks should use their own `healthcheck:` and are skipped.)
- **Start timing + ETA** — every deploy records its duration; the dashboard/splash show an ETA from the last few successful deploys.
- **Redeploy reasons** — every deploy records why it ran (manual note, `webhook`, or `scheduled (…)`).
- **Reverse proxy** — front a test env by hostname (Host header) or `/e/{name}/` path, over a shared Docker network. While an env is deploying or not yet up, edp serves a **"deploying ~Xs"/"starting"** splash with an ETA that auto-refreshes and proxies through once live.
- **Control surfaces** — htmx dashboard with **live** container status (pushed from Docker events over SSE) + streaming deploy logs, webhook, REST API.
- **Auth** — single admin password (bcrypt + signed session cookie) and per-env webhook tokens; a global API token for scripting.

## Run it

```bash
docker compose up -d --build
# open http://localhost:8080  — first visit sets the admin password
```

Or set the password explicitly (this also **resets** it if you're locked out —
the env var is authoritative and re-applied on every boot):

```bash
EDP_ADMIN_PASSWORD=your-strong-password docker compose up -d --build
```

> Forgot the password? Just set `EDP_ADMIN_PASSWORD` and restart — no need to wipe
> the data volume. To clear it again afterward, unset the var and restart.

`docker-compose.yml` mounts `/var/run/docker.sock` (to drive Docker), plus named
volumes for `/data` (SQLite DB) and `/workspace` (git checkouts / build contexts).

### Configuration (env vars)

| Var | Default | Purpose |
|-----|---------|---------|
| `EDP_ADDR` | `:8080` | HTTP listen address |
| `EDP_DATA_DIR` | `/data` | SQLite DB location |
| `EDP_WORKSPACE_DIR` | `/workspace` | git checkouts / build contexts |
| `EDP_ADMIN_PASSWORD` | — | admin password; **(re)applied on every boot** when set (use it to set or recover the password). If empty, set it in the browser on first visit. |
| `EDP_DOCKER_BIN` | `docker` | docker binary to shell out to |
| `EDP_PYTHON_BIN` | `python3` | interpreter used to run lifecycle hooks |
| `EDP_TRUST_PROXY` | `false` | trust `X-Forwarded-Proto`/`Host` — enable only behind a trusted reverse proxy (NPM, Traefik, …) |
| `EDP_IMPORT` | — | export bundle to load on startup — inline JSON or a path to a `.json` file (idempotent) |

## Defining an environment

Create one in the dashboard (**+ New**) or via the API. Key fields:

- **Source** — `git` (set repo URL, ref, optional access token) or `registry` (image + optional creds).
- **Deploy as** — `container` (Dockerfile path, build context, ports, env, volumes, restart policy) or `compose` (compose file path).
- **Lifecycle** — redeploy schedule, volume sweep, setup/cleanup scripts, prune toggle.

### Environment variables

Set `KEY=VALUE` lines on an env (Environment variables section) and they apply
everywhere that makes sense:

- **single container** → passed with `-e`;
- **compose stack** → put in the `docker compose` process environment, so `${VAR}`
  interpolation and `env_file` resolve (compose services still reference them as usual);
- **hooks** → injected into setup/cleanup and timed-hook scripts.

### Overrides

Some services need to run differently for testing than the repo ships. Each env can override:

- **Entrypoint / command** (container) — sets `--entrypoint` and the container args (quotes respected, e.g. `-c "npm run start:test"`); overrides the image's `ENTRYPOINT`/`CMD`.
- **Entrypoint script** (container) — paste a multi-line startup script; edp runs it as the container's entrypoint via `<interpreter> -c <script>` (interpreter from the Entrypoint field, default `/bin/sh`). Handy for wait-for-db / migrate / seed-then-start without baking it into an image.
- **Custom Dockerfile** (git build) — paste a Dockerfile and edp builds with it instead of the repo's (works even if the repo has none).
- **Compose override** (compose) — paste a compose file that's merged on top of the repo's via an extra `-f`, so you can tweak a service's image/env/command without touching the repo.

### Deploy pipeline

`setup hook → fetch source (clone/pull) → build/pull → replace container / compose up → volume sweep → cleanup hook → optional image prune`, recorded as a deployment with a streamed log and the resulting commit SHA / image digest.

## REST API

Authenticate with the session cookie or `Authorization: Bearer <api_token>` (the
token is generated on first boot — read it from the `settings` table or a future
settings page).

```
GET    /api/environments
POST   /api/environments
GET    /api/environments/{id}
PUT    /api/environments/{id}
DELETE /api/environments/{id}
POST   /api/environments/{id}/deploy
POST   /api/environments/{id}/rotate-token
GET    /api/environments/{id}/deployments
GET    /api/deployments/{id}
GET    /api/deployments/{id}/logs/stream      # SSE
```

### Import / export

Back up, share, or move environments between edp instances as JSON.

- **Export** — `GET /api/export` (all) or `GET /api/environments/{id}/export` (one) downloads a
  bundle including each env's **timed hooks**. Credentials (git/registry tokens), webhook tokens,
  ids, and status are **always stripped** — exports never carry secrets, so they're safe to share.
- **Import** — `POST /api/import` (or the **Import** page) accepts a bundle, a bare array, or a
  single env object. Each env is created with a unique name and a fresh webhook token; hooks are
  recreated. Credentials **are** imported if present in the JSON (and encrypted at rest), so a
  hand-prepared bundle can carry tokens for a full migration.
- **Load on startup** — set `EDP_IMPORT` to a bundle (inline JSON or a mounted file path) and edp
  creates those environments on boot. It's **idempotent** — envs whose name already exists are
  left untouched — so you can leave it set for declarative, infra-as-code provisioning.

Tick **Auto-deploy** (`"auto_deploy": true`) on an env and edp deploys it automatically whenever it's
**created, imported, or loaded on startup** — so `EDP_IMPORT` + auto-deploy brings a fresh edp up
with environments already built and running, no manual Redeploy.

### Webhook

```
POST /hooks/{envId}?token=<webhook_token>
```

Each environment has its own rotatable token (shown on the env detail page). Point
your git provider / CI at this URL to trigger a redeploy. A wrong or missing token
returns `403`.

## Reverse proxy

Set a **container port** (and optionally a **proxy host**) on an environment to
front it through edp:

- **Path-based:** `http://<edp-host>/e/<name>/` → the env's container.
- **Host-based:** point a hostname (e.g. `app.test.local`) at edp and set it as the env's proxy host; edp routes by `Host`.

edp reaches the container **by name over a shared Docker network** (`edp`), which
it creates and joins on startup and attaches proxied containers to on deploy — so
no published host port is required. While an env is deploying or not yet running,
edp serves a splash page (HTTP 503) showing **"deploying ~Xs remaining"** (or
"starting") with an ETA, auto-refreshing until the app is live.

### Raw TCP port forwarding

For non-HTTP services (or if you just want a plain port), set a **listen port** on
the env in addition to the container port — any protocol, no Host/path rewriting
(and no splash).

Each forward runs as its own tiny **sidecar container** (`edp-fwd-<id>`) that
publishes *only* that one port and pipes it to the env's container over the shared
network. So:

- **Only ports actually in use are ever open** — no port range, nothing extra exposed.
- **Fully dynamic** — adding or removing a forward needs no edp restart and no
  pre-published ports; edp reconciles the sidecars within a few seconds.

The sidecars run the edp image itself (no extra dependency); edp auto-detects that
image, or set `EDP_IMAGE` to pin it. They use `--restart unless-stopped`, so
forwards survive an edp restart and are reclaimed on the next reconcile.

> Proxied apps are served without edp's login (they're the test apps themselves) —
> keep edp on a trusted network/VPN. Compose-stack proxying isn't wired yet
> (single-container envs only).

### Clear browser data on redeploy

Tick **Clear browser data on redeploy** (cookies / localStorage+IndexedDB / cache /
force-reload) on an env. After each redeploy, the next page load through the HTTP
proxy sends a [`Clear-Site-Data`](https://developer.mozilla.org/docs/Web/HTTP/Headers/Clear-Site-Data)
header so the browser drops that origin's stored data and you test from a clean
slate. It fires **once per redeploy**, only on a top-level navigation (not assets/XHR),
and only for the HTTP reverse proxy (raw TCP forwards can't carry the header).

## Behind an existing reverse proxy (NPM, Traefik, …)

edp can run behind another reverse proxy — e.g. **Nginx Proxy Manager** terminating
TLS for the dashboard and/or the test-env hostnames. Set **`EDP_TRUST_PROXY=1`** and
edp will:

- use `X-Forwarded-Proto`/`X-Forwarded-Host` for the real external scheme + host, so
  generated **webhook and proxy URLs** are correct;
- mark the **session cookie `Secure`** when the external connection is HTTPS;
- match **host-based env routing** against `X-Forwarded-Host`, so NPM can forward an
  env's hostname to edp and edp routes it to the right container;
- pass `X-Forwarded-Proto`/`Host` through to the proxied app.

Point your upstream proxy at edp's `8080` and forward the host header (or set
`X-Forwarded-Host`). Leave `EDP_TRUST_PROXY` **off** when edp is exposed directly —
otherwise clients could spoof those headers.

## Architecture

```
Go binary (CGO-free) ── embedded dashboard (html/template + htmx, vendored)
  ├─ HTTP server: dashboard, /api/*, /hooks/*, SSE (logs + status)
  ├─ Deploy engine: serialized per env; shells out to docker / docker compose / git
  ├─ Events watcher: streams `docker events` → status hub → dashboard (live dots)
  ├─ Scheduler: per-env redeploy timer (duration or cron)
  ├─ Secret box: AES-256-GCM for credentials at rest
  └─ Store: SQLite (modernc.org/sqlite, no CGO)
```

Source layout: `cmd/edp` (entrypoint), `internal/{config,store,secret,docker,source,sh,deploy,scheduler,events,statushub,logbus,server}`.

## Security notes

- Intended for **internal/trusted-network** test infrastructure. Put it behind a VPN or reverse proxy with TLS if exposed.
- Mounting the Docker socket grants the container root-equivalent control of the host — treat the admin password and API token accordingly.
- Credentials (git/registry) are encrypted at rest (AES-256-GCM) with a key file (`/data/secret.key`, mode 0600) kept separate from the SQLite DB, and are never returned by the API. Keep the `/data` volume private — anyone with both the DB and the key file can decrypt.

## Build / develop locally

```bash
go build ./...
go build -o edp ./cmd/edp
EDP_DATA_DIR=./data EDP_WORKSPACE_DIR=./ws EDP_ADMIN_PASSWORD=devpass123 ./edp
```

Note: lifecycle hooks run via `python3`, so a full deploy is best exercised inside
the Linux container (the published image bundles `docker-cli`,
`docker-cli-compose`, `git`, and `python3`).

### Styling

The UI is styled with **Tailwind CSS v4**, precompiled to `internal/server/static/app.css`
(committed, so builds need no Node/network). After editing templates, regenerate it with the
standalone Tailwind CLI:

```bash
tailwindcss -i internal/server/styles/input.css -o internal/server/static/app.css --minify
```

Design tokens (palette, fonts) live at the top of `internal/server/styles/input.css`.
