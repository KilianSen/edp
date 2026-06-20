import { useEffect, useState } from "react";
import { api } from "../lib/api";
import type { Summary } from "../lib/types";

// HealthSummary shows the fleet roll-up: totals plus an up/down chip per
// instance with its environment-count breakdown. Polls alongside the dashboard.
export default function HealthSummary() {
  const [sum, setSum] = useState<Summary | null>(null);

  useEffect(() => {
    const load = () => api.summary().then(setSum).catch(() => {});
    load();
    const t = setInterval(load, 5000);
    return () => clearInterval(t);
  }, []);

  if (!sum) return null;
  const { totals, instances } = sum;

  return (
    <div className="mb-6 rounded-xl border border-[#262e42] bg-[#131826] p-4">
      <div className="mb-3 flex flex-wrap items-baseline gap-x-5 gap-y-1 text-sm">
        <span className="font-medium">Fleet</span>
        <span className="text-[#8595b6]">
          <b className="text-[#e7ebf4]">{totals.reachable}</b>/{totals.instances} instances up
        </span>
        <span className="text-[#8595b6]">
          <b className="text-[#e7ebf4]">{totals.environments}</b> environments
        </span>
        {totals.by_status["running"] ? (
          <span className="text-[#2dd4bf]">{totals.by_status["running"]} running</span>
        ) : null}
        {totals.by_status["failed"] ? (
          <span className="text-[#f87171]">{totals.by_status["failed"]} failed</span>
        ) : null}
      </div>

      <div className="flex flex-wrap gap-2">
        {instances.map((h) => {
          const counts = Object.entries(h.by_status)
            .map(([k, v]) => `${v} ${k}`)
            .join(" · ");
          return (
            <div
              key={h.instance_id}
              title={h.reachable ? counts || "no environments" : h.error}
              className="flex items-center gap-2 rounded-lg border border-[#262e42] bg-[#0a0e16] px-3 py-1.5 text-xs"
            >
              <span
                className={`inline-block h-2 w-2 rounded-full ${h.reachable ? "bg-[#2dd4bf]" : "bg-[#f87171]"}`}
              />
              <span className="font-medium">{h.instance_label}</span>
              <span className="text-[#8595b6]">
                {h.reachable ? `${h.environments} env${h.environments === 1 ? "" : "s"}` : "unreachable"}
              </span>
            </div>
          );
        })}
        {instances.length === 0 && <span className="text-xs text-[#8595b6]">No instances registered.</span>}
      </div>
    </div>
  );
}
