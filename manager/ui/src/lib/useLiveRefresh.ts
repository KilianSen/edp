import { useEffect } from "react";
import { api } from "./api";
import { streamSSE } from "./sse";

// useLiveRefresh opens an SSE events stream to every registered instance (via the
// manager pass-through to edp's /api/events/stream) and invokes onChange,
// debounced, whenever any instance reports a container change — so the dashboard
// updates promptly instead of only on its poll interval. Polling stays as a
// fallback for instances that are briefly unreachable.
export function useLiveRefresh(onChange: () => void) {
  useEffect(() => {
    let stops: Array<() => void> = [];
    let timer: ReturnType<typeof setTimeout> | null = null;
    let cancelled = false;

    const ping = () => {
      if (timer) return;
      timer = setTimeout(() => {
        timer = null;
        onChange();
      }, 800);
    };

    api
      .listInstances()
      .then((list) => {
        if (cancelled) return;
        stops = (list ?? []).map((i) =>
          streamSSE(`/api/instances/${i.id}/edp/api/events/stream`, { onMessage: ping, onError: () => {} }),
        );
      })
      .catch(() => {});

    return () => {
      cancelled = true;
      if (timer) clearTimeout(timer);
      stops.forEach((s) => s());
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
}
