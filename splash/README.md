# edp-splash

The standalone "deploying / starting…" interstitial and control UI for **edp**
environments, in the `splash/` directory of the edp monorepo. It's the spun-out
version of edp's built-in splash: edp ships a minimal one embedded in its binary;
use this when you want to own the page — restyle it, add controls, host it on your
own domain, and point `EDP_SPLASH_URL` (or a per-env Splash UI URL) at it.

A dependency-free static site (HTML/CSS/JS, no build step), packaged as a tiny
nginx container. (Distinct from [`ui/`](../ui), the admin dashboard — this page is
what an env's *end users* see while it's down.)

## How it works

When a proxied env isn't ready, edp redirects top-level navigations to your
splash URL with context as query params:

| param | meaning |
|-------|---------|
| `env` | environment name |
| `state` | `deploying` or `starting` |
| `eta` | estimated deploy duration (ms; `0` if unknown) |
| `elapsed` | ms the in-flight deploy has been running |
| `return` | URL to send the user back to once the env is ready |
| `ctl` | control base URL for this env |
| `token` | the env's webhook token, authorizing control calls |

The UI then drives two **per-env control endpoints** (CORS-enabled, gated by the
webhook token — no admin credentials in the browser):

```
GET  <ctl>/_edp/status?token=<token>     → {name,status,ready,deploying,eta_ms,elapsed_ms}
POST <ctl>/_edp/redeploy?token=<token>   → {deployment_id}
```

It polls `status`, shows a **Redeploy** button that calls `redeploy`, and
navigates back to `return` as soon as `ready` is true.

## Run it

```bash
docker compose up -d --build      # serves on http://localhost:8088
# or
docker build -t edp-splash . && docker run --rm -p 8088:80 edp-splash
```

For local development you can also just open `index.html` with any static
server (e.g. `python3 -m http.server`) and append the query params by hand.

## Point edp at it

Host edp-splash somewhere edp's users can reach, then tell edp to use it:

- **Globally:** set `EDP_SPLASH_URL=https://splash.example.com/` on edp.
- **Per-env:** set the **Splash UI URL** field on an environment (overrides the
  global one).

If you serve the UI from a different origin than the envs, allow it to call the
control endpoints: `EDP_CORS_ORIGINS=https://splash.example.com` on edp (the
default is `*`; calls are token-gated either way).

> Tip: edp can deploy this repo as one of its own environments — point an env at
> this project (git build) and front it on a hostname, then use that hostname as
> your `EDP_SPLASH_URL`.

## Customize

- **`config.js`** — branding label, poll interval, whether to show the Redeploy
  button. Mount your own over `/usr/share/nginx/html/config.js` to change it
  without rebuilding.
- **`styles.css`** — colors are CSS variables at the top; restyle the card freely.
- **`index.html` / `app.js`** — the markup IDs in `index.html` are what `app.js`
  drives; keep them in sync if you rework the layout.

## Files

| file | role |
|------|------|
| `index.html` | the splash shell |
| `styles.css` | look & feel (CSS variables up top) |
| `app.js` | reads query params, polls status, triggers redeploy, returns when ready |
| `config.js` | deploy-time branding/behavior knobs |
| `nginx.conf` | serves the shell for any path; no-cache on shell/config |
| `Dockerfile` / `docker-compose.yml` | packaging |
