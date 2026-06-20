# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

edp ("Easy Deploy Platform") is a single Go container that manages the lifecycle of **test environments** — build/pull an image, deploy it as a container or compose stack, redeploy on a timer/webhook/API call, sweep volumes, and front it with a reverse proxy. It drives the host Docker daemon via the mounted `/var/run/docker.sock` and labels everything it creates with `edp.managed=true` / `edp.env=<id>`. The README is the authoritative product/feature reference; this file covers how the code is built and wired.

## Commands

This is the `edp/` subproject of the monorepo (its own Go module); run these from
`edp/`. Image builds, however, use the **repo root** as context (see below).

```bash
go build ./...                 # compile everything
go build -o edp ./cmd/edp      # build the binary
go vet ./...                   # lint (CI runs this)
go test ./...                  # run all tests
go test ./internal/proxy/...   # one package
go test -run TestName ./internal/deploy/   # one test

# run locally (hooks need python3 on PATH; full deploys need docker + git)
EDP_DATA_DIR=./data EDP_WORKSPACE_DIR=./ws EDP_ADMIN_PASSWORD=devpass123 ./edp

docker compose up -d --build   # run the full container (recommended for real deploys)
```

CI is the monorepo-wide `.github/workflows/release.yml` (at the repo root, not here): on push to `master` it vets/builds/tests both Go modules, builds both UIs, versions via **Conventional Commits** (`fix:`→patch, `feat:`→minor, `feat!:`/`BREAKING CHANGE`→major), and pushes multi-arch images for every subproject to GHCR. See the root `CLAUDE.md`. Commit messages drive releases — follow the convention.

### Dashboard (separate project) + two images

There is **no embedded UI** in this binary. The dashboard is the React + TypeScript + Vite (Tailwind v4) app in the repo-root **`../ui`** directory, which consumes the JSON API with a Bearer token. Edit CSS/UI there, not here. This Go module is rooted in `edp/`; its import paths (`edp/internal/...`) are independent of the directory, so the monorepo move needed no code changes.

The same binary ships as two images, differing only by `EDP_UI_DIR`. Both build with the **monorepo root as context** (the service is in `edp/`, the dashboard in `ui/`):
- **headless** (`edp/Dockerfile`) — API + webhooks + proxy/splash; `/` 404s.
- **bundled UI** (`edp/Dockerfile.ui`) — also serves a built dashboard at `/`. It builds `ui/` (Node stage), copies its `dist/` into the runtime image, and sets `EDP_UI_DIR=/usr/share/edp-ui`. The root `.dockerignore` excludes `ui/node_modules` + `ui/dist` (and the sibling subprojects) so the context stays lean.

When `EDP_UI_DIR` is set, `Handler()` registers `spaHandler(dir)` (`server/spa.go`) as the mux `/` fallback — static files with index.html SPA fallback. It sits **behind** the `/api` + `/hooks` routes (more specific patterns win) and **behind** the reverse proxy (still the outermost handler, so host-/path-based test-env routing keeps working on the same port). Same-origin, so no CORS needed in the bundled image.

## Architecture

A single CGO-free binary (`cmd/edp/main.go`). The driver is `modernc.org/sqlite` (pure Go) specifically to keep `CGO_ENABLED=0`; the Dockerfile cross-compiles via `GOARCH` and the runtime image only adds `docker-cli`, `docker-cli-compose`, `git`, and `python3` because edp **shells out** to those tools rather than using Docker libraries.

`main.go` wires the components and owns their lifecycles:

