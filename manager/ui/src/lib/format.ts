// durMs renders a millisecond duration compactly: "840ms", "8.4s", "1m 5s".
export function durMs(ms: number): string {
  if (!ms || ms <= 0) return "";
  if (ms < 1000) return `${ms}ms`;
  const s = ms / 1000;
  if (s < 60) return `${s.toFixed(1)}s`;
  return `${Math.floor(s / 60)}m ${Math.floor(s) % 60}s`;
}

// fmtDate renders an ISO timestamp like "Jan 02 15:04".
export function fmtDate(iso: string | null | undefined): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (isNaN(d.getTime())) return "";
  return new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).format(d);
}

// relTime renders a short relative time like "3m ago", "2h ago", "just now".
export function relTime(iso: string | null | undefined): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (isNaN(d.getTime())) return "";
  const secs = Math.round((Date.now() - d.getTime()) / 1000);
  if (secs < 45) return "just now";
  const m = Math.round(secs / 60);
  if (m < 60) return `${m}m ago`;
  const h = Math.round(m / 60);
  if (h < 24) return `${h}h ago`;
  return `${Math.round(h / 24)}d ago`;
}

// statusBadge returns pill classes for a deploy/run/env status.
export function statusBadge(status: string): string {
  const base = "rounded px-1.5 py-0.5 text-[11px] font-medium ";
  switch (status) {
    case "success":
      return base + "bg-[#0f2e29] text-[#2dd4bf]";
    case "failed":
      return base + "bg-[#2e1418] text-[#f87171]";
    case "running":
    case "queued":
      return base + "bg-[#2e2613] text-[#f5b544]";
    default:
      return base + "bg-[#1b2236] text-[#8595b6]";
  }
}

// sourceSummary describes where an env comes from, for a compact row.
export function sourceSummary(e: {
  source_type: string;
  registry_image: string;
  repo_url: string;
  git_ref: string;
}): string {
  if (e.source_type === "registry") return e.registry_image || "registry";
  if (e.source_type === "dockerfile") return "inline Dockerfile";
  const repo = e.repo_url.replace(/^https?:\/\//, "").replace(/\.git$/, "");
  return e.git_ref ? `${repo}#${e.git_ref}` : repo || "git";
}
