import { useEffect, useRef, useState } from "react";
import { streamSSE } from "../lib/sse";
import { badgeClass } from "../lib/format";

// LogStream live-tails a deployment or hook-run log over SSE (replays stored
// output first, then streams). On the terminal "done" event it shows the final
// status badge and notifies the parent so it can refresh. Ported from edp.js
// initLog. `key`ing this component on the stream path resets it cleanly.
export function LogStream({
  streamPath,
  status,
  onDone,
}: {
  streamPath: string;
  status: string;
  onDone?: (status: string) => void;
}) {
  const [text, setText] = useState("");
  const [finalStatus, setFinalStatus] = useState(status);
  const preRef = useRef<HTMLPreElement>(null);

  useEffect(() => {
    setText("");
    setFinalStatus(status);
    const stop = streamSSE(streamPath, {
      onMessage: (data) => setText((t) => t + data + "\n"),
      onEvent: (event, data) => {
        if (event === "done") {
          setFinalStatus(data);
          onDone?.(data);
        }
      },
    });
    return stop;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [streamPath]);

  useEffect(() => {
    const el = preRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [text]);

  return (
    <>
      <div className="flex items-center gap-2">
        <p className="eyebrow">Log</p>
        <span className={badgeClass(finalStatus)}>{finalStatus}</span>
      </div>
      <pre
        ref={preRef}
        className="mt-3 h-80 grow overflow-auto whitespace-pre-wrap break-words rounded-lg border border-line bg-[#05070c] p-3 font-mono text-[12.5px] leading-relaxed"
      >
        {text}
      </pre>
    </>
  );
}
