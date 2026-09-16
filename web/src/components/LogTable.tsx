import { useEffect, useMemo, useState } from "react";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Chip from "@mui/material/Chip";
import IconButton from "@mui/material/IconButton";
import InputAdornment from "@mui/material/InputAdornment";
import Menu from "@mui/material/Menu";
import MenuItem from "@mui/material/MenuItem";
import Stack from "@mui/material/Stack";
import TextField from "@mui/material/TextField";
import Tooltip from "@mui/material/Tooltip";
import Typography from "@mui/material/Typography";
import SearchIcon from "@mui/icons-material/Search";
import FileDownloadIcon from "@mui/icons-material/FileDownload";
import PrintIcon from "@mui/icons-material/Print";
import RefreshIcon from "@mui/icons-material/Refresh";
import { EVENT_TYPES } from "../types";
import type { WebEvent } from "../types";
import { eventColors, monoFont } from "../theme";
import { exportCSV, exportXLSX } from "../export";
import { loadWithTTL, saveWithTTL } from "../persist";

// 시간/유형/발신자IP/수신자IP are fixed-width so addresses and results — the
// two columns worth the most horizontal room — get it. The whole grid has a
// min-width and scrolls horizontally rather than squeezing columns past
// legibility on narrow screens.
const GRID_COLS = "96px 66px 1.3fr 1.3fr 1.8fr 1fr 108px 108px";
const GRID_MIN_WIDTH = 980;

const LIVE_FILTER_KEY = "mm.liveFilter";

interface LiveFilter {
  query: string;
  types: string[];
}

interface Props {
  title: string;
  events: WebEvent[];
  /** Live mode filters client-side; history mode delegates to onSearch. */
  mode: "live" | "history";
  onSearch?: (query: string) => void;
  onRefresh?: () => void;
  initialQuery?: string;
  loading?: boolean;
  emptyHint: string;
  filename: string;
}

const FAILURE_TYPES = new Set(["BOUNCE", "REJECT"]);

