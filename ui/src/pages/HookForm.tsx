import { useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { api } from "../lib/api";
import { useAsync } from "../lib/useAsync";
import type { TimedHook } from "../lib/types";

function blankHook(envID: number): TimedHook {
  return { id: 0, env_id: envID, name: "", schedule: "", script: "", enabled: true, status: "", created_at: "", updated_at: "" };
}

export function HookForm() {
  const { envId, hookId } = useParams();
  const editing = !!hookId;
  const navigate = useNavigate();

  const { data, error } = useAsync<{ hook: TimedHook; envName: string }>(async () => {
    if (editing) {
      const hook = await api.getHook(Number(hookId));
      const env = await api.getEnv(hook.env_id);
      return { hook, envName: env.name };
    }
    const env = await api.getEnv(Number(envId));
    return { hook: blankHook(env.id), envName: env.name };
  }, [envId, hookId]);

  if (error) return <p className="text-sm text-fail">{error}</p>;
  if (!data) return <p className="text-sm text-dim">Loading…</p>;
  return <HookFormBody initial={data.hook} envName={data.envName} editing={editing} onDone={(id) => navigate(`/timed-hooks/${id}`)} />;
}

function HookFormBody({
  initial,
  envName,
  editing,
  onDone,
}: {
  initial: TimedHook;
  envName: string;
  editing: boolean;
  onDone: (id: number) => void;
}) {
  const [hook, setHook] = useState<TimedHook>(initial);
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const set = <K extends keyof TimedHook>(k: K, v: TimedHook[K]) => setHook((h) => ({ ...h, [k]: v }));
  const backTo = editing ? `/timed-hooks/${hook.id}` : `/env/${hook.env_id}`;

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setErr(null);
    try {
      const saved = editing ? await api.updateHook(hook.id, hook) : await api.createHook(hook.env_id, hook);
      onDone(saved.id);
    } catch (e2) {
      setErr(e2 instanceof Error ? e2.message : "save failed");
      setBusy(false);
    }
  }

  return (
    <>
      <div className="mb-6">
        <Link to={backTo} className="font-mono text-xs text-faint hover:text-dim">
          ← back
        </Link>
        <h1 className="mt-2 font-display text-2xl font-semibold tracking-tight">
          {editing ? "Edit timed hook" : "New timed hook"}
          <span className="font-sans text-base font-normal text-faint"> · {envName}</span>
        </h1>
      </div>

      <p className="mb-5 max-w-2xl text-sm text-dim">
        A timed hook runs a Python script on a schedule <span className="text-fg">without redeploying</span> the
        environment. Your script gets <code>EDP_ENV_NAME</code>, <code>EDP_CONTAINER</code>,{" "}
        <code>EDP_COMPOSE_PROJECT</code>, and <code>EDP_REPO_DIR</code> in its environment.
      </p>

      {err && <p className="mb-4 rounded-lg border border-fail/30 bg-fail-soft px-3 py-2 text-sm text-fail">{err}</p>}

      <form onSubmit={submit} className="space-y-5">
        <section className="card space-y-4 p-5">
          <label className="lbl">
            Name
            <input value={hook.name} onChange={(e) => set("name", e.target.value)} required className="field" placeholder="nightly-db-cleanup" />
          </label>
          <div className="grid items-end gap-4 sm:grid-cols-[1fr_auto]">
            <label className="lbl">
              Schedule <span className="text-faint">(cron or duration; blank = manual only)</span>
              <input value={hook.schedule} onChange={(e) => set("schedule", e.target.value)} className="field" placeholder="15m · 0 3 * * *" />
            </label>
            <label className="flex h-10 items-center gap-2.5 text-sm text-dim">
              <input
                type="checkbox"
                checked={hook.enabled}
                onChange={(e) => set("enabled", e.target.checked)}
                className="h-4 w-4 rounded border-line bg-ink text-go focus:ring-go/40"
              />{" "}
              Enabled
            </label>
          </div>
          <label className="lbl">
            Python script
            <textarea
              value={hook.script}
              onChange={(e) => set("script", e.target.value)}
              rows={10}
              className="field font-mono text-[13px]"
              placeholder={"import subprocess, os\nsubprocess.run(['docker','exec',os.environ['EDP_CONTAINER'],'sh','-c','echo maintenance'], check=True)"}
            />
          </label>
        </section>
        <div className="flex items-center gap-3">
          <button type="submit" disabled={busy} className="btn btn-primary">
            {editing ? "Save changes" : "Create hook"}
          </button>
          <Link className="btn btn-ghost" to={backTo}>
            Cancel
          </Link>
        </div>
      </form>
    </>
  );
}
