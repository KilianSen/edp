// edp-splash configuration. Edit to taste, or replace at deploy time
// (e.g. mount over /usr/share/nginx/html/config.js in the container).
//
// Everything here is optional — the per-request values (env name, control base,
// token, return URL, ETA) come from the query string edp appends on redirect.
window.EDP_SPLASH_CONFIG = {
  brand: "edp",     // small label in the footer
  pollMs: 2500,     // how often to poll <ctl>/_edp/status
  showRedeploy: true, // show the Redeploy button (it calls <ctl>/_edp/redeploy)
};
