import { useState } from "react";

const STORAGE_KEY = "mm.refreshIntervalMs";

export const REFRESH_INTERVAL_OPTIONS = [
  { label: "10초", ms: 10_000 },
  { label: "1분", ms: 60_000 },
  { label: "3분", ms: 3 * 60_000 },
  { label: "5분", ms: 5 * 60_000 },
  { label: "15분", ms: 15 * 60_000 },
  { label: "30분", ms: 30 * 60_000 },
] as const;

const DEFAULT_MS = REFRESH_INTERVAL_OPTIONS[0].ms;

function loadStoredMs(): number {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    const ms = raw ? Number(raw) : NaN;
    if (REFRESH_INTERVAL_OPTIONS.some((o) => o.ms === ms)) return ms;
  } catch {
    // ignore — private mode / blocked storage falls through to the default
  }
  return DEFAULT_MS;
}

// A plain preference, not a 24h-TTL search filter (see persist.ts) — this
// should keep applying indefinitely until the user picks something else.
export function useRefreshInterval() {
  const [intervalMs, setIntervalMsState] = useState<number>(loadStoredMs);

  function setIntervalMs(ms: number) {
    setIntervalMsState(ms);
    try {
      localStorage.setItem(STORAGE_KEY, String(ms));
    } catch {
      // ignore
    }
  }

  return { intervalMs, setIntervalMs };
}
