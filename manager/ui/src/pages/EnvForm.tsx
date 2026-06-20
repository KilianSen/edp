import { useEffect, useState } from "react";
import type { ChangeEvent, FormEvent, ReactNode } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { api } from "../lib/api";
import type { Environment, Instance } from "../lib/types";
import { blankEnv } from "../lib/types";

const FIELD =
  "w-full rounded-lg border border-[#33405d] bg-[#0a0e16] px-3 py-2 text-sm outline-none focus:border-[#2dd4bf]";

export default function EnvForm() {
  const { instanceId, envId } = useParams();
  const editing = envId != null;
  const nav = useNavigate();

  const [form, setForm] = useState<Environment>(blankEnv());
  const [instances, setInstances] = useState<Instance[]>([]);
  const [target, setTarget] = useState<number>(editing ? Number(instanceId) : 0);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  // the instance this env lives on / will be created on
  const iid = editing ? Number(instanceId) : target;

  useEffect(() => {
    if (editing) {
      api.getEnv(Number(instanceId), Number(envId)).then(setForm).catch((e) => setErr(e instanceof Error ? e.message : "load failed"));
      return;
    }
    // create: load instances so placement can default automatically
    api
      .listInstances()
      .then((l) => {
        const list = l ?? [];
        setInstances(list);
        setTarget((t) => t || list[0]?.id || 0);
      })
      .catch((e) => setErr(e instanceof Error ? e.message : "load failed"));
  }, [editing, instanceId, envId]);

  // bound-input prop helpers (cast the dynamic-key spread back to Environment)
  const txt = (k: keyof Environment) => ({
    value: (form[k] as string) ?? "",
    onChange: (e: ChangeEvent<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>) =>
      setForm((f) => ({ ...f, [k]: e.target.value }) as Environment),
  });
  const chk = (k: keyof Environment) => ({
    checked: !!form[k],
    onChange: (e: ChangeEvent<HTMLInputElement>) => setForm((f) => ({ ...f, [k]: e.target.checked }) as Environment),
  });

  async function submit(e: FormEvent) {
    e.preventDefault();
    setErr("");
    if (!form.name.trim()) {
      setErr("name is required");
      return;
    }
    if (!editing && !target) {
      setErr("no instance available — register one under Instances first");
      return;
    }
    setBusy(true);
    try {
      const saved = editing ? await api.updateEnv(iid, Number(envId), form) : await api.createEnv(iid, form);
      nav(`/i/${iid}/env/${saved.id}`);
    } catch (e) {
      setErr(e instanceof Error ? e.message : "save failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="max-w-3xl">
      <div className="mb-4">
        <Link to={editing ? `/i/${iid}/env/${envId}` : "/"} className="text-sm text-[#8595b6] hover:text-[#e7ebf4]">
          ← Back
        </Link>
      </div>
      <h1 className="mb-5 text-xl font-semibold">{editing ? `Edit ${form.name}` : "New environment"}</h1>

      {err && <div className="mb-4 rounded-lg border border-[#5b2330] bg-[#2a141a] px-4 py-2 text-sm text-[#f8a3ad]">{err}</div>}

      <form onSubmit={submit} className="space-y-6">
        <Section title="Basics">
          {!editing && instances.length > 1 && (
            <Lbl label="Instance">
              <select value={target} onChange={(e) => setTarget(Number(e.target.value))} className={FIELD}>
                {instances.map((i) => (
                  <option key={i.id} value={i.id}>
                    {i.label}
                  </option>
                ))}
              </select>
            </Lbl>
          )}
          {!editing && instances.length === 0 && (
            <p className="text-sm text-[#f5d08a]">
              No instances registered — add one under <b>Instances</b> before creating an environment.
            </p>
          )}
          <Lbl label="Name">
            <input {...txt("name")} className={FIELD} placeholder="my-test-env" />
          </Lbl>
          <div className="grid gap-4 sm:grid-cols-2">
            <Lbl label="Source">
              <select {...txt("source_type")} className={FIELD}>
                <option value="git">git (build)</option>
                <option value="registry">registry (pull)</option>
                <option value="dockerfile">inline Dockerfile</option>
              </select>
            </Lbl>
            <Lbl label="Deploy as">
              <select {...txt("deploy_type")} className={FIELD}>
                <option value="container">container</option>
                <option value="compose">compose</option>
              </select>
            </Lbl>
          </div>
        </Section>

        {form.source_type === "git" && (
          <Section title="Git source">
            <Lbl label="Repo URL">
              <input {...txt("repo_url")} className={FIELD} placeholder="https://github.com/org/repo.git" />
            </Lbl>
            <div className="grid gap-4 sm:grid-cols-2">
              <Lbl label="Ref (branch/tag/sha)">
                <input {...txt("git_ref")} className={FIELD} placeholder="main" />
              </Lbl>
              <Lbl label="Access token (optional)">
                <input {...txt("git_token")} type="password" className={FIELD} placeholder="leave blank to keep" />
              </Lbl>
            </div>
          </Section>
        )}

        {form.source_type === "registry" && (
          <Section title="Registry source">
            <Lbl label="Image">
              <input {...txt("registry_image")} className={FIELD} placeholder="nginx:alpine" />
            </Lbl>
            <div className="grid gap-4 sm:grid-cols-2">
              <Lbl label="Username (optional)">
                <input {...txt("registry_username")} className={FIELD} />
              </Lbl>
              <Lbl label="Password (optional)">
                <input {...txt("registry_password")} type="password" className={FIELD} placeholder="leave blank to keep" />
              </Lbl>
            </div>
          </Section>
        )}

        {form.source_type === "dockerfile" && (
          <Section title="Inline Dockerfile">
            <Lbl label="Dockerfile">
              <textarea {...txt("dockerfile_content")} rows={6} className={`${FIELD} font-mono`} />
            </Lbl>
          </Section>
        )}

        {form.deploy_type === "container" && (
          <Section title="Container run config">
            {form.source_type === "git" && (
              <div className="grid gap-4 sm:grid-cols-3">
                <Lbl label="Dockerfile path">
                  <input {...txt("dockerfile_path")} className={FIELD} placeholder="Dockerfile" />
                </Lbl>
                <Lbl label="Build context">
                  <input {...txt("build_context")} className={FIELD} placeholder="." />
                </Lbl>
                <Lbl label="Image name">
                  <input {...txt("image_name")} className={FIELD} />
                </Lbl>
              </div>
            )}
            <div className="grid gap-4 sm:grid-cols-2">
              <Lbl label="Ports (8080:80, …)">
                <input {...txt("run_ports")} className={`${FIELD} font-mono`} />
              </Lbl>
              <Lbl label="Restart policy">
                <input {...txt("restart_policy")} className={FIELD} placeholder="unless-stopped" />
              </Lbl>
            </div>
            <Lbl label="Environment variables (KEY=VALUE per line)">
              <textarea {...txt("run_env")} rows={3} className={`${FIELD} font-mono`} />
            </Lbl>
            <Lbl label="Volumes (name:/path per line)">
              <textarea {...txt("run_volumes")} rows={2} className={`${FIELD} font-mono`} />
            </Lbl>
            <Lbl label="Networks (comma separated)">
              <input {...txt("run_networks")} className={FIELD} />
            </Lbl>
            <div className="grid gap-4 sm:grid-cols-2">
              <Lbl label="Entrypoint override">
                <input {...txt("entrypoint")} className={FIELD} />
              </Lbl>
              <Lbl label="Command override">
                <input {...txt("command")} className={`${FIELD} font-mono`} />
              </Lbl>
            </div>
            <Lbl label="Entrypoint script (inline)">
              <textarea {...txt("entrypoint_script")} rows={3} className={`${FIELD} font-mono`} />
            </Lbl>
          </Section>
        )}

        {form.deploy_type === "compose" && (
          <Section title="Compose">
            <Lbl label="Compose file path">
              <input {...txt("compose_path")} className={FIELD} placeholder="docker-compose.yml" />
            </Lbl>
            <Lbl label="Compose override (inline, merged)">
              <textarea {...txt("compose_override")} rows={5} className={`${FIELD} font-mono`} />
            </Lbl>
          </Section>
        )}

        <Section title="Proxy">
          <div className="grid gap-4 sm:grid-cols-3">
            <Lbl label="Proxy host (optional)">
              <input {...txt("proxy_host")} className={FIELD} placeholder="app.test.local" />
            </Lbl>
            <Lbl label="Container port">
              <input {...txt("proxy_port")} className={`${FIELD} font-mono`} placeholder="80" />
            </Lbl>
            <Lbl label="TCP forward port">
              <input {...txt("listen_port")} className={`${FIELD} font-mono`} />
            </Lbl>
          </div>
          <Lbl label="Splash UI URL (optional)">
            <input {...txt("splash_url")} className={FIELD} />
          </Lbl>
        </Section>

        <Section title="Lifecycle">
          <div className="grid gap-4 sm:grid-cols-2">
            <Lbl label="Redeploy schedule (duration or cron)">
              <input {...txt("redeploy_schedule")} className={`${FIELD} font-mono`} placeholder="30m or 0 */6 * * *" />
            </Lbl>
            <Lbl label="Volume sweep">
              <select {...txt("volume_sweep")} className={FIELD}>
                <option value="none">none</option>
                <option value="named">named</option>
                <option value="all">all</option>
              </select>
            </Lbl>
            <Lbl label="Health check">
              <select {...txt("health_type")} className={FIELD}>
                <option value="none">none</option>
                <option value="http">http</option>
                <option value="exec">exec</option>
              </select>
            </Lbl>
            <Lbl label="Health target (URL or command)">
              <input {...txt("health_target")} className={FIELD} />
            </Lbl>
          </div>
          <div className="flex gap-6 pt-1 text-sm text-[#c7d0e4]">
            <label className="flex items-center gap-2">
              <input type="checkbox" {...chk("auto_deploy")} /> Auto-deploy on save
            </label>
            <label className="flex items-center gap-2">
              <input type="checkbox" {...chk("prune_images")} /> Prune images after deploy
            </label>
          </div>
        </Section>

        <div className="flex gap-3">
          <button
            disabled={busy}
            className="rounded-lg bg-[#2dd4bf] px-5 py-2 text-sm font-medium text-[#06241f] disabled:opacity-50"
          >
            {busy ? "Saving…" : editing ? "Save changes" : "Create environment"}
          </button>
          <Link
            to={editing ? `/i/${iid}/env/${envId}` : "/"}
            className="rounded-lg border border-[#33405d] px-5 py-2 text-sm text-[#8595b6] hover:text-[#e7ebf4]"
          >
            Cancel
          </Link>
        </div>
      </form>
    </div>
  );
}

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <fieldset className="rounded-xl border border-[#262e42] bg-[#131826] p-4">
      <legend className="px-1 text-sm font-medium text-[#8595b6]">{title}</legend>
      <div className="space-y-4">{children}</div>
    </fieldset>
  );
}

function Lbl({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="block">
      <span className="mb-1 block text-[11px] uppercase tracking-wide text-[#54607e]">{label}</span>
      {children}
    </label>
  );
}
