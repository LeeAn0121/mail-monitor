import { useCallback, useEffect, useState } from "react";
import type { ForwardingEntry } from "./types";

export function useForwardings() {
  const [forwardings, setForwardings] = useState<ForwardingEntry[]>([]);
  const [enabled, setEnabled] = useState(true);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetch("/api/forwardings");
      const data = await res.json();
      if (data.error) {
        setError(data.error);
      } else {
        setError(null);
        setForwardings(data.forwardings ?? []);
        setEnabled(data.enabled ?? true);
      }
    } catch {
      setError("포워딩 목록을 불러오지 못했습니다. 서버 연결을 확인하세요.");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  async function add(source: string, destination: string): Promise<string | null> {
    setPending(true);
    try {
      const res = await fetch("/api/forwardings", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ source, destination }),
      });
      const data = await res.json();
      if (data.error) return data.error as string;
      await refresh();
      return null;
    } catch {
      return "포워딩 추가 요청에 실패했습니다.";
    } finally {
      setPending(false);
    }
  }

  async function remove(source: string, destination: string): Promise<string | null> {
    setPending(true);
    try {
      const params = new URLSearchParams({ source, destination });
      const res = await fetch(`/api/forwardings?${params.toString()}`, { method: "DELETE" });
      const data = await res.json();
      if (data.error) return data.error as string;
      await refresh();
      return null;
    } catch {
      return "포워딩 삭제 요청에 실패했습니다.";
    } finally {
      setPending(false);
    }
  }

  return { forwardings, enabled, loading, error, pending, add, remove, refresh };
}
