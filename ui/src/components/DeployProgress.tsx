import { useEffect, useState } from "react";

// DeployProgress animates the deploy ETA bar, ported from edp.js tickProgress:
// fill grows toward `eta` (capped at 92%), text counts down remaining seconds.
// With no eta it parks at 35% and just reads "Deploying…".
export function DeployProgress({ startMs, etaMs }: { startMs: number; etaMs: number }) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), 500);
    return () => clearInterval(t);
  }, []);

  const elapsed = now - startMs;
  let width = "35%";
  let text = "Deploying…";
  if (etaMs > 0) {
    width = Math.min(92, (elapsed / etaMs) * 100) + "%";
    text = `Deploying… ~${Math.max(0, Math.round((etaMs - elapsed) / 1000))}s left`;
  }

  return (
    <div>
      <div className="progress-track">
        <div className="progress-fill" style={{ width }}>
          <span className="sheen" />
        </div>
      </div>
      <p className="mt-1.5 font-mono text-xs text-work">{text}</p>
    </div>
  );
}
