import { useState } from "react";
import { Link, useNavigate, useParams, useSearchParams } from "react-router-dom";
import { api, downloadExport } from "../lib/api";
import { apiBase } from "../lib/config";
import { useAsync } from "../lib/useAsync";
import { badgeClass, durMs, fmtDate } from "../lib/format";
import { DeployProgress } from "../components/DeployProgress";
import { LogStream } from "../components/LogStream";
import type { Deployment, Environment, HookRun, OverviewItem, TimedHook } from "../lib/types";

interface HookView extends TimedHook {
  last: HookRun | null;
}
interface Detail {
  ov: OverviewItem;
  deployments: Deployment[];
  hooks: HookView[];
}

const webhookOrigin = apiBase || window.location.origin;

export function EnvDetail() {
  const { id } = useParams();
  const envID = Number(id);
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const focusParam = params.get("deploy");

  const { data, error, reload } = useAsync<Detail>(async () => {
    const [overview, deployments, hooks] = await Promise.all([
      api.overview(),
      api.listDeployments(envID),
      api.listHooks(envID),
    ]);
    const ov = overview.find((o) => o.env.id === envID);
    if (!ov) throw new Error("not found");
    const hookViews = await Promise.all(
      hooks.map(async (h) => ({ ...h, last: (await api.listHookRuns(h.id))[0] ?? null })),
    );
    return { ov, deployments, hooks: hookViews };
  }, [envID]);

  if (error) return <p className="text-sm text-fail">{error}</p>;
  if (!data) return <p className="text-sm text-dim">Loading…</p>;

  const { ov, deployments, hooks } = data;
  const env = ov.env;
  const deploying = !!ov.last && (ov.last.status === "running" || ov.last.status === "queued");
  const focus = focusParam ? deployments.find((d) => d.id === Number(focusParam)) : deployments[0];
  const webhookURL = `${webhookOrigin}/hooks/${env.id}?token=${env.webhook_token}`;

  return (
    <>
      <Header env={env} ov={ov} deploying={deploying} onChanged={reload} onDeleted={() => navigate("/")} />

      {deploying && ov.last?.started_at && (
        <div className="card mb-5 p-4">
          <DeployProgress startMs={new Date(ov.last.started_at).getTime()} etaMs={ov.estimate_ms} />
        </div>
      )}

      <Access env={env} webhookURL={webhookURL} onChanged={reload} />

      <div className="grid gap-5 lg:grid-cols-2">
        <section className="card p-5">
          <p className="eyebrow">Deployments</p>
          <ul className="mt-3 divide-y divide-line-soft">
            {deployments.length === 0 && <li className="py-3 text-sm text-dim">No deployments yet.</li>}
            {deployments.map((d) => (
              <li key={d.id} className="flex items-center justify-between gap-3 py-2.5">
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <span className={badgeClass(d.status)}>{d.status}</span>
                    <span className="font-mono text-xs text-faint">{d.trigger}</span>
                    {d.duration_ms > 0 && <span className="font-mono text-xs text-faint">{durMs(d.duration_ms)}</span>}
                  </div>
                  {d.reason && <p className="mt-1 truncate text-xs text-dim">{d.reason}</p>}
                </div>
                <div className="shrink-0 text-right">
                  <Link className="font-mono text-xs text-go hover:underline" to={`/env/${env.id}?deploy=${d.id}`}>
                    log
                  </Link>
                  <p className="mt-0.5 font-mono text-[11px] text-faint">{fmtDate(d.started_at)}</p>
                </div>
              </li>
            ))}
          </ul>
        </section>

        <section className="card flex flex-col p-5">
          {focus ? (
            <LogStream
              key={focus.id}
              streamPath={`/api/deployments/${focus.id}/logs/stream`}
              status={focus.status}
              onDone={reload}
            />
          ) : (
            <>
              <p className="eyebrow">Log</p>
              <p className="mt-3 text-sm text-dim">Trigger a deploy to stream its output here.</p>
            </>
          )}
        </section>
      </div>

      <Hooks envID={env.id} hooks={hooks} onChanged={reload} />
    </>
  );
}

