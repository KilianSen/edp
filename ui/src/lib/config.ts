// apiBase resolves the edp API origin, in priority order:
//   1. window.EDP_UI_CONFIG.apiBase  (runtime config.js — production)
//   2. import.meta.env.VITE_EDP_API  (build-time env)
//   3. "" → same origin (dev uses Vite's proxy; prod uses a fronting proxy)
export const apiBase: string = (
  window.EDP_UI_CONFIG?.apiBase ||
  import.meta.env.VITE_EDP_API ||
  ""
).replace(/\/+$/, "");

// url joins the configured base with an API/path, e.g. url("/api/status").
export function url(path: string): string {
  return apiBase + path;
}
