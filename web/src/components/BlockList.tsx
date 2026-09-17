import { useState } from "react";
import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import IconButton from "@mui/material/IconButton";
import InputAdornment from "@mui/material/InputAdornment";
import Stack from "@mui/material/Stack";
import TextField from "@mui/material/TextField";
import Tooltip from "@mui/material/Tooltip";
import Typography from "@mui/material/Typography";
import BlockIcon from "@mui/icons-material/Block";
import DeleteOutlineIcon from "@mui/icons-material/DeleteOutline";
import RefreshIcon from "@mui/icons-material/Refresh";
import { monoFont } from "../theme";
import type { useBlocklist } from "../useBlocklist";

export default function BlockList({ blocklist }: { blocklist: ReturnType<typeof useBlocklist> }) {
  const { blocked, loading, error, pending, block, unblock, refresh } = blocklist;
  const [email, setEmail] = useState("");
  const [formError, setFormError] = useState<string | null>(null);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    const trimmed = email.trim();
    if (!trimmed) return;
    const err = await block(trimmed);
    if (err) {
      setFormError(err);
    } else {
      setFormError(null);
      setEmail("");
    }
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
            발신자 차단
          </Typography>
          <Typography variant="caption" sx={{ color: "text.disabled", fontFamily: monoFont }}>
            {blocked.length}건
          </Typography>
        </Stack>

        <Stack
          component="form"
          onSubmit={submit}
          direction={{ xs: "column", sm: "row" }}
          spacing={1}
          sx={{ width: { xs: "100%", sm: "auto" } }}
        >
          <TextField
            size="small"
            fullWidth
            placeholder="차단할 이메일 주소"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            sx={{ width: { xs: "100%", sm: 260 } }}
            InputProps={{
              startAdornment: (
                <InputAdornment position="start">
                  <BlockIcon fontSize="small" />
                </InputAdornment>
              ),
            }}
          />
          <Stack direction="row" spacing={1}>
            <Button
              type="submit"
              size="small"
              variant="outlined"
              disabled={pending || !email.trim()}
              sx={{ flex: { xs: 1, sm: "initial" } }}
            >
              차단
            </Button>
            <Tooltip title="새로고침">
              <IconButton size="small" onClick={refresh}>
                <RefreshIcon fontSize="small" />
              </IconButton>
            </Tooltip>
          </Stack>
        </Stack>
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
        ) : blocked.length === 0 ? (
          <Typography variant="body2" color="text.disabled" sx={{ p: 3, textAlign: "center" }}>
            차단된 발신자가 없습니다.
          </Typography>
        ) : (
          blocked.map((b) => (
            <Stack
              key={b.email}
              direction="row"
              alignItems="center"
              justifyContent="space-between"
              sx={{
                px: 2,
                py: 1,
                borderBottom: 1,
                borderColor: "divider",
                "&:hover": { bgcolor: "action.hover" },
              }}
            >
              <Stack direction="row" spacing={2} alignItems="baseline" sx={{ minWidth: 0 }}>
                <Typography sx={{ fontFamily: monoFont, fontSize: 13 }} noWrap>
                  {b.email}
                </Typography>
                {b.addedAt && (
                  <Typography variant="caption" sx={{ color: "text.disabled", fontFamily: monoFont, flexShrink: 0 }}>
                    {b.addedAt}
                  </Typography>
                )}
              </Stack>
              <Tooltip title="차단 해제">
                <IconButton size="small" onClick={() => unblock(b.email)} disabled={pending}>
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
