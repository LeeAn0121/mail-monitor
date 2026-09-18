import { useCallback, useEffect, useState } from "react";
import type { DirectoryUser } from "./types";

export function useUsers() {
  const [users, setUsers] = useState<DirectoryUser[]>([]);
  const [enabled, setEnabled] = useState(true);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

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

  async function create(email: string, password: string, name: string): Promise<string | null> {
    setPending(true);
    try {
      const res = await fetch("/api/users", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, password, name }),
      });
      const data = await res.json();
      if (data.error) return data.error as string;
      if (data.alreadyExists) return "이미 등록된 이메일입니다.";
      await refresh();
      return null;
    } catch {
      return "계정 등록 요청에 실패했습니다.";
    } finally {
      setPending(false);
    }
  }

  async function update(email: string, password: string, name: string): Promise<string | null> {
    setPending(true);
    try {
      const res = await fetch("/api/users", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, password, name }),
      });
      const data = await res.json();
      if (data.error) return data.error as string;
      await refresh();
      return null;
    } catch {
      return "계정 수정 요청에 실패했습니다.";
    } finally {
      setPending(false);
    }
  }

  async function remove(email: string): Promise<string | null> {
    setPending(true);
    try {
      const res = await fetch(`/api/users?email=${encodeURIComponent(email)}`, { method: "DELETE" });
      const data = await res.json();
      if (data.error) return data.error as string;
      await refresh();
      return null;
    } catch {
      return "계정 삭제 요청에 실패했습니다.";
    } finally {
      setPending(false);
    }
  }

  return { users, enabled, loading, error, pending, create, update, remove, refresh };
}
