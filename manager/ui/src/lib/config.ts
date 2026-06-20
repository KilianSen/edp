// apiBase resolves the manager API origin:
//   1. window.EDPM_UI_CONFIG.apiBase  (runtime config.js — production)
//   2. import.meta.env.VITE_EDPM_API  (build-time env)
//   3. "" → same origin (the bundled UI is served by the manager)
declare global {
  interface Window {
    EDPM_UI_CONFIG?: { apiBase?: string };
  }
}

export const apiBase: string = (
  window.EDPM_UI_CONFIG?.apiBase ||
  import.meta.env.VITE_EDPM_API ||
  ""
).replace(/\/+$/, "");

export function url(path: string): string {
  return apiBase + path;
}
