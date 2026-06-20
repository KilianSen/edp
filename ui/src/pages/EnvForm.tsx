import { useState } from "react";
import { Link, useNavigate, useParams, useSearchParams } from "react-router-dom";
import { api } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { blankEnv } from "../lib/types";
import type { Environment } from "../lib/types";
import { Accordion, Area, Check, Select, Text } from "../components/form";

const PRESETS: Record<string, { source: string; deploy: string }> = {
  git: { source: "git", deploy: "container" },
  image: { source: "registry", deploy: "container" },
  compose: { source: "git", deploy: "compose" },
  dockerfile: { source: "dockerfile", deploy: "container" },
};

export function EnvForm() {
  const { id } = useParams();
  const editing = !!id;
  const navigate = useNavigate();
  const [params] = useSearchParams();

  const { data, loading, error } = useAsync(async () => {
    if (editing) return api.getEnv(Number(id));
    const e = blankEnv();
    const p = PRESETS[params.get("preset") || "git"];
    if (p) {
      e.source_type = p.source;
      e.deploy_type = p.deploy;
    }
    return e;
  }, [id]);

  if (loading) return <p className="text-sm text-dim">Loading…</p>;
  if (error || !data) return <p className="text-sm text-fail">{error || "not found"}</p>;
  return <EnvFormBody initial={data} editing={editing} onDone={(eid) => navigate(`/env/${eid}`)} />;
}