function Header({
  env,
  ov,
  deploying,
  onChanged,
  onDeleted,
}: {
  env: Environment;
  ov: OverviewItem;
  deploying: boolean;
  onChanged: () => void;
  onDeleted: () => void;
}) {
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);

  async function redeploy(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      await api.deploy(env.id, reason || "manual");
      setReason("");
      onChanged();
    } finally {
      setBusy(false);
    }
  }
  async function exportEnv() {
    const { href, revoke } = await downloadExport(`/api/environments/${env.id}/export`);
    download(href, `edp-env-${env.name}.json`);
    setTimeout(revoke, 1000);
  }
  async function remove() {
    if (!confirm(`Delete environment “${env.name}”? This removes its config (not running containers).`)) return;
    await api.deleteEnv(env.id);
    onDeleted();
  }

  return (
    <div className="mb-6">
      <Link to="/" className="font-mono text-xs text-faint hover:text-dim">
        ← environments
      </Link>
      <div
        className="mt-2 flex flex-wrap items-start justify-between gap-4"
        data-state={ov.container_state}
        {...(deploying ? { "data-deploying": "1" } : {})}
      >
        <div>
          <h1 className="flex items-center gap-2.5 font-display text-2xl font-semibold tracking-tight">
            <span className="dot" />
            {env.name}
          </h1>
          <div className="mt-1.5 flex flex-wrap items-center gap-1.5">
            <span className="pill bg-raised text-dim">{env.source_type}</span>
            <span className="pill bg-raised text-dim">{env.deploy_type}</span>
            <span className="pill bg-raised text-dim capitalize">
              {ov.container_state === "none" ? "not running" : `${ov.container_state} ${ov.container_info}`}
            </span>
            {ov.estimate_ms > 0 && <span className="pill bg-raised text-dim">~{durMs(ov.estimate_ms)} start</span>}
          </div>
        </div>
        <div className="flex items-center gap-2">
          <form onSubmit={redeploy} className="flex items-center gap-2">
            <input
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              className="field !mt-0 w-44"
              placeholder="reason (optional)"
            />
            <button disabled={busy} className="btn btn-primary">
              Redeploy
            </button>
          </form>
          <Link className="btn" to={`/env/${env.id}/edit`}>
            Edit
          </Link>
          <button className="btn" onClick={exportEnv} title="Download this environment as JSON">
            Export
          </button>
          <button className="btn btn-ghost" onClick={remove}>
            Delete
          </button>
        </div>
      </div>
    </div>
  );
}

function Access({ env, webhookURL, onChanged }: { env: Environment; webhookURL: string; onChanged: () => void }) {
  const [busy, setBusy] = useState(false);
  async function rotate() {
    setBusy(true);
    try {
      await api.rotateToken(env.id);
      onChanged();
    } finally {
      setBusy(false);
    }
  }
  return (
    <section className="card mb-5 p-5">
      <p className="eyebrow">Access</p>
      <div className="mt-3 space-y-3 text-sm">
        {env.proxy_port && (
          <div>
            <p className="stat-label mb-1">HTTP reverse proxy</p>
            <code className="block break-all rounded-md border border-line bg-ink px-3 py-2 font-mono text-xs">
              {webhookOrigin}/e/{env.name}/
            </code>
            {env.proxy_host && (
              <code className="mt-1.5 block break-all rounded-md border border-line bg-ink px-3 py-2 font-mono text-xs">
                http://{env.proxy_host}/ <span className="text-faint">(point this host at edp)</span>
              </code>
            )}
          </div>
        )}
        {env.listen_port && (
          <div>
            <p className="stat-label mb-1">TCP port-forward</p>
            <code className="block rounded-md border border-line bg-ink px-3 py-2 font-mono text-xs">
              :{env.listen_port} → container :{env.proxy_port}
            </code>
          </div>
        )}
        <div>
          <p className="stat-label mb-1">
            Webhook <span className="normal-case tracking-normal text-faint">— POST to redeploy from CI/git</span>
          </p>
          <code className="block break-all rounded-md border border-line bg-ink px-3 py-2 font-mono text-xs">
            {webhookURL}
          </code>
          <button onClick={rotate} disabled={busy} className="btn btn-sm btn-ghost mt-2">
            Rotate token
          </button>
        </div>
      </div>
    </section>
  );
}

function Hooks({ envID, hooks, onChanged }: { envID: number; hooks: HookView[]; onChanged: () => void }) {
  async function run(id: number) {
    await api.runHook(id);
    setTimeout(onChanged, 500);
  }
  return (
    <section className="card mt-5 p-5">
      <div className="flex items-center justify-between">
        <div>
          <p className="eyebrow">Timed hooks</p>
          <p className="mt-1 text-xs text-dim">
            Scheduled Python scripts that run against this env <span className="text-fg">without</span> redeploying it.
          </p>
        </div>
        <Link className="btn btn-sm" to={`/env/${envID}/hooks/new`}>
          Add hook
        </Link>
      </div>
      <ul className="mt-3 divide-y divide-line-soft">
        {hooks.length === 0 && (
          <li className="py-3 text-sm text-dim">
            No timed hooks.{" "}
            <Link className="text-go hover:underline" to={`/env/${envID}/hooks/new`}>
              Add one
            </Link>
            .
          </li>
        )}
        {hooks.map((h) => (
          <li key={h.id} className="flex items-center justify-between gap-3 py-2.5">
            <div className="min-w-0">
              <Link to={`/timed-hooks/${h.id}`} className="font-medium hover:text-go">
                {h.name}
              </Link>
              {!h.enabled && <span className="pill bg-raised text-faint">off</span>}
              <p className="mt-0.5 font-mono text-xs text-faint">{h.schedule ? `every ${h.schedule}` : "manual"}</p>
            </div>
            <div className="flex shrink-0 items-center gap-2">
              {h.last ? (
                <span className={badgeClass(h.last.status)}>{h.last.status}</span>
              ) : (
                <span className="font-mono text-xs text-faint">never run</span>
              )}
              <button className="btn btn-sm" onClick={() => run(h.id)}>
                Run now
              </button>
              <Link className="btn btn-sm btn-ghost" to={`/timed-hooks/${h.id}/edit`}>
                Edit
              </Link>
            </div>
          </li>
        ))}
      </ul>
    </section>
  );
}

function download(href: string, filename: string) {
  const a = document.createElement("a");
  a.href = href;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
}
