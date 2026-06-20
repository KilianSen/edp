// edp-ui runtime configuration. Edit to taste, or replace at deploy time
// (e.g. mount over /usr/share/nginx/html/config.js in the container).
//
// apiBase: the origin of the edp instance this dashboard controls. Leave empty
// to talk to the same origin that served this page (useful behind a single
// reverse proxy that routes /api + /hooks to edp). When set to a different
// origin, that edp must allow it via EDP_CORS_ORIGINS.
window.EDP_UI_CONFIG = {
  apiBase: "", // e.g. "https://edp.example.com"
};
