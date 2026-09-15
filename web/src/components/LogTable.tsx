import { useMemo, useState } from "react";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Chip from "@mui/material/Chip";
import InputAdornment from "@mui/material/InputAdornment";
import Menu from "@mui/material/Menu";
import MenuItem from "@mui/material/MenuItem";
import Stack from "@mui/material/Stack";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";
import SearchIcon from "@mui/icons-material/Search";
import FileDownloadIcon from "@mui/icons-material/FileDownload";
import PrintIcon from "@mui/icons-material/Print";
import { EVENT_TYPES } from "../types";
import type { WebEvent } from "../types";
import { eventColors, monoFont } from "../theme";
import { exportCSV, exportXLSX } from "../export";

const GRID_COLS = "104px 84px 1fr 1fr 2.4fr";

interface Props {
  title: string;
  events: WebEvent[];
  /** Live mode filters client-side; history mode delegates to onSearch. */
  mode: "live" | "history";
  onSearch?: (query: string) => void;
  loading?: boolean;
  emptyHint: string;
  filename: string;
}

export default function LogTable({ title, events, mode, onSearch, loading, emptyHint, filename }: Props) {
  const [query, setQuery] = useState("");
  const [activeTypes, setActiveTypes] = useState<Set<string>>(new Set(EVENT_TYPES));
  const [exportAnchor, setExportAnchor] = useState<HTMLElement | null>(null);

  const filtered = useMemo(() => {
    if (mode === "history") return events;
    const q = query.trim().toLowerCase();
    return events.filter((e) => {
      if (!activeTypes.has(e.type)) return false;
      if (!q) return true;
      return (
        e.text.toLowerCase().includes(q) ||
        e.from.toLowerCase().includes(q) ||
        e.to.toLowerCase().includes(q)
      );
    });
  }, [events, query, activeTypes, mode]);

  function toggleType(type: string) {
    setActiveTypes((prev) => {
      const next = new Set(prev);
      if (next.has(type)) next.delete(type);
      else next.add(type);
      return next;
    });
  }

  function submitSearch(e: React.FormEvent) {
    e.preventDefault();
    onSearch?.(query.trim());
  }

  return (
    <Box sx={{ border: 1, borderColor: "divider", display: "flex", flexDirection: "column", minHeight: 0 }}>
      <Stack
        direction={{ xs: "column", md: "row" }}
        spacing={1.5}
        alignItems={{ md: "center" }}
        justifyContent="space-between"
        sx={{ px: 2, py: 1.5, borderBottom: 1, borderColor: "divider" }}
      >
        <Typography variant="subtitle2" sx={{ fontWeight: 700, flexShrink: 0 }}>
          {title}
        </Typography>

        <Stack direction="row" spacing={1.5} alignItems="center" flexWrap="wrap" useFlexGap>
          <Box component="form" onSubmit={submitSearch}>
            <TextField
              size="small"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder={mode === "history" ? "user@domain.com / IP / 제목 키워드" : "발신·수신·내용 검색"}
              sx={{ width: 260 }}
              InputProps={{
                startAdornment: (
                  <InputAdornment position="start">
                    <SearchIcon fontSize="small" />
                  </InputAdornment>
                ),
              }}
            />
          </Box>

          {mode === "history" && (
            <Button type="submit" onClick={submitSearch} size="small" variant="outlined" disabled={loading}>
              {loading ? "검색 중..." : "검색"}
            </Button>
          )}

          {mode === "live" && (
            <Stack direction="row" spacing={0.5} flexWrap="wrap" useFlexGap>
              {EVENT_TYPES.map((type) => {
                const on = activeTypes.has(type);
                return (
                  <Chip
                    key={type}
                    label={type}
                    size="small"
                    onClick={() => toggleType(type)}
                    sx={{
                      fontWeight: 600,
                      color: on ? eventColors[type] : "text.disabled",
                      border: 1,
                      borderColor: on ? eventColors[type] : "divider",
                      bgcolor: "transparent",
                    }}
                  />
                );
              })}
            </Stack>
          )}

          <Button
            size="small"
            startIcon={<FileDownloadIcon fontSize="small" />}
            onClick={(e) => setExportAnchor(e.currentTarget)}
            disabled={filtered.length === 0}
          >
            내보내기
          </Button>
          <Menu anchorEl={exportAnchor} open={!!exportAnchor} onClose={() => setExportAnchor(null)}>
            <MenuItem
              onClick={() => {
                exportCSV(filtered, `${filename}.csv`);
                setExportAnchor(null);
              }}
            >
              CSV로 저장
            </MenuItem>
            <MenuItem
              onClick={() => {
                exportXLSX(filtered, `${filename}.xlsx`);
                setExportAnchor(null);
              }}
            >
              Excel(XLSX)로 저장
            </MenuItem>
            <MenuItem
              onClick={() => {
                setExportAnchor(null);
                window.print();
              }}
            >
              <PrintIcon fontSize="small" style={{ marginRight: 8 }} />
              인쇄
            </MenuItem>
          </Menu>
        </Stack>
      </Stack>

      <Box
        sx={{
          display: "grid",
          gridTemplateColumns: GRID_COLS,
          px: 2,
          py: 0.75,
          borderBottom: 1,
          borderColor: "divider",
          color: "text.secondary",
        }}
      >
        {["시간", "유형", "발신", "수신", "내용"].map((h) => (
          <Typography key={h} variant="caption" sx={{ fontWeight: 600 }}>
            {h}
          </Typography>
        ))}
      </Box>

      <Box sx={{ overflowY: "auto", flex: 1, minHeight: 0 }} className="log-scroll">
        {filtered.length === 0 ? (
          <Typography variant="body2" color="text.disabled" sx={{ p: 3, textAlign: "center" }}>
            {emptyHint}
          </Typography>
        ) : (
          filtered.map((ev, i) => (
            <Box
              key={i}
              sx={{
                display: "grid",
                gridTemplateColumns: GRID_COLS,
                px: 2,
                py: 0.75,
                borderBottom: 1,
                borderColor: "divider",
                alignItems: "start",
                "&:hover": { bgcolor: "action.hover" },
              }}
            >
              <Typography variant="body2" sx={{ fontFamily: monoFont, color: "text.secondary", fontSize: 12.5 }}>
                {ev.when}
              </Typography>
              <Typography
                variant="body2"
                sx={{ fontFamily: monoFont, fontWeight: 600, fontSize: 12.5, color: eventColors[ev.type] ?? "text.primary" }}
              >
                {ev.glyph} {ev.type}
              </Typography>
              <Typography variant="body2" sx={{ fontFamily: monoFont, fontSize: 12.5, wordBreak: "break-all" }}>
                {ev.from}
              </Typography>
              <Typography variant="body2" sx={{ fontFamily: monoFont, fontSize: 12.5, wordBreak: "break-all" }}>
                {ev.to}
              </Typography>
              <Typography variant="body2" sx={{ fontSize: 13 }}>
                {ev.text}
              </Typography>
            </Box>
          ))
        )}
      </Box>
    </Box>
  );
}
