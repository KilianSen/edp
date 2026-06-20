# edp — Easy Deploy Platform

A monorepo for managing **test environments**: build/pull, deploy, redeploy, sweep,
and front them with a reverse proxy — plus a dashboard and a control plane to run
many edp instances from one place.

## Components

| Dir | What it is | Image(s) |
|-----|-----------|----------|
| [`edp/`](edp) | The deployer service (Go). Drives the host Docker daemon over the mounted socket; REST API + webhooks + reverse proxy/splash. | `edp`, `edp:…-ui` |
| [`ui/`](ui) | edp's dashboard — React + TS + Vite + Tailwind, talks to edp's API with a Bearer token. Bundled into `edp:…-ui` or hosted standalone. | (part of `edp:…-ui`) |
| [`splash/`](splash) | The standalone "deploying…" interstitial/control UI an env can redirect to while down. Dependency-free static site (nginx). | `edp-splash` |
| [`manager/`](manager) | The control plane (Go + bundled React UI): registers many edp instances and gives one merged, instance-abstracted dashboard over all of them. | `edp-manager` |

Two Go modules — [`edp/go.mod`](edp/go.mod) and [`manager/go.mod`](manager/go.mod); `ui/` and `splash/` are frontends.

## Quick start

**One edp + dashboard:**
```bash
cd edp && docker compose up -d --build      # http://localhost:8080
```

**The full control-plane demo** (an edp with the Docker socket + the manager,
self-wired, with a seeded auto-deploying env):
```bash
cd manager && docker compose -f docker-compose.stack.yml up -d --build
# manager → http://localhost:9090   edp → http://localhost:8080   (login: devpass123)
```

## Images (GHCR, via `.github/workflows/release.yml`)

On push to `master` the release builds + pushes multi-arch images, versioned from
Conventional Commits:

- `ghcr.io/<owner>/edp` — headless deployer (`:latest`, `:X.Y.Z`)
- `ghcr.io/<owner>/edp` — bundled dashboard (`:latest-ui`, `:X.Y.Z-ui`)
- `ghcr.io/<owner>/edp-manager` — control plane (UI always bundled)
- `ghcr.io/<owner>/edp-splash` — splash site

## More

Each component has its own README and (for the Go services) a `CLAUDE.md`:
[edp](edp/README.md) · [manager](manager/README.md) · [splash](splash/README.md) · [ui](ui).
