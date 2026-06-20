# Artemis (feature/math/scaffold) on edp, via Portainer, behind NPM

Runs the single self-updating Artemis testing container — **no repo to host, no
Artemis image to host**. Portainer runs **edp**; edp builds the runtime from an
inline Dockerfile and runs + fronts it. Web on **8080**, JVM debug on **5005**.

```
NPM (host network) ──► 127.0.0.1:8800 (edp) ──► Artemis runtime container (8080)
  artemis-1.cloud…       router + dashboard       clones+builds Artemis, self-updates
```

## Just paste it

**`portainer-edp-stack.yml`** → Portainer › Stacks › Add stack (web editor). It carries
the entire Artemis environment in a compose `config`, so edp imports and auto-deploys it
on first boot. **Edit two things:**

1. `image:` → your edp GHCR ref + exact tag (e.g. `ghcr.io/kiliansen/edp:1.0.0`).
2. `EDP_ADMIN_PASSWORD` → set it (remove after first login).

Then in **NPM** add a Proxy Host: `artemis-1.cloud.kiliansen.de` → `http://127.0.0.1:8800`,
**Websockets Support on**, generous read/connect timeouts (first boot is long). edp matches
the host and proxies to the Artemis container, showing a **starting** splash until 8080 answers.

> Private edp image? Log the Portainer host into GHCR (`docker login ghcr.io`) or add the
> registry in Portainer. The first boot clones + Gradle-builds Artemis (many minutes) and the
> container self-updates the branch every `PULL_INTERVAL_SECONDS` after that. Host needs ~4–8 GB.

## What's in here

| file | role |
|------|------|
| `portainer-edp-stack.yml` | **the deliverable** — runs edp + the embedded Artemis env |
| `artemis-bundle.json` | the edp environment (source_type `dockerfile`, Dockerfile embedded). Also importable on its own via the dashboard or `POST /api/import` |
| `Dockerfile.selfcontained` | the inline Dockerfile edp builds — JDK + MySQL + the entrypoint/config baked in as base64, **no `COPY`** so it needs no build context |
| `entrypoint.sh`, `zz-artemis.cnf` | readable sources that `Dockerfile.selfcontained` embeds |

It uses edp's `dockerfile` source (build from an inline Dockerfile, no git repo). The env is
set for `feature/math/scaffold`; override `ARTEMIS_BRANCH` / `ARTEMIS_PROFILES` /
`PULL_INTERVAL_SECONDS` in the bundle's `run_env` to retarget. MySQL + the checkout persist in
the `artemis-src` / `artemis-mysql` volumes; `clear_site_data` wipes session on redeploy;
`health_type` is `none` because the first build outlasts any health timeout.

### Regenerating the bundle
If you change `entrypoint.sh` or `zz-artemis.cnf`, rebuild `Dockerfile.selfcontained` (embed
them as base64) and re-fold it into `artemis-bundle.json`, then re-paste the stack.

### Alternative: git build
`Dockerfile` + `artemis.edp.json` are the same thing for the git-source route — if you ever
*do* host these files in a repo, point an edp env at it instead of using the inline Dockerfile.

> The image initializes MySQL with an **empty root password** (dev/testing only) — keep it on
> a trusted network, which is edp's model anyway.