export default function LogTable({
  title,
  events,
  mode,
  onSearch,
  onRefresh,
  initialQuery,
  loading,
  emptyHint,
  filename,
}: Props) {
  const [query, setQuery] = useState(() => {
    if (mode === "history") return initialQuery ?? "";
    return loadWithTTL<LiveFilter>(LIVE_FILTER_KEY)?.query ?? "";
  });
  const [activeTypes, setActiveTypes] = useState<Set<string>>(() => {
    if (mode !== "live") return new Set(EVENT_TYPES);
    const saved = loadWithTTL<LiveFilter>(LIVE_FILTER_KEY)?.types;
    return saved && saved.length > 0 ? new Set(saved) : new Set(EVENT_TYPES);
  });
  const [exportAnchor, setExportAnchor] = useState<HTMLElement | null>(null);

  useEffect(() => {
    if (mode !== "live") return;
    saveWithTTL<LiveFilter>(LIVE_FILTER_KEY, { query, types: Array.from(activeTypes) });
  }, [mode, query, activeTypes]);

  const filtered = useMemo(() => {
    if (mode === "history") return events;
    const q = query.trim().toLowerCase();
    return events.filter((e) => {
      if (!activeTypes.has(e.type)) return false;
      if (!q) return true;
      return (
        e.subject.toLowerCase().includes(q) ||
        e.from.toLowerCase().includes(q) ||
        e.to.toLowerCase().includes(q) ||
        e.result.toLowerCase().includes(q)
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
    <Box sx={{ border: 1, borderColor: "divider", display: "flex", flexDirection: "column", minHeight: 0, width: "100%" }}>
      <Stack
        direction={{ xs: "column", md: "row" }}
        spacing={1.5}
        alignItems={{ md: "center" }}
        justifyContent="space-between"
        sx={{ px: 2, py: 1.5, borderBottom: 1, borderColor: "divider" }}
      >
        <Stack direction="row" spacing={1} alignItems="baseline" flexShrink={0}>
          <Typography variant="subtitle2" sx={{ fontWeight: 700 }}>
            {title}
          </Typography>
          <Typography variant="caption" sx={{ color: "text.disabled", fontFamily: monoFont }}>
            {filtered.length.toLocaleString()}건
          </Typography>
        </Stack>

        <Stack direction="row" spacing={1.5} alignItems="center" flexWrap="wrap" useFlexGap>
          <Box component="form" onSubmit={submitSearch}>
            <TextField
              size="small"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder={mode === "history" ? "user@domain.com / IP / 제목 키워드" : "발신·수신·제목·처리결과 검색"}
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

          {onRefresh && (
            <Tooltip title="새로고침">
              <IconButton size="small" onClick={onRefresh}>
                <RefreshIcon fontSize="small" />
              </IconButton>
            </Tooltip>
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

      <Box className="log-scroll thin-scroll" sx={{ overflow: "auto", flex: 1, minHeight: 0 }}>
        <Box sx={{ minWidth: GRID_MIN_WIDTH }}>
          <Box
            sx={{
              display: "grid",
              gridTemplateColumns: GRID_COLS,
              px: 2,
              py: 0.75,
              borderBottom: 1,
              borderColor: "divider",
              color: "text.secondary",
              position: "sticky",
              top: 0,
              bgcolor: "background.paper",
              zIndex: 1,
            }}
          >
            {["시간", "유형", "발신", "수신", "내용(제목)", "처리결과", "발신자IP", "수신자IP"].map((h) => (
              <Typography key={h} variant="caption" sx={{ fontWeight: 600 }}>
                {h}
              </Typography>
            ))}
          </Box>

          {filtered.length === 0 ? (
            <Typography variant="body2" color="text.disabled" sx={{ p: 3, textAlign: "center" }}>
              {emptyHint}
            </Typography>
          ) : (
            filtered.map((ev, i) => {
              const isFailure = FAILURE_TYPES.has(ev.type);
              return (
                <Box
                  key={i}
                  sx={{
                    display: "grid",
                    gridTemplateColumns: GRID_COLS,
                    px: 2,
                    py: 0.875,
                    borderBottom: 1,
                    borderColor: "divider",
                    alignItems: "center",
                    bgcolor: i % 2 === 1 ? "action.hover" : "transparent",
                    transition: "background-color .12s",
                    "&:hover": { bgcolor: "action.selected" },
                  }}
                >
                  <Typography variant="body2" sx={{ fontFamily: monoFont, color: "text.secondary", fontSize: 12.5 }}>
                    {ev.when}
                  </Typography>
                  <Typography
                    variant="body2"
                    sx={{
                      fontFamily: monoFont,
                      fontWeight: 700,
                      fontSize: 12.5,
                      color: eventColors[ev.type] ?? "text.primary",
                    }}
                  >
                    {ev.glyph} {ev.type}
                  </Typography>
                  <Typography variant="body2" sx={{ fontFamily: monoFont, fontSize: 12.5, wordBreak: "break-all" }}>
                    {ev.from}
                  </Typography>
                  <Typography variant="body2" sx={{ fontFamily: monoFont, fontSize: 12.5, wordBreak: "break-all" }}>
                    {ev.to}
                  </Typography>
                  <Typography
                    variant="body2"
                    sx={{ fontSize: 13, color: ev.subject ? "text.primary" : "text.disabled" }}
                    noWrap
                    title={ev.subject}
                  >
                    {ev.subject || "(제목 없음)"}
                  </Typography>
                  <Typography
                    variant="body2"
                    noWrap
                    title={ev.result}
                    sx={{
                      fontSize: 12.5,
                      fontWeight: isFailure ? 700 : 400,
                      color: isFailure ? eventColors[ev.type] : "text.secondary",
                    }}
                  >
                    {ev.result}
                  </Typography>
                  <Typography variant="body2" sx={{ fontFamily: monoFont, fontSize: 12, color: "text.secondary" }}>
                    {ev.fromIp || "—"}
                  </Typography>
                  <Typography variant="body2" sx={{ fontFamily: monoFont, fontSize: 12, color: "text.secondary" }}>
                    {ev.toIp || "—"}
                  </Typography>
                </Box>
              );
            })
          )}
        </Box>
      </Box>
    </Box>
  );
}
