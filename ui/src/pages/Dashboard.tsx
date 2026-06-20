import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api, downloadExport } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { streamSSE } from "../lib/sse";
import { durMs, badgeClass } from "../lib/format";
import { DeployProgress } from "../components/DeployProgress";
import type { OverviewItem } from "../lib/types";

export function Dashboard() {
  const { data, error, reload } = useAsync(() => api.overview(), []);

  // Live refresh: the events stream nudges us whenever a container changes state.
  useEffect(() => {
    const stop = streamSSE("/api/events/stream", { onEvent: () => reload(), onMessage: () => reload() });
    return stop;
  }, [reload]);

  async function exportAll() {
    const { href, revoke } = await downloadExport("/api/export");
    triggerDownload(href, "edp-environments.json");
    setTimeout(revoke, 1000);
  }

  if (error) return <p className="text-sm text-fail">{error}</p>;
  const envs = data ?? [];

  return (
    <>
      <div className="mb-6 flex items-end justify-between">
        <div>
          <p className="eyebrow">Console</p>
          <h1 className="mt-1 flex items-center gap-2.5 font-display text-2xl font-semibold tracking-tight">
            Environments
            <span className="dot" data-state="running" title="live — updates as containers change" />
          </h1>
        </div>
        <div className="flex items-center gap-2">
          {envs.length > 0 && (
            <button onClick={exportAll} className="btn btn-sm" title="Download all environments as JSON">
              Export
            </button>
          )}
          <Link to="/env/import" className="btn btn-sm">
            Import
          </Link>
          <Link to="/env/new" className="btn btn-primary">
            New environment
          </Link>
        </div>
      </div>

      {envs.length === 0 ? (
        <div className="card px-6 py-12 text-center">
          <p className="font-display text-xl">Spin up your first environment</p>
          <p className="mx-auto mt-1.5 max-w-md text-sm text-dim">
            Choose a starting point. You only need a name and where your code lives — edp builds, runs, and keeps it
            live.
          </p>
          <div className="mx-auto mt-7 flex max-w-2xl flex-col gap-3 text-left sm:flex-row">
            <Link to="/env/new?preset=git" className="preset">
              <h3>Git repo</h3>
              <p>Clone &amp; build from a Dockerfile</p>
            </Link>
            <Link to="/env/new?preset=image" className="preset">
              <h3>Container image</h3>
              <p>Pull a prebuilt image from a registry</p>
            </Link>
            <Link to="/env/new?preset=compose" className="preset">
              <h3>Compose stack</h3>
              <p>Bring up a docker-compose.yml</p>
            </Link>
          </div>
        </div>
      ) : (
        <ul className="space-y-3">
          {envs.map((it) => (
            <EnvRow key={it.env.id} item={it} onChanged={reload} />
          ))}
        </ul>
      )}
    </>
  );
}

function EnvRow({ item, onChanged }: { item: OverviewItem; onChanged: () => void }) {
  const { env, container_state, last, estimate_ms } = item;
  const deploying = !!last && (last.status === "running" || last.status === "queued");
  const [busy, setBusy] = useState(false);

  async function redeploy() {
    setBusy(true);
    try {
      await api.deploy(env.id, "manual");
      onChanged();
    } finally {
      setTimeout(() => setBusy(false), 600);
    }
  }

  return (
    <li
      className="card relative overflow-hidden p-4 pl-5 transition-colors hover:border-line-soft"
      data-state={container_state}
      {...(deploying ? { "data-deploying": "1" } : {})}
    >
      <div className="rail" />
      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <span className="dot" />
            <Link to={`/env/${env.id}`} className="truncate font-display text-base font-semibold tracking-tight hover:text-go">
              {env.name}
            </Link>
          </div>
          <div className="mt-1 flex flex-wrap items-center gap-1.5 pl-4">
            <span className="pill bg-raised text-dim">{env.source_type}</span>
            <span className="pill bg-raised text-dim">{env.deploy_type}</span>
            {env.health_type !== "none" && <span className="pill bg-raised text-dim">health: {env.health_type}</span>}
            {env.listen_port && <span className="pill bg-raised text-dim">:{env.listen_port}</span>}
            {env.proxy_host && <span className="pill bg-raised text-dim">{env.proxy_host}</span>}
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <button onClick={redeploy} disabled={busy} className="btn btn-primary btn-sm">
            Redeploy
          </button>
          <Link className="btn btn-sm" to={`/env/${env.id}/edit`}>
            Edit
          </Link>
        </div>
      </div>

      {deploying && last?.started_at ? (
        <div className="mt-3.5 pl-4">
          <DeployProgress startMs={new Date(last.started_at).getTime()} etaMs={estimate_ms} />
        </div>
      ) : (
        <>
          <div className="mt-3.5 grid grid-cols-2 gap-x-4 gap-y-3 pl-4 sm:grid-cols-4">
            <Stat label="Container">
              {container_state === "none" ? (
                <span className="text-faint">not running</span>
              ) : (
                <span className="capitalize">{container_state}</span>
              )}
            </Stat>
            <Stat label="Last deploy">
              {last ? <span className={badgeClass(last.status)}>{last.status}</span> : <span className="text-faint">never</span>}
            </Stat>
            <Stat label="Schedule">
              {env.redeploy_schedule ? (
                <span className="font-mono text-[13px]">{env.redeploy_schedule}</span>
              ) : (
                <span className="text-faint">manual</span>
              )}
            </Stat>
            <Stat label="Typical start">
              {estimate_ms ? (
                <span className="font-mono text-[13px]">~{durMs(estimate_ms)}</span>
              ) : (
                <span className="text-faint">—</span>
              )}
            </Stat>
          </div>
          {last?.reason && (
            <p className="mt-2.5 truncate pl-4 text-xs text-dim">
              <span className="text-faint">reason:</span> {last.reason}
            </p>
          )}
        </>
      )}
    </li>
  );
}

function Stat({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <p className="stat-label">{label}</p>
      <p className="mt-0.5 text-sm">{children}</p>
    </div>
  );
}

function triggerDownload(href: string, filename: string) {
  const a = document.createElement("a");
  a.href = href;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
}
