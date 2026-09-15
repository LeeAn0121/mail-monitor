import { useState } from "react";
import type { HistoryResponse, WebEvent } from "./types";

export function useHistorySearch() {
  const [results, setResults] = useState<WebEvent[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [searchedFor, setSearchedFor] = useState<string | null>(null);

  async function search(query: string) {
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

  return { results, loading, error, searchedFor, search };
}
