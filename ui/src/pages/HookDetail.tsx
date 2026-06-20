import { Link, useNavigate, useParams, useSearchParams } from "react-router-dom";
import { api } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import { badgeClass, fmtDate } from "../lib/format";
import { LogStream } from "../components/LogStream";
import type { HookRun, TimedHook } from "../lib/types";

export function HookDetail() {
  const { id } = useParams();
  const hookID = Number(id);
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const runParam = params.get("run");

  const { data, error, reload } = useAsync<{ hook: TimedHook; runs: HookRun[]; envName: string }>(async () => {
    const hook = await api.getHook(hookID);
    const [runs, env] = await Promise.all([api.listHookRuns(hookID), api.getEnv(hook.env_id)]);
    return { hook, runs, envName: env.name };
  }, [hookID]);

  if (error) return <p className="text-sm text-fail">{error}</p>;
  if (!data) return <p className="text-sm text-dim">Loading…</p>;

  const { hook, runs, envName } = data;
  const focus = runParam ? runs.find((r) => r.id === Number(runParam)) : runs[0];

  async function runNow() {
    await api.runHook(hookID);
    setTimeout(reload, 600);
  }
  async function remove() {
    if (!confirm("Delete this hook?")) return;
    await api.deleteHook(hookID);
    navigate(`/env/${hook.env_id}`);
  }

  return (
    <>
      <div className="mb-6">
        <Link to={`/env/${hook.env_id}`} className="font-mono text-xs text-faint hover:text-dim">
          ← {envName}
        </Link>
        <div className="mt-2 flex flex-wrap items-start justify-between gap-4">
          <div>
            <h1 className="flex items-center gap-2 font-display text-2xl font-semibold tracking-tight">
              {hook.name}
              {!hook.enabled && <span className="pill bg-raised text-faint">disabled</span>}
            </h1>
            <p className="mt-1 font-mono text-xs text-faint">
              timed hook · {hook.schedule ? `every ${hook.schedule}` : "manual only"}
            </p>
          </div>
          <div className="flex items-center gap-2">
            <button className="btn btn-primary" onClick={runNow}>
              Run now
            </button>
            <Link className="btn" to={`/timed-hooks/${hook.id}/edit`}>
              Edit
            </Link>
            <button className="btn btn-ghost" onClick={remove}>
              Delete
            </button>
          </div>
        </div>
      </div>

      <div className="grid gap-5 lg:grid-cols-2">
        <section className="card p-5">
          <p className="eyebrow">Runs</p>
          <ul className="mt-3 divide-y divide-line-soft">
            {runs.length === 0 && <li className="py-3 text-sm text-dim">No runs yet.</li>}
            {runs.map((r) => (
              <li key={r.id} className="flex items-center justify-between gap-3 py-2.5">
                <div className="flex items-center gap-2">
                  <span className={badgeClass(r.status)}>{r.status}</span>
                  <span className="font-mono text-xs text-faint">{r.trigger}</span>
                </div>
                <div className="shrink-0 text-right">
                  <Link className="font-mono text-xs text-go hover:underline" to={`/timed-hooks/${hook.id}?run=${r.id}`}>
                    log
                  </Link>
                  <p className="mt-0.5 font-mono text-[11px] text-faint">{fmtDate(r.started_at, true)}</p>
                </div>
              </li>
            ))}
          </ul>
        </section>

        <section className="card flex flex-col p-5">
          {focus ? (
            <LogStream key={focus.id} streamPath={`/api/hook-runs/${focus.id}/logs/stream`} status={focus.status} onDone={reload} />
          ) : (
            <>
              <p className="eyebrow">Log</p>
              <p className="mt-3 text-sm text-dim">Run the hook to stream its output here.</p>
            </>
          )}
        </section>
      </div>

      <section className="card mt-5 p-5">
        <p className="eyebrow">Script</p>
        <pre className="mt-3 max-h-64 overflow-auto whitespace-pre-wrap rounded-lg border border-line bg-[#05070c] p-3 font-mono text-[12.5px] leading-relaxed">
          {hook.script}
        </pre>
      </section>
    </>
  );
}
