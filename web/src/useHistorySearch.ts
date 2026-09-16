import { useEffect, useState } from "react";
import type { HistoryResponse, WebEvent } from "./types";
import { loadWithTTL, saveWithTTL } from "./persist";

const HISTORY_QUERY_KEY = "mm.historyQuery";

export function useHistorySearch() {
  const [results, setResults] = useState<WebEvent[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [searchedFor, setSearchedFor] = useState<string | null>(null);
  const [lastQuery] = useState(() => loadWithTTL<string>(HISTORY_QUERY_KEY));

  async function search(query: string) {
    saveWithTTL(HISTORY_QUERY_KEY, query);
    setLoading(true);
    setError(null);
    try {
      const res = await fetch(`/api/history?q=${encodeURIComponent(query)}`);
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
    if (lastQuery !== null) search(lastQuery);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return { results, loading, error, searchedFor, search, initialQuery: lastQuery ?? "" };
}
