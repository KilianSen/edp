import { useCallback, useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { api } from "../lib/api";
import type { Deployment, Environment } from "../lib/types";
import { durMs, fmtDate, relTime, sourceSummary, statusBadge } from "../lib/format";
import LogStream from "../components/LogStream";

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <div className="text-[11px] uppercase tracking-wide text-[#54607e]">{label}</div>
      <div className="mt-0.5 break-words text-sm text-[#c7d0e4]">{children || "—"}</div>
    </div>
  );
}

export default function EnvDetail() {
  const { instanceId, envId } = useParams();
  const nav = useNavigate();
  const iid = Number(instanceId);
  const eid = Number(envId);

  const [env, setEnv] = useState<Environment | null>(null);
  const [label, setLabel] = useState("");
  const [deps, setDeps] = useState<Deployment[]>([]);
  const [selected, setSelected] = useState<Deployment | null>(null);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  const loadDeps = useCallback(() => {
    api
      .listDeployments(iid, eid)
      .then((d) => {
        const list = d ?? [];
        setDeps(list);
        setSelected((cur) => cur ?? list[0] ?? null);
      })
      .catch((e) => setErr(e instanceof Error ? e.message : "load failed"));
  }, [iid, eid]);

  useEffect(() => {
    api.getEnv(iid, eid).then(setEnv).catch((e) => setErr(e instanceof Error ? e.message : "load failed"));
    api.listInstances().then((l) => setLabel(l?.find((i) => i.id === iid)?.label ?? `#${iid}`)).catch(() => {});
    loadDeps();
  }, [iid, eid, loadDeps]);

  async function redeploy() {
    setBusy(true);
    setErr("");
    try {
      const { deployment_id } = await api.deploy(iid, eid, "manual (manager)");
      // optimistically select the new deployment so its log streams immediately
      setSelected({ id: deployment_id, env_id: eid, status: "running" } as Deployment);
      setTimeout(loadDeps, 800);
    } catch (e) {
      setErr(e instanceof Error ? e.message : "redeploy failed");
    } finally {
      setBusy(false);
    }
  }

  async function remove() {
    if (!confirm(`Delete environment "${env?.name}" on ${label}? This removes it from edp.`)) return;
    try {
      await api.deleteEnv(iid, eid);
      nav("/");
    } catch (e) {
      setErr(e instanceof Error ? e.message : "delete failed");
    }
  }

  if (!env) {
    return (
      <div className="text-sm text-[#8595b6]">
        {err ? <span className="text-[#f8a3ad]">{err}</span> : "Loading…"}
      </div>
    );
  }

  const proxy = env.proxy_host || (env.proxy_port ? `/e/${env.name}/` : "");

  return (
    <div>
      <div className="mb-4 flex items-center gap-3">
        <Link to="/" className="text-sm text-[#8595b6] hover:text-[#e7ebf4]">
          ← Dashboard
        </Link>
        <span className="text-[#33405d]">/</span>
        <span className="text-sm text-[#8595b6]">{label}</span>
      </div>

      <div className="mb-5 flex flex-wrap items-center gap-3">
        <h1 className="text-xl font-semibold">{env.name}</h1>
        <span className={statusBadge(env.status || "idle")}>{env.status || "idle"}</span>
        <div className="ml-auto flex gap-2">
          <Link
            to={`/i/${iid}/env/${eid}/edit`}
            className="rounded-lg border border-[#33405d] px-4 py-1.5 text-sm text-[#8595b6] hover:text-[#e7ebf4]"
          >
            Edit
          </Link>
          <button
            onClick={remove}
            className="rounded-lg border border-[#5b2330] px-4 py-1.5 text-sm text-[#f8a3ad] hover:text-[#fecdd3]"
          >
            Delete
          </button>
          <button
            onClick={redeploy}
            disabled={busy}
            className="rounded-lg bg-[#2dd4bf] px-4 py-1.5 text-sm font-medium text-[#06241f] disabled:opacity-50"
          >
            {busy ? "Deploying…" : "Redeploy"}
          </button>
        </div>
      </div>

      {err && <div className="mb-4 rounded-lg border border-[#5b2330] bg-[#2a141a] px-4 py-2 text-sm text-[#f8a3ad]">{err}</div>}

      {/* config */}
      <div className="mb-6 grid grid-cols-2 gap-4 rounded-xl border border-[#262e42] bg-[#131826] p-4 sm:grid-cols-3">
        <Field label="Source">{`${env.source_type} · ${sourceSummary(env)}`}</Field>
        <Field label="Deploy">{env.deploy_type}</Field>
        <Field label="Ports">{env.run_ports}</Field>
        <Field label="Proxy">{proxy ? `${proxy}${env.proxy_port ? ` → :${env.proxy_port}` : ""}` : ""}</Field>
        <Field label="TCP forward">{env.listen_port}</Field>
        <Field label="Schedule">{env.redeploy_schedule}</Field>
        <Field label="Health">{env.health_type !== "none" ? `${env.health_type} ${env.health_target}` : ""}</Field>
        <Field label="Restart">{env.restart_policy}</Field>
        <Field label="Volume sweep">{env.volume_sweep}</Field>
        <Field label="Auto-deploy">{env.auto_deploy ? "yes" : "no"}</Field>
        <Field label="Prune images">{env.prune_images ? "yes" : "no"}</Field>
        <Field label="Updated">{fmtDate(env.updated_at)}</Field>
      </div>
      {env.run_env && (
        <div className="mb-6">
          <div className="mb-1 text-[11px] uppercase tracking-wide text-[#54607e]">Environment variables</div>
          <pre className="overflow-auto rounded-lg border border-[#1c2336] bg-[#05070c] p-3 font-mono text-xs text-[#c7d0e4]">
            {env.run_env}
          </pre>
        </div>
      )}

      {/* deployments + log */}
      <div className="grid gap-4 lg:grid-cols-[320px_1fr]">
        <div>
          <h2 className="mb-2 text-sm font-medium uppercase tracking-wide text-[#8595b6]">Deployments</h2>
          <div className="overflow-hidden rounded-xl border border-[#262e42]">
            {deps.map((d) => (
              <button
                key={d.id}
                onClick={() => setSelected(d)}
                className={`flex w-full flex-col items-start gap-0.5 border-b border-[#1c2336] px-3 py-2 text-left last:border-0 hover:bg-[#161d2e] ${
                  selected?.id === d.id ? "bg-[#161d2e]" : ""
                }`}
              >
                <div className="flex w-full items-center gap-2">
                  <span className={statusBadge(d.status)}>{d.status}</span>
                  <span className="text-xs text-[#8595b6]">{d.trigger}</span>
                  <span className="ml-auto text-xs text-[#54607e]">{durMs(d.duration_ms)}</span>
                </div>
                <div className="text-xs text-[#54607e]">
                  {relTime(d.finished_at || d.started_at)}
                  {d.reason ? ` · ${d.reason}` : ""}
                </div>
              </button>
            ))}
            {deps.length === 0 && <p className="px-3 py-2 text-xs text-[#8595b6]">No deployments yet.</p>}
          </div>
        </div>

        <div className="rounded-xl border border-[#262e42] bg-[#0c111c] p-4">
          {selected ? (
            <LogStream
              key={selected.id}
              instanceID={iid}
              deploymentID={selected.id}
              initialStatus={selected.status}
              heightClass="h-[28rem]"
              onDone={loadDeps}
            />
          ) : (
            <p className="text-sm text-[#8595b6]">Select a deployment to view its log.</p>
          )}
        </div>
      </div>
    </div>
  );
}
