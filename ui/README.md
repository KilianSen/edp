# edp-ui

The dashboard for [edp](..) (Easy Deploy Platform), in the `ui/` directory of the
edp monorepo. It is a static React + TypeScript (Vite) single-page app that talks
to edp's JSON API with a Bearer token — the spun-out replacement for the dashboard
that used to be embedded in the edp binary.

It can run two ways: **bundled** into the edp container (the `Dockerfile.ui` image
serves it same-origin at `/`), or **standalone** on its own host against a headless
edp (cross-origin, gated by `EDP_CORS_ORIGINS`). edp itself ships only the API,
webhooks, and the reverse proxy / "deploying…" splash; this app is the console.

## How it authenticates

One admin password guards edp. The login screen exchanges it for edp's global API
token (`POST /api/login`), which is stored in `localStorage` and sent as
`Authorization: Bearer <token>` on every request. On a brand-new edp the same
screen sets the password (first-run). "Sign out" just clears the local token.

Because the browser sends a Bearer header (not a cookie), there are no ambient
credentials — so edp serves the API with permissive CORS. Point edp at this app's
origin with `EDP_CORS_ORIGINS` (default `*`).

## Configure which edp it controls

The API origin is resolved at **runtime** from `public/config.js`:

```js
window.EDP_UI_CONFIG = { apiBase: "https://edp.example.com" };
```

Leave `apiBase` empty to use the same origin that served the page (handy when a
single reverse proxy routes `/api` + `/hooks` to edp and everything else here).
Because it's runtime config, one built image can target any edp — just mount your
own `config.js` over `/usr/share/nginx/html/config.js`.

## Develop

```bash
npm install
npm run dev          # http://localhost:5173
```

`npm run dev` proxies `/api` and `/hooks` to `http://localhost:8080` (override with
`VITE_EDP_API`), so the browser stays same-origin and you don't need CORS in dev.
Run an edp alongside it:

```bash
# from the repo root
go build -o edp ./cmd/edp
EDP_DATA_DIR=./data EDP_WORKSPACE_DIR=./ws EDP_ADMIN_PASSWORD=devpass123 ./edp
```

## Build & host

```bash
npm run build        # → dist/  (static files)
```

Serve `dist/` from any static host, or build the bundled nginx image:

```bash
docker build -t edp-ui .
docker run -p 8089:80 \
  -v "$PWD/public/config.js:/usr/share/nginx/html/config.js:ro" \
  edp-ui
```

When edp-ui is on a different origin than edp, set `EDP_CORS_ORIGINS` on edp to
this app's origin (or leave it `*`).

## What's where

- `src/lib/api.ts` — typed fetch client (Bearer auth, 401 → login).
- `src/lib/sse.ts` — live log / status streams over `fetch`+`ReadableStream`
  (EventSource can't send the auth header).
- `src/lib/auth.tsx` — token storage + login/first-run.
- `src/pages/*` — dashboard, env form, env detail, hooks, import, login.
- `src/styles.css` — Tailwind v4 with edp's design tokens ported in.
