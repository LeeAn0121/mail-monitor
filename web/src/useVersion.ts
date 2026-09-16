import { useEffect, useState } from "react";
import type { VersionInfo } from "./types";

export function useVersion(): VersionInfo | null {
  const [info, setInfo] = useState<VersionInfo | null>(null);

  useEffect(() => {
    fetch("/api/version")
      .then((res) => res.json())
      .then(setInfo)
      .catch(() => {});
  }, []);

  return info;
}
