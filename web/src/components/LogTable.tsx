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
import BlockIcon from "@mui/icons-material/Block";
import { DataGrid } from "@mui/x-data-grid";
import type { GridColDef, GridRowParams } from "@mui/x-data-grid";
import { DateTimePicker } from "@mui/x-date-pickers/DateTimePicker";
import dayjs from "dayjs";
import type { Dayjs } from "dayjs";
import { EVENT_TYPES } from "../types";
import type { WebEvent } from "../types";
import { eventColors, monoFont } from "../theme";
import { exportCSV, exportXLSX } from "../export";
import { loadWithTTL, saveWithTTL } from "../persist";
import type { DateRange } from "../useHistorySearch";
import EventDetailDialog from "./EventDetailDialog";

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
  onSearch?: (query: string, range: DateRange) => void;
  onRefresh?: () => void;
  onBlockSender?: (email: string) => void;
  blockedEmails?: Set<string>;
  initialQuery?: string;
  initialRange?: DateRange;
  loading?: boolean;
  emptyHint: string;
  filename: string;
}

const FAILURE_TYPES = new Set(["BOUNCE", "REJECT"]);

// Every text column wraps instead of truncating — a log viewer's whole job
// is to show what happened, and a clipped bounce reason or subject defeats
// that. Rows grow to fit (getRowHeight="auto" below) rather than clipping.
const wrapCell = { whiteSpace: "normal", lineHeight: 1.4, py: 1 } as const;

