# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working in this repository.

## What this is

A **monorepo** for the edp ("Easy Deploy Platform") project. Four subprojects, each self-contained:

| Dir | Role | Stack | Deep guide |
|-----|------|-------|-----------|
| `edp/` | the deployer service — drives the host Docker daemon, REST API + webhooks + reverse proxy/splash | Go (module `edp`) | [`edp/CLAUDE.md`](edp/CLAUDE.md) |
| `ui/` | edp's dashboard SPA (consumes edp's API) | React + TS + Vite + Tailwind v4 | — |
| `splash/` | standalone "deploying…" interstitial/control UI | static HTML/CSS/JS + nginx | — |
| `manager/` | control plane over many edp instances (registry + fan-out + bundled UI) | Go (module `edp-manager`) + React | [`manager/README.md`](manager/README.md) |

**Two Go modules** (`edp/go.mod`, `manager/go.mod`) — there is no module at the repo root. `go ./...` from the root does nothing useful; run Go commands inside `edp/` or `manager/`. Each module keeps its own import path (`edp/…`, `edp-manager/…`), so a subproject's location in the tree is independent of its code.

When working inside a subproject, read its own `CLAUDE.md`/`README.md` first — this root file is just the map.

## Commands

```bash
# edp service
cd edp && go build ./... && go test ./...
cd manager && go build ./... && go test ./...    # control plane

# UIs (Vite dev servers proxy /api to the running backend)
cd ui && npm install && npm run dev
cd manager/ui && npm install && npm run dev
```

### Run the demos
```bash
cd edp && docker compose up -d --build                                   # edp + dashboard :8080
cd manager && docker compose -f docker-compose.stack.yml up -d --build   # edp + manager, self-wired
```

## Images & release

The single root workflow `.github/workflows/release.yml` is the **only** release pipeline (the subprojects have no CI of their own). On push to `master` it gates on both Go modules + both UIs, computes a version from **Conventional Commits**, and builds/pushes multi-arch images to GHCR:

- `edp` (headless) — `edp/Dockerfile`, **context = repo root** (the service is in `edp/`, the dashboard in `ui/`).
- `edp` bundled UI — `edp/Dockerfile.ui`, same root context; `-ui`-suffixed tags.
- `edp-manager` — `manager/Dockerfile`, context = `manager/` (self-contained; UI always bundled, no headless variant — the manager *is* the UI).
- `edp-splash` — `splash/Dockerfile`, context = `splash/`.

Because edp's images build from the repo root, the **root `.dockerignore`** keeps `edp/` + `ui/` in context and drops the sibling subprojects + node/build artifacts. The manager and splash build from their own dirs and use their own `.dockerignore`.

## How the pieces talk

- `ui/` → edp's `/api/*` with a Bearer token (CORS-enabled via `EDP_CORS_ORIGINS`); SSE consumed via `fetch`+stream, not `EventSource`.
- An env can redirect to `splash/` while down (`EDP_SPLASH_URL`/per-env `splash_url`); the splash drives redeploy/status via per-env control endpoints gated by the env's webhook token.
- `manager/` holds each edp's API token server-side and exposes **aggregate** fan-out reads (`/api/overview|environments|status|summary`, instance-tagged) plus a **per-instance pass-through** (`/api/instances/{id}/edp/<path>`, SSE-capable) for detail/actions/log-streaming. Its dashboard is instance-abstracted (envs are primary; instance is a tag).

## Conventions

- Both Go services follow the same patterns: pure-Go `modernc.org/sqlite` (CGO-free), an AES-256-GCM `secret` box for tokens at rest, `EDP_*`/`EDPM_*` env-var config, and the **`EDP_UI_DIR`/`EDPM_UI_DIR` knob** — when set, the binary serves a built UI dir at `/` via an SPA handler; unset = API-only.
- Commit messages drive releases — use Conventional Commits.
