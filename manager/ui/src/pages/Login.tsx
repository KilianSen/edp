import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import { api } from "../lib/api";
import { useAuth } from "../lib/auth";

export default function Login() {
  const { login } = useAuth();
  const nav = useNavigate();
  const [configured, setConfigured] = useState<boolean | null>(null);
  const [password, setPassword] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    api.authStatus().then((s) => setConfigured(s.configured)).catch(() => setConfigured(true));
  }, []);

  const firstRun = configured === false;

  async function submit(e: FormEvent) {
    e.preventDefault();
    setErr("");
    setBusy(true);
    try {
      await login(password);
      nav("/", { replace: true });
    } catch (e) {
      setErr(e instanceof Error ? e.message : "login failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="grid min-h-screen place-items-center px-4">
      <form onSubmit={submit} className="w-full max-w-sm rounded-xl border border-[#262e42] bg-[#131826] p-7">
        <h1 className="mb-1 text-lg font-semibold">edp-manager</h1>
        <p className="mb-5 text-sm text-[#8595b6]">
          {firstRun ? "Set an admin password to get started." : "Sign in to manage your edp instances."}
        </p>
        <input
          type="password"
          autoFocus
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          placeholder={firstRun ? "Choose a password (min 8 chars)" : "Password"}
          className="mb-3 w-full rounded-lg border border-[#33405d] bg-[#0a0e16] px-3 py-2 text-sm outline-none focus:border-[#2dd4bf]"
        />
        {err && <p className="mb-3 text-sm text-[#f87171]">{err}</p>}
        <button
          disabled={busy}
          className="w-full rounded-lg bg-[#2dd4bf] py-2 text-sm font-medium text-[#06241f] disabled:opacity-50"
        >
          {firstRun ? "Set password & continue" : "Sign in"}
        </button>
      </form>
    </div>
  );
}
