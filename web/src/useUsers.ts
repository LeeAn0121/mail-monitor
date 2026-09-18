import { useCallback, useEffect, useState } from "react";
import type { DirectoryUser } from "./types";

export function useUsers() {
  const [users, setUsers] = useState<DirectoryUser[]>([]);
  const [enabled, setEnabled] = useState(true);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetch("/api/users");
      const data = await res.json();
      if (data.error) {
        setError(data.error);
      } else {
        setError(null);
        setUsers(data.users ?? []);
        setEnabled(data.enabled ?? true);
      }
    } catch {
      setError("사용자 목록을 불러오지 못했습니다. 서버 연결을 확인하세요.");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return { users, enabled, loading, error, refresh };
}