export default function LogTable({
  title,
  events,
  mode,
  onSearch,
  onRefresh,
  onBlockSender,
  blockedEmails,
  initialQuery,
  initialRange,
  loading,
  emptyHint,
  filename,
}: Props) {
  const [query, setQuery] = useState(() => {
    if (mode === "history") return initialQuery ?? "";
    return loadWithTTL<LiveFilter>(LIVE_FILTER_KEY)?.query ?? "";
  });
  const [fromDate, setFromDate] = useState<Dayjs | null>(() =>
    initialRange?.from ? dayjs(initialRange.from) : null,
  );
  const [toDate, setToDate] = useState<Dayjs | null>(() => (initialRange?.to ? dayjs(initialRange.to) : null));
  const [activeTypes, setActiveTypes] = useState<Set<string>>(() => {
    if (mode !== "live") return new Set(EVENT_TYPES);
    const saved = loadWithTTL<LiveFilter>(LIVE_FILTER_KEY)?.types;
    return saved && saved.length > 0 ? new Set(saved) : new Set(EVENT_TYPES);
  });
  const [exportAnchor, setExportAnchor] = useState<HTMLElement | null>(null);
  const [selected, setSelected] = useState<WebEvent | null>(null);

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
        e.fromDisplay.toLowerCase().includes(q) ||
        e.toDisplay.toLowerCase().includes(q) ||
        e.result.toLowerCase().includes(q) ||
        e.resultDetail.toLowerCase().includes(q)
      );
    });
  }, [events, query, activeTypes, mode]);

  const rows = useMemo(() => filtered.map((e, i) => ({ id: i, ...e })), [filtered]);

  const columns: GridColDef<(typeof rows)[number]>[] = useMemo(
    () => [
      {
        field: "when",
        headerName: "시간",
        width: 100,
        cellClassName: "mm-mono",
      },
      {
        field: "type",
        headerName: "유형",
        width: 84,
        cellClassName: "mm-mono",
        renderCell: (p) => (
          <span style={{ color: eventColors[p.value as string] ?? "inherit", fontWeight: 700 }}>
            {p.row.glyph} {p.value}
          </span>
        ),
      },
      { field: "fromDisplay", headerName: "발신", flex: 1.1, minWidth: 180, cellClassName: "mm-mono", sx: wrapCell },
      { field: "toDisplay", headerName: "수신", flex: 1.1, minWidth: 180, cellClassName: "mm-mono", sx: wrapCell },
      {
        field: "origTo",
        headerName: "원본 수신(별칭)",
        flex: 1,
        minWidth: 160,
        cellClassName: "mm-mono mm-dim",
        sx: wrapCell,
        renderCell: (p) => (p.value ? <span>{p.value}</span> : <span style={{ opacity: 0.4 }}>—</span>),
      },
      {
        field: "subject",
        headerName: "내용(제목)",
        flex: 1.6,
        minWidth: 200,
        sx: wrapCell,
        renderCell: (p) =>
          p.value ? (
            <span>{p.value}</span>
          ) : (
            <span style={{ opacity: 0.5 }}>(제목 없음)</span>
          ),
      },
      {
        field: "result",
        headerName: "처리결과",
        flex: 1.4,
        minWidth: 200,
        sx: wrapCell,
        renderCell: (p) => {
          const failure = FAILURE_TYPES.has(p.row.type);
          return (
            <span style={{ fontWeight: failure ? 700 : 400, color: failure ? eventColors[p.row.type] : "inherit" }}>
              {p.value}
            </span>
          );
        },
      },
      { field: "fromIp", headerName: "발신자IP", width: 130, cellClassName: "mm-mono mm-dim" },
      { field: "toIp", headerName: "수신자IP", width: 130, cellClassName: "mm-mono mm-dim" },
      ...(onBlockSender
        ? [
            {
              field: "__block",
              headerName: "",
              width: 48,
              sortable: false,
              filterable: false,
              disableColumnMenu: true,
              renderCell: (p: { row: (typeof rows)[number] }) => {
                const already = blockedEmails?.has(p.row.from);
                return (
                  <Tooltip title={already ? "이미 차단됨" : "발신자 차단"}>
                    <span>
                      <IconButton
                        size="small"
                        color={already ? "error" : "default"}
                        onClick={(e) => {
                          e.stopPropagation();
                          onBlockSender(p.row.from);
                        }}
                      >
                        <BlockIcon fontSize="small" />
                      </IconButton>
                    </span>
                  </Tooltip>
                );
              },
            } satisfies GridColDef<(typeof rows)[number]>,
          ]
        : []),
    ],
    [onBlockSender, blockedEmails],
  );

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
    onSearch?.(query.trim(), {
      from: fromDate ? fromDate.toISOString() : null,
      to: toDate ? toDate.toISOString() : null,
    });
  }

  function clearRange() {
    setFromDate(null);
    setToDate(null);
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
            <>
              <DateTimePicker
                label="시작"
                value={fromDate}
                onChange={setFromDate}
                ampm={false}
                format="YYYY-MM-DD HH:mm"
                slotProps={{ textField: { size: "small", sx: { width: 190 } } }}
              />
              <DateTimePicker
                label="종료"
                value={toDate}
                onChange={setToDate}
                ampm={false}
                format="YYYY-MM-DD HH:mm"
                minDateTime={fromDate ?? undefined}
                slotProps={{ textField: { size: "small", sx: { width: 190 } } }}
              />
              {(fromDate || toDate) && (
                <Button size="small" onClick={clearRange}>
                  기간 초기화
                </Button>
              )}
              <Button type="submit" onClick={submitSearch} size="small" variant="outlined" disabled={loading}>
                {loading ? "검색 중..." : "검색"}
              </Button>
            </>
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

      <Box sx={{ flex: 1, minHeight: 0 }} className="thin-scroll">
        <DataGrid
          rows={rows}
          columns={columns}
          density="standard"
          getRowHeight={() => "auto"}
          disableRowSelectionOnClick
          hideFooterSelectedRowCount
          onRowClick={(p: GridRowParams<(typeof rows)[number]>) => setSelected(p.row)}
          localeText={{ noRowsLabel: emptyHint }}
          pageSizeOptions={[25, 50, 100]}
          initialState={{ pagination: { paginationModel: { pageSize: 50, page: 0 } } }}
          sx={{
            border: "none",
            height: "100%",
            fontSize: 13,
            "--DataGrid-rowBorderColor": "var(--mui-palette-divider, #1e2730)",
            "& .MuiDataGrid-columnHeaders": {
              bgcolor: "background.paper",
              borderBottom: 1,
              borderColor: "divider",
            },
            "& .MuiDataGrid-columnHeaderTitle": { fontWeight: 700, fontSize: 12 },
            "& .MuiDataGrid-cell": { borderColor: "divider", alignItems: "flex-start" },
            "& .MuiDataGrid-cell.mm-mono": { fontFamily: monoFont, fontSize: 12.5 },
            "& .MuiDataGrid-cell.mm-dim": { color: "text.secondary" },
            "& .MuiDataGrid-row": { cursor: "pointer" },
            "& .MuiDataGrid-row:hover": { bgcolor: "action.hover" },
            "& .MuiDataGrid-footerContainer": { borderColor: "divider" },
            "& .MuiDataGrid-virtualScroller": { minHeight: 80 },
          }}
        />
      </Box>
      <EventDetailDialog
        event={selected}
        onClose={() => setSelected(null)}
        onBlockSender={onBlockSender}
        alreadyBlocked={selected ? blockedEmails?.has(selected.from) : false}
      />
    </Box>
  );
}
