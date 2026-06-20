// Runtime config for edp-manager-ui. Empty apiBase = same origin (the bundled
// UI is served by edp-manager itself). Override to point a separately-hosted
// build at a remote manager, e.g. window.EDPM_UI_CONFIG = { apiBase: "https://manager.example.com" }.
window.EDPM_UI_CONFIG = { apiBase: "" };
