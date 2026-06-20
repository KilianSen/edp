# edp — Easy Deploy Platform

A single container that manages the lifecycle of **test environments** — build or
pull, deploy, redeploy, sweep volumes — with an inbound webhook and a REST API.
The web dashboard is a separate single-page app, [`edp-ui`](../ui) (in this monorepo),
that drives this API. It is **not** a Portainer replacement; it's a focused, low-overhead
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
- **Control surfaces** — REST API + webhook, with **live** container status (pushed from Docker events over SSE) and streaming deploy logs. The dashboard ([`edp-ui`](../ui)) is a separate SPA that consumes them.
- **Auth** — single admin password (bcrypt); a global API token (Bearer) guards the API, plus per-env webhook tokens. `POST /api/login` exchanges the password for the token.

## Container images

edp ships in two flavours of the **same binary** — the only difference is whether
the dashboard is bundled in (`EDP_UI_DIR`):

| Image | Contains | Serves at `/` | Use when |
|-------|----------|---------------|----------|
| **headless** (`Dockerfile`) | API + webhooks + reverse proxy/splash | `404` | you host `edp-ui` separately, or only use the API/webhook |
| **bundled UI** (`Dockerfile.ui`) | the above **+** the `edp-ui` SPA | the dashboard | you want one container with everything, same-origin (no CORS) |

Build them **from the monorepo root** (the service is in `edp/`, the dashboard in [`ui/`](../ui)):

```bash
docker build -f edp/Dockerfile -t edp .          # headless
docker build -f edp/Dockerfile.ui -t edp:ui .    # bundled UI (builds ui/ too)
```

The bundled image serves the dashboard and API on one port/origin, so the SPA's
`config.js` `apiBase` stays empty and no `EDP_CORS_ORIGINS` is needed. (You can
also run the dashboard standalone against a headless edp — see [`ui/`](../ui).)

## Run it

