// Small localStorage wrapper with a TTL — used to remember the live filter
// and last history search across page reloads for a day, without needing a
// backend session. Wrapped in try/catch throughout: some browsers throw on
// storage access (private mode, blocked site data), and a dashboard that
// can't remember your last search should still work fine without it.
const TTL_MS = 24 * 60 * 60 * 1000;

export function saveWithTTL<T>(key: string, value: T) {
  try {
    localStorage.setItem(key, JSON.stringify({ value, savedAt: Date.now() }));
  } catch {
    // ignore — persistence is a convenience, not a requirement
  }
}

export function loadWithTTL<T>(key: string): T | null {
  try {
    const raw = localStorage.getItem(key);
    if (!raw) return null;
    const { value, savedAt } = JSON.parse(raw) as { value: T; savedAt: number };
    if (Date.now() - savedAt > TTL_MS) {
      localStorage.removeItem(key);
      return null;
    }
    return value;
  } catch {
    return null;
  }
}
