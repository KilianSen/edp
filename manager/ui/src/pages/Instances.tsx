import { useCallback, useEffect, useState } from "react";
import type { FormEvent } from "react";
import { api } from "../lib/api";
import type { Instance } from "../lib/types";

export default function Instances() {
  const [list, setList] = useState<Instance[]>([]);
  const [err, setErr] = useState("");
  const [form, setForm] = useState({ label: "", base_url: "", api_token: "" });
  const [tests, setTests] = useState<Record<number, string>>({});

  const load = useCallback(() => {
    api
      .listInstances()
      .then((l) => setList(l ?? []))
      .catch((e) => setErr(e instanceof Error ? e.message : "load failed"));
  }, []);

  useEffect(() => load(), [load]);

  async function add(e: FormEvent) {
    e.preventDefault();
    setErr("");
    try {
      await api.createInstance(form);
      setForm({ label: "", base_url: "", api_token: "" });
      load();
    } catch (e) {
      setErr(e instanceof Error ? e.message : "create failed");
    }
  }

  async function remove(id: number) {
    if (!confirm("Remove this instance from the manager? (the edp instance itself is untouched)")) return;
    await api.deleteInstance(id);
    load();
  }

  async function test(id: number) {
    setTests((t) => ({ ...t, [id]: "…" }));
    try {
      const r = await api.testInstance(id);
      setTests((t) => ({ ...t, [id]: r.ok ? "reachable ✓" : `error: ${r.error}` }));
    } catch (e) {
      setTests((t) => ({ ...t, [id]: e instanceof Error ? e.message : "error" }));
    }
  }

  const input = "rounded-lg border border-[#33405d] bg-[#0a0e16] px-3 py-2 text-sm outline-none focus:border-[#2dd4bf]";

  return (
    <div>
      <h1 className="mb-4 text-xl font-semibold">Instances</h1>

      {err && <div className="mb-4 rounded-lg border border-[#5b2330] bg-[#2a141a] px-4 py-2 text-sm text-[#f8a3ad]">{err}</div>}

      <form onSubmit={add} className="mb-6 grid gap-3 rounded-xl border border-[#262e42] bg-[#131826] p-4 sm:grid-cols-4">
        <input
          className={input}
          placeholder="label (e.g. prod)"
          value={form.label}
          onChange={(e) => setForm({ ...form, label: e.target.value })}
        />
        <input
          className={`${input} sm:col-span-2`}
          placeholder="base URL (https://edp-1.internal)"
          value={form.base_url}
          onChange={(e) => setForm({ ...form, base_url: e.target.value })}
        />
        <input
          className={input}
          type="password"
          placeholder="API token"
          value={form.api_token}
          onChange={(e) => setForm({ ...form, api_token: e.target.value })}
        />
        <button className="rounded-lg bg-[#2dd4bf] py-2 text-sm font-medium text-[#06241f] sm:col-span-4">
          Add instance
        </button>
      </form>

      <div className="overflow-hidden rounded-xl border border-[#262e42]">
        {list.map((i) => (
          <div key={i.id} className="flex flex-wrap items-center gap-3 border-b border-[#1c2336] px-4 py-3 last:border-0">
            <span className="font-medium">{i.label}</span>
            <span className="text-sm text-[#8595b6]">{i.base_url}</span>
            {tests[i.id] && <span className="text-xs text-[#f5b544]">{tests[i.id]}</span>}
            <div className="ml-auto flex gap-2">
              <button onClick={() => test(i.id)} className="rounded-md border border-[#33405d] px-3 py-1 text-xs text-[#8595b6] hover:text-[#e7ebf4]">
                Test
              </button>
              <button onClick={() => remove(i.id)} className="rounded-md border border-[#5b2330] px-3 py-1 text-xs text-[#f8a3ad] hover:text-[#fecdd3]">
                Remove
              </button>
            </div>
          </div>
        ))}
        {list.length === 0 && <p className="px-4 py-3 text-sm text-[#8595b6]">No instances registered yet.</p>}
      </div>
    </div>
  );
}