```bash
docker compose up -d --build
# bundled-UI image: open http://localhost:8080 — first visit sets the admin password
# headless image:  POST the admin password to /api/login to get the API token
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
| `EDP_REAP_ON_EXIT` | `false` | on shutdown, tear down all of this instance's managed containers, compose stacks, forwarders, and the shared network (see [Teardown](#teardown)). Leave **off** unless envs are reprovisioned on boot. |
| `EDP_SPLASH_URL` | — | external "deploying/starting" UI to redirect to while a proxied env is down (per-env **Splash UI URL** overrides it). Empty = built-in splash. See [External splash UI](#external-splash-ui). |
| `EDP_CORS_ORIGINS` | `*` | `Access-Control-Allow-Origin` for the JSON API (`/api/*`) and the per-env proxy control endpoints (`/_edp/*`), so a separately-hosted dashboard (`edp-ui`) or splash UI can call them cross-origin. Safe to leave `*`: the API is Bearer-token authed, not cookie-authed. |
| `EDP_UI_DIR` | — | directory of a built `edp-ui` to serve at `/` (with SPA fallback). Set in the **bundled-UI image**; leave empty for headless (API only). Same origin as the API, so no CORS needed. |

## Defining an environment

Create one in the dashboard (**New environment**) or via the API. Key fields:

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

Authenticate with `Authorization: Bearer <api_token>`. The token is generated on
first boot; obtain it by posting the admin password to `POST /api/login`, which is
exactly what `edp-ui`'s login screen does. (On a fresh edp the same call also sets
the password — first-run.)

```
GET    /api/auth                               # {configured} — is a password set yet?
POST   /api/login                              # {password} → {token, first_run}
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
GET    /api/overview                           # decorated env list (state, last deploy, ETA)
GET    /api/status                             # live container state per env
GET    /api/events/stream                      # SSE: container-state changes
```

SSE endpoints can't carry an `Authorization` header from `EventSource`; consume
them with `fetch` + a streaming reader (as `edp-ui` does) so the Bearer token
applies.

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

### External splash UI

By default, while a proxied env is deploying or not yet up, edp serves its
built-in "deploying ~Xs" splash. You can instead point edp at **your own UI** and
let it drive redeploys — useful for a branded interstitial or a richer control
page.

Set it globally with `EDP_SPLASH_URL`, or per-env with **Splash UI URL** (the
per-env value wins). When set, edp redirects top-level navigations of a down env
to that URL with context query params:

```
https://ui.example.com/splash?env=<name>&state=deploying&eta=<ms>&elapsed=<ms>
    &return=<original-url>&ctl=<control-base>&token=<webhook-token>
```

The UI drives two **per-env control endpoints** on the proxied origin
(CORS-enabled, gated by the env's webhook token — no admin credentials needed):

```
GET  <ctl>/_edp/status?token=<token>     # {name,status,ready,deploying,eta_ms,elapsed_ms}
POST <ctl>/_edp/redeploy?token=<token>   # trigger a redeploy → {deployment_id}
```

It polls `status`, offers a redeploy action, and sends the user back to `return`
once `ready` is true. Asset/XHR requests still get a plain `503` (only top-level
navigations redirect), so SPAs don't bounce their API calls to the UI.

A ready-to-host reference implementation lives in [`splash/`](splash) — a
dependency-free static UI (no build step) packaged as a small nginx container.
Deploy it as-is, restyle it, or even run it as one of edp's own environments.
Lock down who may call the control endpoints with `EDP_CORS_ORIGINS` if desired.

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

## Teardown

The containers, compose stacks, and forwarder sidecars edp creates are
independent top-level Docker objects (each with its own restart policy), **not**
part of edp's own compose stack. So `docker compose down` on edp removes only edp
— the test environments keep running. That's deliberate: updating or restarting
edp (`docker compose up -d --build`) must not nuke your envs.

To remove everything an edp instance created, reap it:

```bash
docker exec edp edp reap            # tear down while edp is still up
# or, after edp itself is gone (reads instance state from the /data volume):
docker compose run --rm edp reap
```

`reap` removes this instance's single-container envs, compose stacks (`down -v`),
forwarder sidecars, and the shared `edp` network. It is **scoped to the
instance** via a stable `edp.instance` label (persisted in `/data`, so it
survives edp container recreation) — running several edp instances on one host,
reaping one leaves the others untouched.

To reap automatically on every shutdown, set **`EDP_REAP_ON_EXIT=1`**. Only do
this when your envs are reprovisioned on boot (e.g. `EDP_IMPORT` + auto-deploy),
since it means a routine restart/update of edp tears all envs down and rebuilds
them.

## Architecture

```
Go binary (CGO-free) ── API + webhooks + reverse proxy (no embedded UI)
  ├─ HTTP server: /api/* (Bearer), /hooks/* (per-env token), SSE (logs + status), CORS
  ├─ Deploy engine: serialized per env; shells out to docker / docker compose / git
  ├─ Events watcher: streams `docker events` → status hub → SSE (live dots)
  ├─ Scheduler: per-env redeploy timer (duration or cron)
  ├─ Secret box: AES-256-GCM for credentials at rest
  └─ Store: SQLite (modernc.org/sqlite, no CGO)

ui/ (edp-ui) ──────────── React + TypeScript SPA → consumes /api/* with a Bearer token
```

Source layout: `cmd/edp` (entrypoint), `internal/{config,store,secret,docker,source,sh,deploy,scheduler,events,statushub,logbus,server}`, the admin dashboard in [`ui/`](../ui), and the standalone "deploying…" splash in [`splash/`](splash).

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

### Dashboard

The dashboard is a separate single-page app in [`ui/`](../ui) (React + TypeScript + Vite, styled
with Tailwind CSS v4). It talks to this binary's JSON API with a Bearer token. The **bundled-UI
image** serves it same-origin (no CORS); for standalone dev/hosting, point it at a headless edp
with `EDP_CORS_ORIGINS` allowing its origin. See [`ui/README.md`](ui/README.md) for
dev/build/host instructions.
