// durMs renders a millisecond duration compactly: "840ms", "8.4s", "1m 5s".
// Ported from edp's tmplFuncs.durMs.
export function durMs(ms: number): string {
  if (!ms || ms <= 0) return "";
  if (ms < 1000) return `${ms}ms`;
  const s = ms / 1000;
  if (s < 60) return `${s.toFixed(1)}s`;
  return `${Math.floor(s / 60)}m ${Math.floor(s) % 60}s`;
}

// fmtDate renders an ISO timestamp like the Go templates' "Jan 02 15:04".
export function fmtDate(iso: string | null, withSeconds = false): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (isNaN(d.getTime())) return "";
  const opts: Intl.DateTimeFormatOptions = {
    month: "short",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  };
  if (withSeconds) opts.second = "2-digit";
  return new Intl.DateTimeFormat("en-US", opts).format(d);
}

// badgeClass maps a deploy/run status to the .badge-* component class.
export function badgeClass(status: string): string {
  switch (status) {
    case "success":
      return "badge badge-success";
    case "failed":
      return "badge badge-failed";
    case "running":
    case "queued":
      return "badge badge-running";
    default:
      return "badge badge-idle";
  }
}
