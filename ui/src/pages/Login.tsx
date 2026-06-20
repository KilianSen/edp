import { useEffect, useState } from "react";
import { api } from "../lib/api";
import { useAuth } from "../lib/auth";

// Login doubles as first-run setup: if no admin password is set yet, the same
// form sets it. Mirrors the old login.html + doLogin first-run flow.
export function Login() {
  const { login } = useAuth();
  const [firstRun, setFirstRun] = useState(false);
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    api
      .authStatus()
      .then((s) => setFirstRun(!s.configured))
      .catch(() => {});
  }, []);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await login(password);
      // success → AuthProvider flips `authed`, App swaps in the dashboard
    } catch (err) {
      setError(err instanceof Error ? err.message : "login failed");
      setBusy(false);
    }
  }

  return (
    <main className="mx-auto max-w-5xl px-5 py-8">
      <div className="mx-auto mt-16 max-w-sm">
        <div className="card relative overflow-hidden p-7" style={{ animation: "edp-rise .4s ease both" }}>
          <div className="rail" data-state="running" />
          <p className="eyebrow">{firstRun ? "First run" : "Control console"}</p>
          <h1 className="mt-1.5 font-display text-2xl font-semibold tracking-tight">
            {firstRun ? "Set an admin password" : "Sign in to edp"}
          </h1>
          <p className="mt-1.5 text-sm text-dim">
            {firstRun
              ? "Choose a password to lock the dashboard — at least 8 characters."
              : "One password guards the dashboard and API."}
          </p>

          {error && (
            <p className="mt-4 rounded-lg border border-fail/30 bg-fail-soft px-3 py-2 text-sm text-fail">{error}</p>
          )}

          <form onSubmit={submit} className="mt-5 space-y-4">
            <label className="lbl">
              Password
              <input
                type="password"
                autoFocus
                required
                minLength={8}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className="field"
                placeholder="••••••••"
              />
            </label>
            <button type="submit" disabled={busy} className="btn btn-primary w-full">
              {firstRun ? "Set password & continue" : "Sign in"}
            </button>
          </form>
        </div>
        <p className="mt-4 text-center font-mono text-[11px] text-faint">
          edp · self-hosted test-environment deployer
        </p>
      </div>
    </main>
  );
}
