import { useState } from "react";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Dialog from "@mui/material/Dialog";
import DialogContent from "@mui/material/DialogContent";
import DialogTitle from "@mui/material/DialogTitle";
import Divider from "@mui/material/Divider";
import IconButton from "@mui/material/IconButton";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import CloseIcon from "@mui/icons-material/Close";
import ContentCopyIcon from "@mui/icons-material/ContentCopy";
import CheckIcon from "@mui/icons-material/Check";
import ErrorOutlineIcon from "@mui/icons-material/ErrorOutline";
import type { WebEvent } from "../types";
import { eventColors, monoFont } from "../theme";

// navigator.clipboard requires a secure context (HTTPS, or localhost) — this
// dashboard is commonly reached over plain HTTP on a LAN/internal IP, where
// the API is simply absent or silently rejects. Fall back to the older
// execCommand("copy") path (works over HTTP, deprecated but still supported)
// so copying doesn't just fail quietly either way.
async function copyToClipboard(text: string): Promise<boolean> {
  if (navigator.clipboard && window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch {
      // fall through to the execCommand fallback below
    }
  }
  try {
    const ta = document.createElement("textarea");
    ta.value = text;
    ta.style.position = "fixed";
    ta.style.opacity = "0";
    document.body.appendChild(ta);
    ta.focus();
    ta.select();
    const ok = document.execCommand("copy");
    document.body.removeChild(ta);
    return ok;
  } catch {
    return false;
  }
}

function Field({ label, value, mono, color }: { label: string; value: string; mono?: boolean; color?: string }) {
  return (
    <Box>
      <Typography variant="caption" sx={{ color: "text.disabled", display: "block" }}>
        {label}
      </Typography>
      <Typography
        variant="body2"
        sx={{ fontFamily: mono ? monoFont : undefined, color: color ?? "text.primary", wordBreak: "break-word" }}
      >
        {value || "—"}
      </Typography>
    </Box>
  );
}

type CopyState = "idle" | "copied" | "failed";

export default function EventDetailDialog({ event, onClose }: { event: WebEvent | null; onClose: () => void }) {
  const [copyState, setCopyState] = useState<CopyState>("idle");

  async function copyRaw() {
    if (!event) return;
    const ok = await copyToClipboard(event.raw);
    setCopyState(ok ? "copied" : "failed");
    setTimeout(() => setCopyState("idle"), 1500);
  }

  return (
    <Dialog open={!!event} onClose={onClose} maxWidth="sm" fullWidth>
      {event && (
        <>
          <DialogTitle sx={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
            <Stack direction="row" spacing={1} alignItems="center">
              <Typography sx={{ fontFamily: monoFont, fontWeight: 700, color: eventColors[event.type] }}>
                {event.glyph} {event.type}
              </Typography>
              <Typography variant="caption" sx={{ fontFamily: monoFont, color: "text.disabled" }}>
                {event.when}
              </Typography>
            </Stack>
            <IconButton size="small" onClick={onClose}>
              <CloseIcon fontSize="small" />
            </IconButton>
          </DialogTitle>
          <DialogContent dividers>
            <Stack spacing={2}>
              <Stack direction="row" spacing={3}>
                <Box sx={{ flex: 1 }}>
                  <Field label="발신" value={event.from} mono />
                </Box>
                <Box sx={{ flex: 1 }}>
                  <Field label="수신" value={event.to} mono />
                </Box>
              </Stack>
              {event.origTo && (
                <Field label="원본 수신(별칭)" value={event.origTo} mono />
              )}
              <Field label="내용(제목)" value={event.subject} />
              <Field
                label="처리결과"
                value={event.result}
                color={["BOUNCE", "REJECT"].includes(event.type) ? eventColors[event.type] : undefined}
              />
              <Stack direction="row" spacing={3}>
                <Box sx={{ flex: 1 }}>
                  <Field label="발신자 IP" value={event.fromIp} mono />
                </Box>
                <Box sx={{ flex: 1 }}>
                  <Field label="수신자 IP" value={event.toIp} mono />
                </Box>
              </Stack>

              <Divider />

              <Box>
                <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 0.5 }}>
                  <Typography variant="caption" sx={{ color: "text.disabled" }}>
                    원본 로그
                  </Typography>
                  <Button
                    size="small"
                    color={copyState === "failed" ? "error" : "primary"}
                    startIcon={
                      copyState === "copied" ? (
                        <CheckIcon fontSize="small" />
                      ) : copyState === "failed" ? (
                        <ErrorOutlineIcon fontSize="small" />
                      ) : (
                        <ContentCopyIcon fontSize="small" />
                      )
                    }
                    onClick={copyRaw}
                  >
                    {copyState === "copied" ? "복사됨" : copyState === "failed" ? "복사 실패" : "복사"}
                  </Button>
                </Stack>
                <Box
                  component="pre"
                  sx={{
                    m: 0,
                    p: 1.5,
                    fontFamily: monoFont,
                    fontSize: 12,
                    bgcolor: "background.default",
                    border: 1,
                    borderColor: "divider",
                    borderRadius: 1,
                    overflowX: "auto",
                    whiteSpace: "pre-wrap",
                    wordBreak: "break-all",
                  }}
                >
                  {event.raw}
                </Box>
              </Box>
            </Stack>
          </DialogContent>
        </>
      )}
    </Dialog>
  );
}
