import { useEffect, useRef, useState } from "react";
import type { RankEntry, Snapshot, WebEvent } from "./types";

const MAX_FEED = 300;
// How often we re-poll /api/snapshot just for alertActive and the sender/
// receiver rankings — the event feed itself stays live via SSE and never
// needs this poll, so a slow interval is fine.
const RESYNC_MS = 5000;

interface FeedState {
  connected: boolean;
  loaded: boolean;
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
    loaded: false,
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
    let retryTimer: ReturnType<typeof setTimeout> | undefined;

    async function resync(opts?: { reload?: boolean }) {
      try {
        const res = await fetch("/api/snapshot");
        const data: Snapshot = await res.json();
        if (cancelled) return;
        const reload = opts?.reload ?? false;
        setState((prev) => ({
          ...prev,
          loaded: true,
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
        // Page loaded before the server was reachable (e.g. right after a
        // deploy restart), or a transient network blip — retry quickly
        // instead of leaving the dashboard looking empty until the next
        // slow periodic resync (or a manual refresh click) happens to work.
        if (!cancelled && !loadedSnapshot.current) {
          retryTimer = setTimeout(() => resync(opts), 1000);
        }
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
      clearTimeout(retryTimer);
      es.close();
    };
  }, []);

  return { ...state, refresh: () => resyncRef.current() };
}