function EnvFormBody({
  initial,
  editing,
  onDone,
}: {
  initial: Environment;
  editing: boolean;
  onDone: (id: number) => void;
}) {
  const [env, setEnv] = useState<Environment>(initial);
  const [activePreset, setActivePreset] = useState<string>("");
  const [saveErr, setSaveErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const set = <K extends keyof Environment>(k: K, v: Environment[K]) => setEnv((e) => ({ ...e, [k]: v }));
  const src = env.source_type;
  const dep = env.deploy_type;
  // when() mirrors edp.js: a section shows if its source/deploy constraints (CSV
  // lists, or undefined = always) both include the current selection.
  const when = (sources?: string[], deploys?: string[]) =>
    (!sources || sources.includes(src)) && (!deploys || deploys.includes(dep));

  function applyPreset(key: string) {
    const p = PRESETS[key];
    if (!p) return;
    setActivePreset(key);
    setEnv((e) => ({ ...e, source_type: p.source, deploy_type: p.deploy }));
  }

  // clear_site_data is a CSV of directives toggled by four checkboxes.
  const csd = (env.clear_site_data || "").split(",").map((s) => s.trim()).filter(Boolean);
  const toggleCsd = (key: string, on: boolean) => {
    const next = new Set(csd);
    if (on) next.add(key);
    else next.delete(key);
    set("clear_site_data", Array.from(next).join(","));
  };

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setSaveErr(null);
    try {
      const saved = editing ? await api.updateEnv(env.id, env) : await api.createEnv(env);
      onDone(saved.id);
    } catch (err) {
      setSaveErr(err instanceof Error ? err.message : "save failed");
      setBusy(false);
    }
  }

  const backTo = editing ? `/env/${env.id}` : "/";

  return (
    <>
      <div className="mb-6">
        <Link to={backTo} className="font-mono text-xs text-faint hover:text-dim">
          ← back
        </Link>
        <h1 className="mt-2 font-display text-2xl font-semibold tracking-tight">
          {editing ? `Edit “${env.name}”` : "New environment"}
        </h1>
        {!editing && (
          <p className="mt-1 text-sm text-dim">
            Pick a starting point, name it, point it at your code — that’s the whole setup. Everything else has sensible
            defaults.
          </p>
        )}
      </div>

      {saveErr && (
        <p className="mb-4 rounded-lg border border-fail/30 bg-fail-soft px-3 py-2 text-sm text-fail">{saveErr}</p>
      )}

      <form onSubmit={submit} className="space-y-5">
        {!editing && (
          <div>
            <p className="eyebrow mb-2">Start from</p>
            <div className="flex flex-col gap-3 sm:flex-row">
              {[
                ["git", "Git repo", "Clone & build from a Dockerfile"],
                ["image", "Container image", "Pull a prebuilt image from a registry"],
                ["compose", "Compose stack", "Bring up a docker-compose.yml from a repo"],
                ["dockerfile", "Inline Dockerfile", "Build from a pasted Dockerfile, no repo"],
              ].map(([key, title, sub]) => (
                <button
                  type="button"
                  key={key}
                  className="preset"
                  data-active={activePreset === key ? "1" : undefined}
                  onClick={() => applyPreset(key)}
                >
                  <h3>{title}</h3>
                  <p>{sub}</p>
                </button>
              ))}
            </div>
          </div>
        )}

        <section className="card p-5">
          <p className="eyebrow">Essentials</p>
          <div className="mt-4 space-y-4">
            <Text label="Name" value={env.name} onChange={(v) => set("name", v)} required autoFocus placeholder="checkout-staging" />
            {when(["git"]) && (
              <>
                <Text
                  label="Repository URL"
                  value={env.repo_url}
                  onChange={(v) => set("repo_url", v)}
                  placeholder="https://github.com/org/repo.git"
                />
                <Text
                  label="Branch / tag / commit"
                  hint="(blank = default branch)"
                  value={env.git_ref}
                  onChange={(v) => set("git_ref", v)}
                  placeholder="main"
                />
              </>
            )}
            {when(["registry"]) && (
              <Text
                label="Image"
                value={env.registry_image}
                onChange={(v) => set("registry_image", v)}
                placeholder="ghcr.io/org/app:tag"
              />
            )}
          </div>
        </section>

        {when(["git", "dockerfile"]) && (
          <Accordion title="Build options">
            {when(["git"]) && (
              <Text label="Access token" hint="(only for private repos)" type="password" value={env.git_token || ""} onChange={(v) => set("git_token", v)} />
            )}
            {when(["git"], ["container"]) && (
              <div className="grid gap-4 sm:grid-cols-2">
                <Text label="Dockerfile path" value={env.dockerfile_path} onChange={(v) => set("dockerfile_path", v)} placeholder="Dockerfile" />
                <Text label="Build context" value={env.build_context} onChange={(v) => set("build_context", v)} placeholder="." />
              </div>
            )}
            {when(undefined, ["container"]) && (
              <Area
                label="Dockerfile"
                hint="(git source: overrides the repo’s; dockerfile source: must be self-contained, no COPY)"
                rows={6}
                value={env.dockerfile_content}
                onChange={(v) => set("dockerfile_content", v)}
                placeholder={'FROM node:20-alpine\nRUN npm ci\nCMD ["npm","run","start:test"]'}
              />
            )}
            {when(undefined, ["compose"]) && (
              <Text label="Compose file" hint="(relative to repo)" value={env.compose_path} onChange={(v) => set("compose_path", v)} placeholder="docker-compose.yml" />
            )}
            {when(undefined, ["compose"]) && (
              <Area
                label="Compose override"
                hint="(merged on top of the file above)"
                rows={6}
                value={env.compose_override}
                onChange={(v) => set("compose_override", v)}
                placeholder={"services:\n  app:\n    environment:\n      - APP_ENV=test"}
              />
            )}
          </Accordion>
        )}

        {when(["registry"]) && (
          <Accordion title="Registry credentials">
            <div className="grid gap-4 sm:grid-cols-2">
              <Text label="Username" value={env.registry_username} onChange={(v) => set("registry_username", v)} />
              <Text label="Password / token" type="password" value={env.registry_password || ""} onChange={(v) => set("registry_password", v)} />
            </div>
          </Accordion>
        )}

        <Accordion title="Environment variables">
          <Area
            label="Variables"
            hint="(KEY=VALUE per line)"
            rows={4}
            value={env.run_env}
            onChange={(v) => set("run_env", v)}
            placeholder={"DATABASE_URL=postgres://db:5432/app\nLOG_LEVEL=debug"}
          />
          <p className="text-xs text-dim">
            Set on the container (<code>-e</code>) for single-container envs. For compose stacks they’re passed to{" "}
            <code>docker compose</code>. Also available to setup/cleanup &amp; timed-hook scripts.
          </p>
        </Accordion>

        {when(undefined, ["container"]) && (
          <Accordion title="Runtime — ports, volumes">
            <div className="grid gap-4 sm:grid-cols-2">
              <Text label="Ports" hint="(host:container, comma-sep)" value={env.run_ports} onChange={(v) => set("run_ports", v)} placeholder="8080:80" />
              <Text label="Networks" hint="(comma-sep)" value={env.run_networks} onChange={(v) => set("run_networks", v)} />
            </div>
            <Area label="Volumes" hint="(name:/path per line)" rows={2} value={env.run_volumes} onChange={(v) => set("run_volumes", v)} />
            <div className="grid gap-4 sm:grid-cols-2">
              <Text label="Image tag" hint="(build target)" value={env.image_name} onChange={(v) => set("image_name", v)} placeholder="auto" />
              <Select label="Restart policy" value={env.restart_policy} onChange={(v) => set("restart_policy", v)} options={["unless-stopped", "always", "on-failure", "no"]} />
            </div>
            <div className="grid gap-4 sm:grid-cols-2">
              <Text label="Entrypoint override" hint="(blank = image default)" mono value={env.entrypoint} onChange={(v) => set("entrypoint", v)} placeholder="/bin/sh" />
              <Text label="Command override" hint="(args; quotes respected)" mono value={env.command} onChange={(v) => set("command", v)} placeholder={'-c "npm run start:test"'} />
            </div>
            <Area
              label="Entrypoint script"
              hint="(runs as the container’s entrypoint; overrides command)"
              rows={6}
              value={env.entrypoint_script}
              onChange={(v) => set("entrypoint_script", v)}
              placeholder={"#!/bin/sh\nuntil nc -z db 5432; do sleep 1; done\nexec npm run start:test"}
            />
          </Accordion>
        )}

        <Accordion title="Lifecycle — schedule, hooks, volume sweep">
          <div className="grid gap-4 sm:grid-cols-2">
            <Text label="Redeploy schedule" hint="(cron or duration; blank = off)" value={env.redeploy_schedule} onChange={(v) => set("redeploy_schedule", v)} placeholder="30m · 0 */6 * * *" />
            <Select label="Volume sweep" value={env.volume_sweep} onChange={(v) => set("volume_sweep", v)} options={["none", "named", "all"]} />
          </div>
          <Area label="Setup script — Python, before deploy" rows={3} value={env.setup_script} onChange={(v) => set("setup_script", v)} placeholder={"import os\nprint('preparing', os.environ['EDP_ENV_NAME'])"} />
          <Area label="Cleanup script — Python, after deploy" rows={2} value={env.cleanup_script} onChange={(v) => set("cleanup_script", v)} />
          <Check label="Auto-deploy when created / imported / loaded on startup" checked={env.auto_deploy} onChange={(v) => set("auto_deploy", v)} />
          <Check label="Prune dangling images after deploy" checked={env.prune_images} onChange={(v) => set("prune_images", v)} />
        </Accordion>

        <Accordion title="Health check">
          <p className="text-xs text-dim">edp waits for this to pass after deploy — it defines “ready” and times the start for the ETA.</p>
          <div className="grid gap-4 sm:grid-cols-[180px_1fr]">
            <Select label="Type" value={env.health_type} onChange={(v) => set("health_type", v)} options={["none", "http", "exec"]} />
            <Text label="Target" hint="(URL for http · command for exec)" mono value={env.health_target} onChange={(v) => set("health_target", v)} placeholder="http://edp-<name>:8080/health" />
          </div>
        </Accordion>

        <Accordion title="Access & proxy">
          <p className="text-xs text-dim">
            Reach this env through edp at <code>/e/{env.name || "<name>"}/</code>, by hostname, or a raw TCP port. Leave
            the container port blank to keep it off.
          </p>
          <div className="grid gap-4 sm:grid-cols-2">
            <Text label="Proxy host" hint="(HTTP, optional)" value={env.proxy_host} onChange={(v) => set("proxy_host", v)} placeholder="app.test.local" />
            <Text label="Container port" hint="(target)" mono value={env.proxy_port} onChange={(v) => set("proxy_port", v)} placeholder="80" />
          </div>
          <Text label="TCP port-forward" hint="(host listen port, optional)" mono value={env.listen_port} onChange={(v) => set("listen_port", v)} placeholder="9001" />
          <Text label="Splash UI URL" hint="(overrides EDP_SPLASH_URL while this env is down)" value={env.splash_url} onChange={(v) => set("splash_url", v)} />

          <div className="border-t border-line-soft pt-4">
            <p className="lbl">Clear browser data on redeploy <span className="text-faint">(HTTP proxy only)</span></p>
            <p className="mt-0.5 text-xs text-dim">After a redeploy, the next visit through edp tells the browser to wipe the selected data for a clean test.</p>
            <div className="mt-2.5 flex flex-wrap gap-x-5 gap-y-2 text-sm text-dim">
              {[
                ["cookies", "Cookies"],
                ["storage", "localStorage / IndexedDB"],
                ["cache", "Cache"],
                ["executionContexts", "Force reload"],
              ].map(([key, label]) => (
                <label key={key} className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    checked={csd.includes(key)}
                    onChange={(e) => toggleCsd(key, e.target.checked)}
                    className="h-4 w-4 rounded border-line bg-ink text-go focus:ring-go/40"
                  />{" "}
                  {label}
                </label>
              ))}
            </div>
          </div>
        </Accordion>

        <div className="flex items-center gap-3 pb-4">
          <button type="submit" disabled={busy} className="btn btn-primary">
            {editing ? "Save changes" : "Create environment"}
          </button>
          <Link className="btn btn-ghost" to={backTo}>
            Cancel
          </Link>
        </div>
      </form>
    </>
  );
}
