import { useMemo, useState } from "react";
import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import IconButton from "@mui/material/IconButton";
import InputAdornment from "@mui/material/InputAdornment";
import Stack from "@mui/material/Stack";
import TextField from "@mui/material/TextField";
import Tooltip from "@mui/material/Tooltip";
import Typography from "@mui/material/Typography";
import ArrowForwardIcon from "@mui/icons-material/ArrowForward";
import DeleteOutlineIcon from "@mui/icons-material/DeleteOutline";
import RefreshIcon from "@mui/icons-material/Refresh";
import SearchIcon from "@mui/icons-material/Search";
import { monoFont } from "../theme";
import { useForwardings } from "../useForwardings";

function AddressLabel({ email, name }: { email: string; name: string }) {
  return (
    <Stack direction="row" spacing={1} alignItems="baseline" sx={{ minWidth: 0 }}>
      <Typography sx={{ fontFamily: monoFont, fontSize: 13 }} noWrap>
        {email}
      </Typography>
      {name && (
        <Typography variant="caption" sx={{ color: "text.disabled", flexShrink: 0 }}>
          ({name})
        </Typography>
      )}
    </Stack>
  );
}

export default function ForwardingList() {
  const { forwardings, enabled, loading, error, pending, add, remove, refresh } = useForwardings();
  const [query, setQuery] = useState("");
  const [source, setSource] = useState("");
  const [destination, setDestination] = useState("");
  const [formError, setFormError] = useState<string | null>(null);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return forwardings;
    return forwardings.filter(
      (f) =>
        f.source.toLowerCase().includes(q) ||
        f.destination.toLowerCase().includes(q) ||
        f.sourceName.toLowerCase().includes(q) ||
        f.destinationName.toLowerCase().includes(q),
    );
  }, [forwardings, query]);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    const s = source.trim();
    const d = destination.trim();
    if (!s || !d) return;
    const err = await add(s, d);
    if (err) {
      setFormError(err);
    } else {
      setFormError(null);
      setSource("");
      setDestination("");
    }
  }

  return (
    <Box sx={{ border: 1, borderColor: "divider", display: "flex", flexDirection: "column", minHeight: 0, flex: 1 }}>
      <Stack
        direction={{ xs: "column", md: "row" }}
        spacing={1.5}
        alignItems={{ md: "center" }}
        justifyContent="space-between"
        sx={{ px: 2, py: 1.5, borderBottom: 1, borderColor: "divider" }}
      >
        <Stack direction="row" spacing={1} alignItems="baseline" flexShrink={0}>
          <Typography variant="subtitle2" sx={{ fontWeight: 700 }}>
            포워딩 관리
          </Typography>
          <Typography variant="caption" sx={{ color: "text.disabled", fontFamily: monoFont }}>
            {filtered.length.toLocaleString()}건
          </Typography>
        </Stack>

        <Stack direction="row" spacing={1.5} alignItems="center" flexWrap="wrap" useFlexGap>
          <TextField
            size="small"
            placeholder="검색"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            sx={{ width: { xs: "100%", sm: 200 } }}
            InputProps={{
              startAdornment: (
                <InputAdornment position="start">
                  <SearchIcon fontSize="small" />
                </InputAdornment>
              ),
            }}
          />
          <Tooltip title="새로고침">
            <IconButton size="small" onClick={refresh}>
              <RefreshIcon fontSize="small" />
            </IconButton>
          </Tooltip>
        </Stack>
      </Stack>

      <Stack
        component="form"
        onSubmit={submit}
        direction={{ xs: "column", sm: "row" }}
        spacing={1}
        alignItems={{ sm: "center" }}
        sx={{ px: 2, py: 1.5, borderBottom: 1, borderColor: "divider" }}
      >
        <TextField
          size="small"
          fullWidth
          placeholder="원본 주소 (source)"
          value={source}
          onChange={(e) => setSource(e.target.value)}
          sx={{ width: { xs: "100%", sm: 240 } }}
        />
        <ArrowForwardIcon fontSize="small" sx={{ color: "text.disabled", display: { xs: "none", sm: "block" } }} />
        <TextField
          size="small"
          fullWidth
          placeholder="전달 주소 (destination)"
          value={destination}
          onChange={(e) => setDestination(e.target.value)}
          sx={{ width: { xs: "100%", sm: 240 } }}
        />
        <Button
          type="submit"
          size="small"
          variant="outlined"
          disabled={pending || !source.trim() || !destination.trim()}
          sx={{ flexShrink: 0, width: { xs: "100%", sm: "auto" } }}
        >
          추가
        </Button>
      </Stack>

      {(error || formError) && (
        <Alert severity="error" sx={{ borderRadius: 0 }}>
          {formError ?? error}
        </Alert>
      )}

      <Box sx={{ overflowY: "auto", flex: 1, minHeight: 0 }} className="thin-scroll">
        {loading ? (
          <Typography variant="body2" color="text.disabled" sx={{ p: 3, textAlign: "center" }}>
            불러오는 중...
          </Typography>
        ) : !enabled ? (
          <Typography variant="body2" color="text.disabled" sx={{ p: 3, textAlign: "center" }}>
            포워딩 관리 기능이 설정되어 있지 않습니다 — MAIL_MONITOR_DB_DSN 환경변수를
            설정해야 사용할 수 있습니다.
          </Typography>
        ) : filtered.length === 0 ? (
          <Typography variant="body2" color="text.disabled" sx={{ p: 3, textAlign: "center" }}>
            {forwardings.length === 0 ? "등록된 포워딩이 없습니다." : "검색 결과가 없습니다."}
          </Typography>
        ) : (
          filtered.map((f) => (
            <Stack
              key={`${f.source}→${f.destination}`}
              direction="row"
              alignItems="center"
              justifyContent="space-between"
              spacing={2}
              sx={{
                px: 2,
                py: 1,
                borderBottom: 1,
                borderColor: "divider",
                "&:hover": { bgcolor: "action.hover" },
              }}
            >
              <Stack direction="row" spacing={1.5} alignItems="center" sx={{ minWidth: 0, flex: 1 }}>
                <AddressLabel email={f.source} name={f.sourceName} />
                <ArrowForwardIcon fontSize="small" sx={{ color: "text.disabled", flexShrink: 0 }} />
                <AddressLabel email={f.destination} name={f.destinationName} />
              </Stack>
              <Tooltip title="삭제">
                <IconButton size="small" onClick={() => remove(f.source, f.destination)} disabled={pending}>
                  <DeleteOutlineIcon fontSize="small" />
                </IconButton>
              </Tooltip>
            </Stack>
          ))
        )}
      </Box>
    </Box>
  );
}
