import * as XLSX from "xlsx";
import type { WebEvent } from "./types";

const COLUMNS = [
  "시간",
  "유형",
  "발신",
  "수신",
  "원본수신(별칭)",
  "내용",
  "처리결과",
  "발신자IP",
  "수신자IP",
] as const;

function toRows(events: WebEvent[]): string[][] {
  return events.map((e) => [e.when, e.type, e.from, e.to, e.origTo, e.subject, e.result, e.fromIp, e.toIp]);
}

function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

export function exportCSV(events: WebEvent[], filename = "mail-monitor.csv") {
  const rows = [COLUMNS as unknown as string[], ...toRows(events)];
  const csv = rows
    .map((row) => row.map((cell) => `"${cell.replace(/"/g, '""')}"`).join(","))
    .join("\r\n");
  // BOM so Excel on Windows opens UTF-8 Korean text correctly instead of
  // mangling it as the system codepage.
  downloadBlob(new Blob(["﻿" + csv], { type: "text/csv;charset=utf-8" }), filename);
}

export function exportXLSX(events: WebEvent[], filename = "mail-monitor.xlsx") {
  const sheet = XLSX.utils.aoa_to_sheet([COLUMNS as unknown as string[], ...toRows(events)]);
  sheet["!cols"] = [
    { wch: 18 },
    { wch: 8 },
    { wch: 26 },
    { wch: 26 },
    { wch: 26 },
    { wch: 40 },
    { wch: 16 },
    { wch: 16 },
    { wch: 16 },
  ];
  const book = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(book, sheet, "mail-monitor");
  XLSX.writeFile(book, filename);
}
