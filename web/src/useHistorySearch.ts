import { useEffect, useState } from "react";
import type { HistoryResponse, WebEvent } from "./types";
import { loadWithTTL, saveWithTTL } from "./persist";

const HISTORY_QUERY_KEY = "mm.historyQuery";
const HISTORY_RANGE_KEY = "mm.historyRange";

export interface DateRange {
  from: string | null; // ISO 8601, inclusive
  to: string | null; // ISO 8601, inclusive
}

const EMPTY_RANGE: DateRange = { from: null, to: null };

export function useHistorySearch() {
  const [results, setResults] = useState<WebEvent[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [searchedFor, setSearchedFor] = useState<string | null>(null);
  const [appliedRange, setAppliedRange] = useState<DateRange>(EMPTY_RANGE);
  const [lastQuery] = useState(() => loadWithTTL<string>(HISTORY_QUERY_KEY));
  const [lastRange] = useState(() => loadWithTTL<DateRange>(HISTORY_RANGE_KEY) ?? EMPTY_RANGE);

  async function search(query: string, range: DateRange = EMPTY_RANGE) {
    saveWithTTL(HISTORY_QUERY_KEY, query);
    saveWithTTL(HISTORY_RANGE_KEY, range);
    setAppliedRange(range);
    setLoading(true);
    setError(null);
    try {
      const params = new URLSearchParams({ q: query });
      if (range.from) params.set("from", range.from);
      if (range.to) params.set("to", range.to);
      const res = await fetch(`/api/history?${params.toString()}`);
      const data: HistoryResponse = await res.json();
      if (data.error) {
        setError(data.error);
        setResults([]);
      } else {
        setResults(data.events ?? []);
      }
    } catch {
      setError("검색 요청에 실패했습니다. 서버 연결을 확인하세요.");
      setResults([]);
    } finally {
      setSearchedFor(query);
      setLoading(false);
    }
  }

  // Restore the last search (within 24h) once on mount, so reopening the
  // dashboard doesn't lose what you were looking for.
  useEffect(() => {
    if (lastQuery !== null) search(lastQuery, lastRange);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return {
    results,
    loading,
    error,
    searchedFor,
    search,
    initialQuery: lastQuery ?? "",
    initialRange: lastRange,
    appliedRange,
  };
}
