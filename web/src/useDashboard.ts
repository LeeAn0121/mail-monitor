import { useEffect, useRef, useState } from "react";
import type { RankEntry, Snapshot, WebEvent } from "./types";

const MAX_FEED = 300;
// How often we re-poll /api/snapshot just for alertActive and the sender/
// receiver rankings — the event feed itself stays live via SSE and never
// needs this poll, so a slow interval is fine.
const RESYNC_MS = 5000;

export interface DashboardState {
  connected: boolean;
  events: WebEvent[];
  counts: Record<string, number>;
  senderRanking: RankEntry[];
  receiverRanking: RankEntry[];
  alertActive: boolean;
}

export function useDashboard(): DashboardState {
  const [state, setState] = useState<DashboardState>({
    connected: false,
    events: [],
    counts: {},
    senderRanking: [],
    receiverRanking: [],
    alertActive: false,
  });
  const loadedSnapshot = useRef(false);

  useEffect(() => {
    let cancelled = false;

    async function resync() {
      try {
        const res = await fetch("/api/snapshot");
        const data: Snapshot = await res.json();
        if (cancelled) return;
        setState((prev) => ({
          ...prev,
          // Only seed the feed/counts from the very first snapshot — after
          // that the SSE stream is the source of truth for new arrivals, and
          // re-adopting the snapshot's feed on every resync would clobber
          // anything the stream added since.
          events: loadedSnapshot.current ? prev.events : data.events.slice().reverse(),
          counts: loadedSnapshot.current ? prev.counts : data.counts,
          senderRanking: data.senderRanking,
          receiverRanking: data.receiverRanking,
          alertActive: data.alertActive,
        }));
        loadedSnapshot.current = true;
      } catch {
        // server not reachable yet — resync interval will retry
      }
    }

    resync();
    const resyncTimer = setInterval(resync, RESYNC_MS);

    const es = new EventSource("/api/stream");
    es.onopen = () => setState((prev) => ({ ...prev, connected: true }));
    es.onerror = () => setState((prev) => ({ ...prev, connected: false }));
    es.onmessage = (msg) => {
      const ev: WebEvent = JSON.parse(msg.data);
      setState((prev) => ({
        ...prev,
        events: [ev, ...prev.events].slice(0, MAX_FEED),
        counts: { ...prev.counts, [ev.type]: (prev.counts[ev.type] ?? 0) + 1 },
      }));
    };

    return () => {
      cancelled = true;
      clearInterval(resyncTimer);
      es.close();
    };
  }, []);

  return state;
}
