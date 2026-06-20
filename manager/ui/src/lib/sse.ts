import { getToken } from "./api";
import { url } from "./config";

interface StreamHandlers {
  onMessage?: (data: string) => void;
  onEvent?: (event: string, data: string) => void;
  onError?: (err: unknown) => void;
}

// streamSSE consumes a text/event-stream endpoint using fetch + a ReadableStream
// reader instead of EventSource, so it can send the Authorization header (which
// EventSource cannot). Here it streams through the manager's per-instance
// pass-through, which proxies the edp SSE log endpoint with FlushInterval -1.
//
// Returns a function that aborts the stream. Default events arrive via onMessage;
// named events (edp's terminal "done") arrive via onEvent(name, data).
export function streamSSE(path: string, handlers: StreamHandlers): () => void {
  const ctrl = new AbortController();
  const token = getToken();

  (async () => {
    try {
      const res = await fetch(url(path), {
        headers: token ? { Authorization: "Bearer " + token } : {},
        signal: ctrl.signal,
      });
      if (!res.ok || !res.body) throw new Error("stream failed: " + res.status);

      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let buf = "";

      let eventName = "";
      let dataLines: string[] = [];
      const dispatch = () => {
        if (dataLines.length === 0 && !eventName) return;
        const data = dataLines.join("\n");
        if (eventName) handlers.onEvent?.(eventName, data);
        else handlers.onMessage?.(data);
        eventName = "";
        dataLines = [];
      };

      for (;;) {
        const { value, done } = await reader.read();
        if (done) break;
        buf += decoder.decode(value, { stream: true });
        const lines = buf.split("\n");
        buf = lines.pop() ?? ""; // keep the trailing partial line
        for (const raw of lines) {
          const line = raw.replace(/\r$/, "");
          if (line === "") {
            dispatch();
          } else if (line.startsWith(":")) {
            // comment / keepalive — ignore
          } else if (line.startsWith("event:")) {
            eventName = line.slice(6).trim();
          } else if (line.startsWith("data:")) {
            dataLines.push(line.slice(5).replace(/^ /, ""));
          }
        }
      }
      dispatch();
    } catch (err) {
      if (!ctrl.signal.aborted) handlers.onError?.(err);
    }
  })();

  return () => ctrl.abort();
}
