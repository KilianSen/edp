// edp dashboard behaviour: form section toggles, live deploy-log streaming, and
// the deploy progress / ETA animation. Loaded on every page; each piece is a
// no-op when its elements aren't present.
(function () {
  // --- env form: show fieldsets relevant to the chosen source / deploy type ---
  var PRESETS = {
    git:        { source: "git", deploy: "container" },
    image:      { source: "registry", deploy: "container" },
    compose:    { source: "git", deploy: "compose" },
    dockerfile: { source: "dockerfile", deploy: "container" }
  };

  function initForm() {
    var source = document.getElementById("source_type");
    var deploy = document.getElementById("deploy_type");
    if (!source || !deploy) return;
    function matches(attr, v) { return attr == null || attr.split(",").indexOf(v) !== -1; }
    function apply() {
      document.querySelectorAll("[data-when-source],[data-when-deploy]").forEach(function (el) {
        el.hidden = !(matches(el.getAttribute("data-when-source"), source.value) &&
                      matches(el.getAttribute("data-when-deploy"), deploy.value));
      });
    }
    if (!source._wired) { source.addEventListener("change", apply); deploy.addEventListener("change", apply); source._wired = 1; }

    // quick-start preset cards set the (hidden) source/deploy selects
    document.querySelectorAll("[data-preset]").forEach(function (card) {
      if (card._wired) return; card._wired = 1;
      card.addEventListener("click", function () {
        var p = PRESETS[card.dataset.preset]; if (!p) return;
        source.value = p.source; deploy.value = p.deploy;
        document.querySelectorAll("[data-preset]").forEach(function (c) { c.removeAttribute("data-active"); });
        card.setAttribute("data-active", "1");
        apply();
      });
    });
    apply();
  }

  // --- live log streaming (deployments and hook runs) ---
  function initLog() {
    var el = document.getElementById("log");
    if (!el || el._sse) return;
    var url = el.dataset.stream || ("/api/deployments/" + el.dataset.deploy + "/logs/stream");
    var status = document.getElementById("log-status");
    var es = new EventSource(url);
    el._sse = es;
    es.onmessage = function (e) { el.textContent += e.data + "\n"; el.scrollTop = el.scrollHeight; };
    es.addEventListener("done", function (e) {
      if (status) { status.textContent = e.data; status.className = "badge badge-" + e.data; }
      es.close();
      if (e.data === "success" || e.data === "failed") setTimeout(function () { location.reload(); }, 1400);
    });
    es.onerror = function () {};
  }

  // --- deploy progress / ETA: one global ticker updates every visible bar ---
  function tickProgress() {
    var now = Date.now();
    document.querySelectorAll("[data-progress]").forEach(function (el) {
      var start = +el.dataset.start, eta = +el.dataset.eta;
      var fill = el.querySelector("[data-fill]"), txt = el.querySelector("[data-eta-text]");
      if (!fill) return;
      var elapsed = now - start;
      if (eta > 0) {
        fill.style.width = Math.min(92, (elapsed / eta) * 100) + "%";
        if (txt) txt.textContent = "Deploying… ~" + Math.max(0, Math.round((eta - elapsed) / 1000)) + "s left";
      } else {
        fill.style.width = "35%";
        if (txt) txt.textContent = "Deploying…";
      }
    });
  }

  function initAll() { initForm(); initLog(); }
  document.addEventListener("DOMContentLoaded", initAll);
  document.body && document.body.addEventListener("htmx:afterSwap", initAll);
  setInterval(tickProgress, 500);
  tickProgress();
})();
