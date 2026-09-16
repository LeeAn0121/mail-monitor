import { useEffect, useRef, useState } from "react";
import type { RankEntry, Snapshot, WebEvent } from "./types";

const MAX_FEED = 300;
// How often we re-poll /api/snapshot just for alertActive and the sender/
// receiver rankings — the event feed itself stays live via SSE and never
// needs this poll, so a slow interval is fine.
const RESYNC_MS = 5000;

interface FeedState {
  connected: boolean;
  events: WebEvent[];
  counts: Record<string, number>;
  senderRanking: RankEntry[];
  receiverRanking: RankEntry[];
  alertActive: boolean;
}

export interface DashboardState extends FeedState {
  refresh: () => void;
}

export function useDashboard(): DashboardState {
  const [state, setState] = useState<FeedState>({
    connected: false,
    events: [],
    counts: {},
    senderRanking: [],
    receiverRanking: [],
    alertActive: false,
  });
  const loadedSnapshot = useRef(false);
  const resyncRef = useRef<() => void>(() => {});

  useEffect(() => {
    let cancelled = false;

    async function resync(opts?: { reload?: boolean }) {
      try {
        const res = await fetch("/api/snapshot");
        const data: Snapshot = await res.json();
        if (cancelled) return;
        const reload = opts?.reload ?? false;
        setState((prev) => ({
          ...prev,
          // Only seed the feed/counts from the very first snapshot — after
          // that the SSE stream is the source of truth for new arrivals, and
          // re-adopting the snapshot's feed on every resync would clobber
          // anything the stream added since. A manual refresh() opts back in
          // once, to recover from a missed SSE reconnect.
          events: loadedSnapshot.current && !reload ? prev.events : data.events.slice().reverse(),
          counts: loadedSnapshot.current && !reload ? prev.counts : data.counts,
          senderRanking: data.senderRanking,
          receiverRanking: data.receiverRanking,
          alertActive: data.alertActive,
        }));
        loadedSnapshot.current = true;
      } catch {
        // server not reachable yet — resync interval will retry
      }
    }

    resyncRef.current = () => resync({ reload: true });
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

  return { ...state, refresh: () => resyncRef.current() };
}
