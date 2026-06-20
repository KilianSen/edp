# edp-manager

Orchestrates multiple **edp** instances from one place: a single login, a
merged view of every environment across all your edp deployments, and a registry
you manage declaratively or at runtime.

It's a small Go service (pure-Go SQLite, no CGO) with the React dashboard in the
same repo (`ui/`), mirroring edp's monorepo layout. The manager *is* the
control-plane UI, so — unlike edp — there's no headless image: the single
`Dockerfile` always bundles the dashboard and serves it at `/` (via
`EDPM_UI_DIR`). That var stays a knob so local dev can run the API alone and
serve the UI from Vite.

## What it does

- **Instance registry** — register each edp instance (label, base URL, API
  token). Seed it declaratively from a YAML file *and* add/remove at runtime; the
  file is idempotent (instances whose label exists are left untouched).
- **Aggregate / fan-out** — `GET /api/overview|environments|status` query every
  instance in parallel and return one merged list, each row tagged with the
  instance it came from. One unreachable edp degrades gracefully (reported in an
  `errors` array) instead of failing the whole response.
- **Per-instance pass-through** — `…/api/instances/{id}/edp/<path>` proxies
  straight to one edp (token injected server-side), for env detail, redeploys,
  and SSE log streaming.
- **One login, tokens stay server-side** — you authenticate to the manager; each
  edp's API token is held encrypted at rest (AES-256-GCM) and never reaches the
  browser.

## Architecture

```
        browser ──Bearer──▶ edp-manager (this) ──┬─Bearer─▶ edp #1  (/api/*)
        (one login)         ├─ registry (SQLite, tokens encrypted)
                            ├─ fan-out + merge   ├─Bearer─▶ edp #2
                            ├─ pass-through proxy │
                            └─ bundled React UI   └─Bearer─▶ edp #3
```

Layout: `cmd/edp-manager` (entrypoint), `internal/{config,store,secret,edpclient,aggregate,bootstrap,server}`, `ui/` (the React dashboard, served from a dir via `EDPM_UI_DIR` — not embedded).

## Run it

```bash
docker compose up -d --build
# open http://localhost:9090  — first visit sets the manager admin password
EDPM_ADMIN_PASSWORD=your-strong-password docker compose up -d --build   # or set it explicitly
```

### Configuration (env vars)

| Var | Default | Purpose |
|-----|---------|---------|
| `EDPM_ADDR` | `:9090` | HTTP listen address |
| `EDPM_DATA_DIR` | `/data` | SQLite DB + encryption key |
| `EDPM_ADMIN_PASSWORD` | — | manager admin password; authoritative on boot (recovery path) |
| `EDPM_CONFIG` | — | path to a YAML instance-registry seed (see `config.example.yml`) |
| `EDPM_CORS_ORIGINS` | `*` | `Access-Control-Allow-Origin` for the API (Bearer-authed) |
| `EDPM_FANOUT_TIMEOUT_MS` | `8000` | per-instance timeout during a fan-out |
| `EDPM_UI_DIR` | — | dir of built dashboard files to serve at `/` (SPA fallback). The image sets it; leave empty for local API-only dev. |

## API

```
GET    /api/auth                              # {configured} — first-run?
POST   /api/login                             # {password} → {token, first_run}

GET    /api/instances                         # registry CRUD
POST   /api/instances
GET    /api/instances/{id}
PUT    /api/instances/{id}                    # omit api_token to keep the stored one
DELETE /api/instances/{id}
POST   /api/instances/{id}/test               # ping (reachable + token valid)

GET    /api/overview                          # fan-out + merge: {items[], errors[]}
GET    /api/environments                      # fan-out + merge
GET    /api/status                            # fan-out + merge
GET    /api/summary                           # fleet health: per-instance up/down + status counts + totals

ANY    /api/instances/{id}/edp/<edp-path>     # pass-through to one edp (SSE ok)
```

Merged rows carry `instance_id` and `instance_label`; failures are in `errors`.

## Build

```bash
docker build -t edp-manager .   # API + bundled dashboard (the only image)
```

## Develop

```bash
# backend (headless unless EDPM_UI_DIR is set)
go run ./cmd/edp-manager
go test ./...

# UI (separate terminal; Vite proxies /api to :9090)
cd ui && npm install && npm run dev

# or serve a production UI build from the same binary:
cd ui && npm run build && cd .. && EDPM_UI_DIR=./ui/dist go run ./cmd/edp-manager
```
