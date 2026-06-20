// edp-splash — drives the interstitial/control UI for an edp environment.
//
// edp redirects a browser here (when the env is down) with query params:
//   env, state, eta, elapsed, return, ctl, token   (see README).
// We poll <ctl>/_edp/status, optionally trigger <ctl>/_edp/redeploy, and send
// the user back to `return` once the environment reports ready.
(function () {
  "use strict";

  var CFG = window.EDP_SPLASH_CONFIG || {};
  var POLL_MS = CFG.pollMs || 2500;

  var q = new URLSearchParams(location.search);
  var ENV = q.get("env") || "environment";
  var CTL = (q.get("ctl") || "").replace(/\/+$/, ""); // strip trailing slash
  var TOKEN = q.get("token") || "";
  var RETURN = q.get("return") || "/";
  var eta = toInt(q.get("eta"));
  var elapsed = toInt(q.get("elapsed"));
  var deploying = (q.get("state") || "starting") === "deploying";
  var start = Date.now() - (elapsed || 0);

  var el = function (id) { return document.getElementById(id); };
  function toInt(v) { var n = parseInt(v || "0", 10); return isNaN(n) ? 0 : n; }
  function ctlURL(path) {
    return CTL + "/_edp/" + path + "?token=" + encodeURIComponent(TOKEN);
  }

  // ---- initial paint ----
  el("env").textContent = ENV;
  if (el("brand") && CFG.brand) el("brand").textContent = CFG.brand;
  document.title = ENV + " — " + (deploying ? "deploying" : "starting") + "…";

  var btn = el("redeploy");
  if (!CFG.showRedeploy || !CTL) btn.style.display = "none";

  function render() {
    el("title").innerHTML =
      (deploying ? "Deploying " : "Starting ") + '<span class="name">' + escapeHTML(ENV) + "</span>…";
    el("sub").textContent = deploying
      ? "A new version is rolling out."
      : "This environment isn’t running yet.";

    var e = Date.now() - start;
    if (eta > 0 && deploying) {
      el("fill").style.width = Math.min(95, Math.round((e / eta) * 100)) + "%";
      var left = Math.max(0, Math.round((eta - e) / 1000));
      el("eta").textContent =
        "~" + left + "s remaining (last deploy took ~" + Math.round(eta / 1000) + "s)";
    } else {
      el("fill").style.width = "40%";
      el("eta").textContent = "";
    }
  }

  // ---- status polling ----
  async function poll() {
    if (!CTL) return; // no control base → just animate (edp falls back to its own splash anyway)
    try {
      var res = await fetch(ctlURL("status"), { cache: "no-store" });
      if (res.ok) {
        var s = await res.json();
        deploying = !!s.deploying;
        if (typeof s.eta_ms === "number" && s.eta_ms > 0) eta = s.eta_ms;
        if (s.ready) {
          el("msg").textContent = "Ready — returning…";
          location.replace(RETURN);
          return;
        }
      }
    } catch (_) {
      // edp or the env is briefly unreachable mid-deploy; keep polling.
    }
    render();
  }

  // ---- redeploy ----
  btn.addEventListener("click", async function () {
    if (!CTL) return;
    btn.disabled = true;
    el("msg").textContent = "Triggering redeploy…";
    try {
      var res = await fetch(ctlURL("redeploy"), { method: "POST" });
      if (res.ok) {
        deploying = true;
        start = Date.now();
        el("msg").textContent = "Redeploy started.";
      } else {
        el("msg").textContent = "Redeploy failed (" + res.status + ").";
      }
    } catch (e) {
      el("msg").textContent = "Redeploy failed: " + e;
    }
    setTimeout(function () { btn.disabled = false; }, 3000);
    render();
  });

  function escapeHTML(s) {
    return String(s).replace(/[&<>"']/g, function (c) {
      return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c];
    });
  }

  // ---- go ----
  render();
  setInterval(render, 500);
  poll();
  setInterval(poll, POLL_MS);
})();
