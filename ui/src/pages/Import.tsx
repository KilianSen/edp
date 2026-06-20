import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { api } from "../lib/api";

export function Import() {
  const navigate = useNavigate();
  const [json, setJson] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  function onFile(e: React.ChangeEvent<HTMLInputElement>) {
    const f = e.target.files?.[0];
    if (!f) return;
    const rd = new FileReader();
    rd.onload = () => setJson(String(rd.result));
    rd.readAsText(f);
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const res = await api.importBundle(json);
      navigate("/", { state: { imported: res.imported } });
    } catch (err) {
      setError(err instanceof Error ? err.message : "import failed");
      setBusy(false);
    }
  }

  return (
    <>
      <div className="mb-6">
        <Link to="/" className="font-mono text-xs text-faint hover:text-dim">
          ← environments
        </Link>
        <h1 className="mt-2 font-display text-2xl font-semibold tracking-tight">Import environments</h1>
        <p className="mt-1 text-sm text-dim">
          Paste an exported bundle, or pick a <code>.json</code> file. Each environment is created with a fresh name if
          one already exists.
        </p>
      </div>

      {error && <p className="mb-4 rounded-lg border border-fail/30 bg-fail-soft px-3 py-2 text-sm text-fail">{error}</p>}

      <form onSubmit={submit} className="space-y-4">
        <section className="card space-y-4 p-5">
          <label className="lbl">
            JSON file
            <input
              type="file"
              accept="application/json,.json"
              onChange={onFile}
              className="field file:mr-3 file:rounded-md file:border-0 file:bg-raised file:px-3 file:py-1 file:text-fg"
            />
          </label>
          <label className="lbl">
            …or paste the bundle
            <textarea
              value={json}
              onChange={(e) => setJson(e.target.value)}
              rows={14}
              required
              className="field font-mono text-[13px]"
              placeholder='{"version":1,"environments":[ ... ]}'
            />
          </label>
          <p className="text-xs text-dim">
            Exports never contain credentials, but if your JSON includes <code>git_token</code> /{" "}
            <code>registry_password</code> they’ll be imported and encrypted at rest.
          </p>
        </section>
        <div className="flex items-center gap-3">
          <button type="submit" disabled={busy} className="btn btn-primary">
            Import
          </button>
          <Link className="btn btn-ghost" to="/">
            Cancel
          </Link>
        </div>
      </form>
    </>
  );
}
