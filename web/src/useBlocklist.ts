import { useCallback, useEffect, useState } from "react";
import type { BlockedSender } from "./types";

export function useBlocklist() {
  const [blocked, setBlocked] = useState<BlockedSender[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetch("/api/blocklist");
      const data = await res.json();
      if (data.error) {
        setError(data.error);
      } else {
        setError(null);
        setBlocked(data.blocked ?? []);
      }
    } catch {
      setError("목록을 불러오지 못했습니다. 서버 연결을 확인하세요.");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  async function block(email: string): Promise<string | null> {
    setPending(true);
    try {
      const res = await fetch("/api/blocklist", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email }),
      });
      const data = await res.json();
      if (data.error) return data.error as string;
      await refresh();
      return null;
    } catch {
      return "차단 요청에 실패했습니다.";
    } finally {
      setPending(false);
    }
  }

  async function unblock(email: string): Promise<string | null> {
    setPending(true);
    try {
      const res = await fetch(`/api/blocklist?email=${encodeURIComponent(email)}`, { method: "DELETE" });
      const data = await res.json();
      if (data.error) return data.error as string;
      await refresh();
      return null;
    } catch {
      return "차단 해제 요청에 실패했습니다.";
    } finally {
      setPending(false);
    }
  }

  return { blocked, loading, error, pending, block, unblock, refresh };
}
