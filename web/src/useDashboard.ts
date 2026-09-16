import { useEffect, useRef, useState } from "react";
import type { RankEntry, Snapshot, WebEvent } from "./types";

const MAX_FEED = 300;

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

// intervalMs controls how often we re-poll /api/snapshot for alertActive
// and the sender/receiver rankings — the event feed itself stays live via
// SSE regardless and never depends on this poll, so a slow interval is
// fine; it's user-configurable (see RefreshIntervalPicker) precisely
// because there's no freshness requirement forcing a particular value.
export function useDashboard(intervalMs: number): DashboardState {
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
  const resyncRef = useRef<(opts?: { reload?: boolean }) => void>(() => {});

  // Mount-once: initial snapshot fetch (with quick-retry until it succeeds)
  // and the SSE connection. Split from the interval effect below so
  // changing intervalMs doesn't tear down and reconnect the SSE stream.
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

    resyncRef.current = resync;
    resync();

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
      clearTimeout(retryTimer);
      es.close();
    };
  }, []);

  // Separate effect so changing the user-configurable interval just resets
  // this timer, without re-running the mount-once setup above.
  useEffect(() => {
    const resyncTimer = setInterval(() => resyncRef.current(), intervalMs);
    return () => clearInterval(resyncTimer);
  }, [intervalMs]);

  return { ...state, refresh: () => resyncRef.current({ reload: true }) };
}