- **`config`** — all runtime config comes from `EDP_*` env vars (see README table); `config.Load()` is the single source.
- **`store`** — SQLite persistence. `schema.sql` is embedded and applied on open; new columns are added via the **`migrate()` ADD-COLUMN pattern** (idempotent — duplicate-column errors are swallowed). When adding a field to an environment/deployment, add it to `schema.sql` *and* append an `ALTER TABLE … ADD COLUMN` to `migrate()` in `store/store.go`. `MaxOpenConns(1)` — SQLite is single-writer, all access is serialized.
- **`secret`** — AES-256-GCM box for credentials (git/registry tokens) at rest, keyed by `/data/secret.key` (mode 0600). Store encrypts on write, decrypts on read; the API never returns secrets.
- **`deploy`** — the heart. `Engine.Trigger` queues a deployment row and runs it async. Deploys are **serialized per-env** (a `sync.Mutex` per env ID) but different envs run concurrently. The pipeline lives in `deploy/steps.go`: `setup hook → fetch source → build/pull → replace container or compose up → volume sweep → cleanup hook → health check → optional prune`. Health checks (`deploy/health.go`) define "ready" and record `readyMs` for the ETA feature.
- **`hooks`** — user lifecycle scripts run as **Python** (`python3 -c <script>`) with `EDP_*` vars injected into `os.environ`. Includes scheduled "timed hooks" separate from the deploy pipeline.
- **`scheduler`** — per-env redeploy timer; accepts a Go duration (`30m`) or cron (`0 */6 * * *`) via `robfig/cron`.
- **`docker`** — thin wrappers over the docker CLI. Constants `LabelManaged`/`LabelEnv` and `SharedNetwork` (the `edp` network) live here; all created resources carry the labels so edp can find/reap them.
- **`events`** — tails `docker events` and pushes container status into `statushub`, which fans out to the dashboard over SSE (the live status dots).
- **`logbus`** — pub/sub for streaming deploy/hook logs to SSE clients while also persisting them.
- **`proxy`** + **`portproxy`** — `proxy` is an HTTP reverse proxy that wraps the whole mux (see below); `portproxy` manages raw-TCP forwards as per-env **sidecar containers** (`edp-fwd-<id>`) that run the edp image itself in `forward` mode. The proxy serves the "deploying" interstitial when an env is down: either the built-in embedded `splash.html`, or — when `EDP_SPLASH_URL`/per-env `splash_url` is set — a 302 to an external UI. That UI drives redeploy/status via reserved per-env control endpoints on the proxied origin (`<env>/_edp/status`, `<env>/_edp/redeploy`), gated by the env's webhook token (not admin creds) and CORS-enabled (`EDP_CORS_ORIGINS`). A standalone reference UI lives in `splash/` in this monorepo (dependency-free static HTML/CSS/JS + nginx Dockerfile, no build step); only the minimal `splash.html` fallback stays embedded here.
- **`server`** — the JSON API and webhook endpoints (no HTML; the dashboard is the separate `edp-ui` SPA). `Handler()` wraps the mux in `corsMiddleware` (reusing `EDP_CORS_ORIGINS`) so a cross-origin `edp-ui` can call `/api/*` with a Bearer token. `POST /api/login` exchanges the admin password for the global API token; `GET /api/auth` reports first-run. SSE endpoints are consumed by `edp-ui` via `fetch`+stream (not `EventSource`, which can't send the auth header). `apiOverview` (`GET /api/overview`) returns the decorated env list (container state, last deployment, ETA) the dashboard needs in one shot.

### Two things that are easy to miss

1. **The reverse proxy is the outermost handler.** `server.Handler()` returns `proxy.New(..., logRequests(mux))`. Proxied test-env traffic (by Host header or `/e/{name}/` path) is handled by the proxy; everything else falls through to the dashboard/API mux. Auth applies to the mux, not to proxied apps.

2. **The binary has sub-command modes.** `main.go` checks `os.Args` *before* normal startup: `edp forward <listen> <target>` runs only the TCP forwarder (this is what the `portproxy` sidecar containers execute), and `edp reap` runs a one-shot teardown and exits. Don't add startup work above those checks.

3. **Everything edp creates is scoped by `edp.instance`.** Each instance has a stable id (`store.InstanceID`, persisted in the `instance_id` setting in `/data`, so it survives container recreation). Every container edp creates — single-container envs (`deploy.containerRunArgs`) and forwarder sidecars (`docker.StartForwarder`) — is stamped with `edp.instance=<id>`. `deploy.Reap` uses this to tear down only the calling instance's objects (`docker.ReapInstance` removes labeled containers + the shared network; compose stacks are torn down per-env from the store, since compose creates their containers without our label). This matters because multiple edp instances may share a host. When adding a new kind of created container, stamp the instance label too.

### Request auth tiers (in `server/`)

- JSON API (`requireAPI`) — `Authorization: Bearer <api_token>` only (global token generated on first boot). There is no session cookie anymore: with no embedded HTML there is nothing to cookie-auth. The admin password is bcrypt-hashed (`EDP_ADMIN_PASSWORD` re-applied authoritatively on every boot when set) and is only used by `POST /api/login`, which returns the API token (and sets the password on first run).
- Webhooks (`/hooks/{id}`) — per-env rotatable token as a query param.
- `/api/auth` and `/api/login` are the only unauthenticated routes (`login` is rate-limited by bcrypt's cost).

`EDP_TRUST_PROXY` makes the server honor `X-Forwarded-Proto/Host` (host-based routing) — only enable behind a trusted upstream proxy.

### On restart

`store.ResetInterrupted` marks any deploys/hook-runs left `running`/`queued` (from a crash) as failed and resets env/hook status to idle — otherwise an env would be stuck "running" forever (proxy shows the splash, scheduler skips it). Keep this invariant if you add new long-running state.

## Conventions

- Env-config fields are stored as free-text and parsed at deploy time: `splitLines` (newline-separated, e.g. env vars / volumes), `splitList` (comma-or-newline, e.g. ports/networks), and `splitArgs` (quote-aware argv tokenizer for command overrides) in `deploy/steps.go`.
- Tests are standard `_test.go` table tests next to the code (`naming`, `secret`, `proxy`, `scheduler`, `store`, `deploy`, `clearsite`, `server/handlers_io`).
- Exports always strip secrets/ids/status; imports re-encrypt any credentials present. `EDP_IMPORT` provisioning is idempotent by env name.
