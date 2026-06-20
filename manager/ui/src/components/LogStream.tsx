import { useEffect, useRef, useState } from "react";
import { instancePath } from "../lib/api";
import { streamSSE } from "../lib/sse";
import { statusBadge } from "../lib/format";

// LogStream live-tails one deployment's log over SSE, streamed through the
// manager's per-instance pass-through (replays stored output, then tails until
// edp's terminal "done" event). Reusable inline (env detail) or inside a drawer.
export default function LogStream({
  instanceID,
  deploymentID,
  initialStatus = "running",
  heightClass = "h-80",
  onDone,
}: {
  instanceID: number;
  deploymentID: number;
  initialStatus?: string;
  heightClass?: string;
  onDone?: (status: string) => void;
}) {
  const [text, setText] = useState("");
  const [status, setStatus] = useState(initialStatus);
  const preRef = useRef<HTMLPreElement>(null);

  useEffect(() => {
    setText("");
    setStatus(initialStatus);
    const path = instancePath(instanceID, `/api/deployments/${deploymentID}/logs/stream`);
    const stop = streamSSE(path, {
      onMessage: (d) => setText((t) => t + d + "\n"),
      onEvent: (event, data) => {
        if (event === "done") {
          setStatus(data);
          onDone?.(data);
        }
      },
      onError: () => setStatus("stream error"),
    });
    return stop;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [instanceID, deploymentID]);

  useEffect(() => {
    const el = preRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [text]);

  return (
    <div className="flex min-h-0 flex-col">
      <div className="mb-2 flex items-center gap-2">
        <span className="text-xs uppercase tracking-wide text-[#8595b6]">Log</span>
        <span className={statusBadge(status)}>{status}</span>
        <span className="text-xs text-[#54607e]">deploy #{deploymentID}</span>
      </div>
      <pre
        ref={preRef}
        className={`m-0 ${heightClass} overflow-auto whitespace-pre-wrap break-words rounded-lg border border-[#1c2336] bg-[#05070c] p-3 font-mono text-[12.5px] leading-relaxed text-[#c7d0e4]`}
      >
        {text || "Waiting for output…"}
      </pre>
    </div>
  );
}
