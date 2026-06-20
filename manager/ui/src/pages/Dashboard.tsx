import { useCallback, useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../lib/api";
import type { ManagedOverview, OverviewFanout } from "../lib/types";
import { durMs, relTime, sourceSummary, statusBadge } from "../lib/format";
import { useLiveRefresh } from "../lib/useLiveRefresh";
import HealthSummary from "../components/HealthSummary";
import LogDrawer from "../components/LogDrawer";

interface ActiveLog {
  instanceID: number;
  deploymentID: number;
  title: string;
}

export default function Dashboard() {
  const [data, setData] = useState<OverviewFanout | null>(null);
  const [err, setErr] = useState("");
  const [q, setQ] = useState("");
  const [busy, setBusy] = useState<string | null>(null);
  const [log, setLog] = useState<ActiveLog | null>(null);

  const load = useCallback(() => {
    api
      .overview()
      .then((d) => {
        setData(d);
        setErr("");
      })
      .catch((e) => setErr(e instanceof Error ? e.message : "load failed"));
  }, []);

  useEffect(() => {
    load();
    const t = setInterval(load, 10000); // fallback poll; SSE drives instant updates
    return () => clearInterval(t);
  }, [load]);

  useLiveRefresh(load);

  async function redeploy(it: ManagedOverview) {
    const key = `${it.instance_id}-${it.env.id}`;
    setBusy(key);
    try {
      const { deployment_id } = await api.deploy(it.instance_id, it.env.id, "manual (manager)");
      setLog({ instanceID: it.instance_id, deploymentID: deployment_id, title: it.env.name });
      setTimeout(load, 800);
    } catch (e) {
      setErr(e instanceof Error ? e.message : "redeploy failed");
    } finally {
      setBusy(null);
    }
  }

  async function remove(it: ManagedOverview) {
    if (!confirm(`Delete "${it.env.name}" on ${it.instance_label}? This removes it from edp.`)) return;
    try {
      await api.deleteEnv(it.instance_id, it.env.id);
      load();
    } catch (e) {
      setErr(e instanceof Error ? e.message : "delete failed");
    }
  }

  // one unified, instance-agnostic list — instance is just a tag on each row
  const rows = useMemo(() => {
    const items = [...(data?.items ?? [])].sort((a, b) => a.env.name.localeCompare(b.env.name));
    const needle = q.trim().toLowerCase();
    if (!needle) return items;
    return items.filter((it) =>
      [it.env.name, it.instance_label, it.env.status, sourceSummary(it.env)]
        .join(" ")
        .toLowerCase()
        .includes(needle),
    );
  }, [data, q]);

  return (
    <div>
      <div className="mb-4 flex items-center gap-3">
        <h1 className="text-xl font-semibold">Environments</h1>
        <input
          value={q}
          onChange={(e) => setQ(e.target.value)}
          placeholder="Search…"
          className="ml-2 w-56 rounded-lg border border-[#33405d] bg-[#0a0e16] px-3 py-1.5 text-sm outline-none focus:border-[#2dd4bf]"
        />
        <Link
          to="/env/new"
          className="ml-auto rounded-lg bg-[#2dd4bf] px-4 py-1.5 text-sm font-medium text-[#06241f]"
        >
          New environment
        </Link>
      </div>

      <HealthSummary />

      {err && (
        <div className="mb-4 rounded-lg border border-[#5b2330] bg-[#2a141a] px-4 py-2 text-sm text-[#f8a3ad]">{err}</div>
      )}
      {data?.errors?.map((e) => (
        <div
          key={e.instance_id}
          className="mb-3 rounded-lg border border-[#5b4a23] bg-[#2a2414] px-4 py-2 text-sm text-[#f5d08a]"
        >
          <b>{e.instance_label}</b> unreachable: {e.error}
        </div>
      ))}

      <div className="overflow-hidden rounded-xl border border-[#262e42]">
        {rows.map((it) => {
          const e = it.env;
          const proxy = e.proxy_host || (e.proxy_port ? `/e/${e.name}/` : "");
          const key = `${it.instance_id}-${e.id}`;
          return (
            <div
              key={key}
              className="flex flex-wrap items-center gap-x-3 gap-y-1 border-b border-[#1c2336] px-4 py-3 last:border-0"
            >
              <Link to={`/i/${it.instance_id}/env/${e.id}`} className="font-medium hover:text-[#2dd4bf]">
                {e.name}
              </Link>
              <span className={statusBadge(e.status || "idle")}>{e.status || "idle"}</span>
              <span className="rounded bg-[#1b2236] px-1.5 py-0.5 text-[11px] text-[#8595b6]" title="instance">
                {it.instance_label}
              </span>
              {it.container_state && it.container_state !== "none" && (
                <span className="text-xs text-[#54607e]">{it.container_state}</span>
              )}
              <span className="truncate text-xs text-[#8595b6]" title={sourceSummary(e)}>
                {sourceSummary(e)}
              </span>
              {proxy && <span className="font-mono text-xs text-[#54607e]">{proxy}</span>}
              {it.last && (
                <span className="text-xs text-[#54607e]">
                  {relTime(it.last.finished_at || it.last.started_at)}
                  {it.last.duration_ms ? ` · ${durMs(it.last.duration_ms)}` : ""}
                </span>
              )}
              <div className="ml-auto flex gap-2">
                {it.last && (
                  <button
                    onClick={() => setLog({ instanceID: it.instance_id, deploymentID: it.last!.id, title: e.name })}
                    className="rounded-md border border-[#33405d] px-3 py-1 text-xs text-[#8595b6] hover:text-[#e7ebf4]"
                  >
                    Logs
                  </button>
                )}
                <button
                  onClick={() => redeploy(it)}
                  disabled={busy === key}
                  className="rounded-md border border-[#33405d] px-3 py-1 text-xs text-[#8595b6] hover:text-[#e7ebf4] disabled:opacity-50"
                >
                  {busy === key ? "…" : "Redeploy"}
                </button>
                <button
                  onClick={() => remove(it)}
                  className="rounded-md border border-[#5b2330] px-3 py-1 text-xs text-[#f8a3ad] hover:text-[#fecdd3]"
                >
                  Delete
                </button>
              </div>
            </div>
          );
        })}
        {rows.length === 0 && (
          <p className="px-4 py-6 text-center text-sm text-[#8595b6]">
            {data && (data.items.length === 0 ? "No environments yet." : "No matches.")}
          </p>
        )}
      </div>

      {log && (
        <LogDrawer
          instanceID={log.instanceID}
          deploymentID={log.deploymentID}
          title={log.title}
          onClose={() => setLog(null)}
          onDone={load}
        />
      )}
    </div>
  );
}
