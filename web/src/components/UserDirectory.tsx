import { useMemo, useState } from "react";
import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import IconButton from "@mui/material/IconButton";
import InputAdornment from "@mui/material/InputAdornment";
import Stack from "@mui/material/Stack";
import TextField from "@mui/material/TextField";
import Tooltip from "@mui/material/Tooltip";
import Typography from "@mui/material/Typography";
import SearchIcon from "@mui/icons-material/Search";
import RefreshIcon from "@mui/icons-material/Refresh";
import VisibilityIcon from "@mui/icons-material/Visibility";
import VisibilityOffIcon from "@mui/icons-material/VisibilityOff";
import ContentCopyIcon from "@mui/icons-material/ContentCopy";
import { monoFont } from "../theme";
import { useUsers } from "../useUsers";
import { copyToClipboard } from "../clipboard";

// Rendered in place of an actual password so the column has a consistent
// width regardless of the real value's length, and never leaks the length
// of a hidden password either.
const MASK = "••••••••";

export default function UserDirectory() {
  const { users, enabled, loading, error, refresh } = useUsers();
  const [query, setQuery] = useState("");
  // Passwords are plaintext in this table — masked by default and revealed
  // one row at a time on click, rather than all at once, so a shoulder-surf
  // or screen-share doesn't expose the whole table just because someone
  // needed to check one password.
  const [revealed, setRevealed] = useState<Set<string>>(new Set());

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return users;
    return users.filter((u) => u.email.toLowerCase().includes(q) || u.name.toLowerCase().includes(q));
  }, [users, query]);

  function toggleReveal(email: string) {
    setRevealed((prev) => {
      const next = new Set(prev);
      if (next.has(email)) next.delete(email);
      else next.add(email);
      return next;
    });
  }

  return (
    <Box sx={{ border: 1, borderColor: "divider", display: "flex", flexDirection: "column", minHeight: 0, flex: 1 }}>
      <Stack
        direction={{ xs: "column", sm: "row" }}
        spacing={1.5}
        alignItems={{ sm: "center" }}
        justifyContent="space-between"
        sx={{ px: 2, py: 1.5, borderBottom: 1, borderColor: "divider" }}
      >
        <Stack direction="row" spacing={1} alignItems="baseline">
          <Typography variant="subtitle2" sx={{ fontWeight: 700 }}>
            사용자 계정
          </Typography>
          <Typography variant="caption" sx={{ color: "text.disabled", fontFamily: monoFont }}>
            {filtered.length.toLocaleString()}건
          </Typography>
        </Stack>

        <Stack direction="row" spacing={1} sx={{ width: { xs: "100%", sm: "auto" } }}>
          <TextField
            size="small"
            fullWidth
            placeholder="이메일 또는 이름 검색"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            sx={{ width: { xs: "100%", sm: 260 } }}
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

      {error && (
        <Alert severity="error" sx={{ borderRadius: 0 }}>
          {error}
        </Alert>
      )}

      <Box sx={{ overflowY: "auto", flex: 1, minHeight: 0 }} className="thin-scroll">
        {loading ? (
          <Typography variant="body2" color="text.disabled" sx={{ p: 3, textAlign: "center" }}>
            불러오는 중...
          </Typography>
        ) : !enabled ? (
          <Typography variant="body2" color="text.disabled" sx={{ p: 3, textAlign: "center" }}>
            사용자 조회 기능이 설정되어 있지 않습니다 — MAIL_MONITOR_DB_DSN 환경변수를
            설정해야 사용할 수 있습니다 (README "이름 표시" 항목 참고).
          </Typography>
        ) : filtered.length === 0 ? (
          <Typography variant="body2" color="text.disabled" sx={{ p: 3, textAlign: "center" }}>
            {users.length === 0 ? "등록된 사용자가 없습니다." : "검색 결과가 없습니다."}
          </Typography>
        ) : (
          <>
            <Box
              sx={{
                display: { xs: "none", sm: "grid" },
                gridTemplateColumns: "1.4fr 1fr 1fr",
                px: 2,
                py: 0.75,
                borderBottom: 1,
                borderColor: "divider",
                color: "text.secondary",
              }}
            >
              {["이메일", "이름", "비밀번호"].map((h) => (
                <Typography key={h} variant="caption" sx={{ fontWeight: 600 }}>
                  {h}
                </Typography>
              ))}
            </Box>
            {filtered.map((u) => {
              const isRevealed = revealed.has(u.email);
              return (
                <Box
                  key={u.email}
                  sx={{
                    display: "grid",
                    gridTemplateColumns: { xs: "1fr", sm: "1.4fr 1fr 1fr" },
                    gap: { xs: 0.5, sm: 2 },
                    px: 2,
                    py: 1,
                    borderBottom: 1,
                    borderColor: "divider",
                    alignItems: "center",
                    "&:hover": { bgcolor: "action.hover" },
                  }}
                >
                  <Typography sx={{ fontFamily: monoFont, fontSize: 13, minWidth: 0 }} noWrap>
                    {u.email}
                  </Typography>
                  <Typography variant="body2">
                    {u.name || <Box component="span" sx={{ color: "text.disabled" }}>이름 없음</Box>}
                  </Typography>
                  <Stack direction="row" spacing={0.5} alignItems="center">
                    <Typography
                      sx={{ fontFamily: monoFont, fontSize: 13, color: isRevealed ? "error.main" : "text.secondary" }}
                    >
                      {isRevealed ? u.password : MASK}
                    </Typography>
                    <Tooltip title={isRevealed ? "숨기기" : "표시"}>
                      <IconButton size="small" onClick={() => toggleReveal(u.email)}>
                        {isRevealed ? <VisibilityOffIcon fontSize="small" /> : <VisibilityIcon fontSize="small" />}
                      </IconButton>
                    </Tooltip>
                    {isRevealed && (
                      <Tooltip title="복사">
                        <IconButton size="small" onClick={() => copyToClipboard(u.password)}>
                          <ContentCopyIcon fontSize="small" />
                        </IconButton>
                      </Tooltip>
                    )}
                  </Stack>
                </Box>
              );
            })}
          </>
        )}
      </Box>
    </Box>
  );
}
