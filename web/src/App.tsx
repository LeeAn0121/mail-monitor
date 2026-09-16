import { useState } from "react";
import Box from "@mui/material/Box";
import Chip from "@mui/material/Chip";
import Link from "@mui/material/Link";
import Snackbar from "@mui/material/Snackbar";
import Alert from "@mui/material/Alert";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import BlockList from "./components/BlockList";
import LogTable from "./components/LogTable";
import NavRail from "./components/NavRail";
import type { View } from "./components/NavRail";
import RankingList from "./components/RankingList";
import StatusStrip from "./components/StatusStrip";
import { accentSignal, eventColors, monoFont } from "./theme";
import { useBlocklist } from "./useBlocklist";
import { useDashboard } from "./useDashboard";
import { useHistorySearch } from "./useHistorySearch";
import { useVersion } from "./useVersion";

export default function App() {
  const [view, setView] = useState<View>("live");
  const { connected, loaded, events, counts, senderRanking, receiverRanking, alertActive, refresh } = useDashboard();
  const history = useHistorySearch();
  const versionInfo = useVersion();
  const blocklist = useBlocklist();
  const [toast, setToast] = useState<{ message: string; severity: "success" | "error" } | null>(null);

  const blockedEmails = new Set(blocklist.blocked.map((b) => b.email));

  async function handleBlockSender(email: string) {
    if (!email || email === "-" || email === "<>") return;
    if (blockedEmails.has(email)) {
      setToast({ message: `이미 차단된 주소입니다: ${email}`, severity: "success" });
      return;
    }
    if (!window.confirm(`이 발신자를 차단할까요?\n${email}`)) return;
    const err = await blocklist.block(email);
    setToast(
      err ? { message: err, severity: "error" } : { message: `차단했습니다: ${email}`, severity: "success" },
    );
  }

  return (
    <Box sx={{ display: "flex", height: "100vh", bgcolor: "background.default" }}>
      <NavRail view={view} onChange={setView} />

      <Box sx={{ flex: 1, minWidth: 0, display: "flex", flexDirection: "column" }}>
        <Stack
          direction="row"
          alignItems="center"
          justifyContent="space-between"
          sx={{ px: 2.5, py: 1.5, borderBottom: 1, borderColor: "divider" }}
        >
          <Stack direction="row" spacing={1} alignItems="baseline">
            <Typography
              variant="h6"
              sx={{ fontFamily: monoFont, fontWeight: 700, letterSpacing: "-0.01em", color: "text.primary" }}
            >
              <Box component="span" sx={{ color: accentSignal }}>
                ~/
              </Box>
              mail-monitor
              <Box component="span" className="mm-cursor" sx={{ color: accentSignal, ml: "1px" }}>
                _
              </Box>
            </Typography>
            {versionInfo && (
              <Link
                href={versionInfo.releaseUrl}
                target="_blank"
                rel="noopener noreferrer"
                underline="hover"
                variant="caption"
                sx={{ fontFamily: monoFont, color: "text.disabled" }}
              >
                v{versionInfo.version}
              </Link>
            )}
          </Stack>
          <Stack direction="row" spacing={1} alignItems="center">
            {alertActive && (
              <Chip
                size="small"
                label="BOUNCE/REJECT 급증"
                sx={{ bgcolor: "transparent", color: eventColors.BOUNCE, border: 1, borderColor: eventColors.BOUNCE, fontWeight: 700 }}
              />
            )}
            <Stack
              direction="row"
              spacing={1}
              alignItems="center"
              sx={{ border: 1, borderColor: connected ? accentSignal : "divider", px: 1.25, py: 0.5 }}
            >
              <Box
                sx={{
                  width: 7,
                  height: 7,
                  borderRadius: "50%",
                  bgcolor: connected ? accentSignal : "text.disabled",
                  boxShadow: connected ? `0 0 6px ${accentSignal}` : "none",
                }}
              />
              <Typography variant="caption" sx={{ fontWeight: 600, color: connected ? accentSignal : "text.secondary" }}>
                {connected ? "실시간 연결됨" : "연결 끊김"}
              </Typography>
            </Stack>
          </Stack>
        </Stack>

        <Box sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2, minHeight: 0, flex: 1 }}>
          {view === "live" && (
            <>
              <StatusStrip counts={counts} />
              <Box sx={{ flex: 1, minHeight: 0, display: "flex" }}>
                <LogTable
                  title="실시간 로그"
                  events={events}
                  mode="live"
                  onRefresh={refresh}
                  onBlockSender={handleBlockSender}
                  blockedEmails={blockedEmails}
                  emptyHint={loaded ? "이벤트를 기다리는 중..." : "불러오는 중..."}
                  filename="mail-monitor_live"
                />
              </Box>
            </>
          )}

          {view === "ranking" && (
            <Stack direction={{ xs: "column", md: "row" }} spacing={2} sx={{ flex: 1, minHeight: 0 }}>
              <RankingList title="발신 랭킹" entries={senderRanking} color={eventColors.SENT} />
              <RankingList title="수신 랭킹" entries={receiverRanking} color={eventColors.RECV} />
            </Stack>
          )}

          {view === "history" && (
            <Box sx={{ display: "flex", flex: 1, minHeight: 0 }}>
              <LogTable
                title="이력 검색"
                events={history.results}
                mode="history"
                onSearch={history.search}
                onRefresh={() =>
                  history.search(history.searchedFor ?? history.initialQuery, history.appliedRange)
                }
                onBlockSender={handleBlockSender}
                blockedEmails={blockedEmails}
                initialQuery={history.initialQuery}
                initialRange={history.initialRange}
                loading={history.loading}
                emptyHint={
                  history.error
                    ? history.error
                    : history.searchedFor === null
                      ? "검색어를 입력하세요 — 비워두면 전체 이력을 가져옵니다 (로테이션된 로그 포함)."
                      : `"${history.searchedFor}"에 대한 결과가 없습니다.`
                }
                filename="mail-monitor_history"
              />
            </Box>
          )}

          {view === "block" && (
            <Box sx={{ display: "flex", flex: 1, minHeight: 0 }}>
              <BlockList blocklist={blocklist} />
            </Box>
          )}
        </Box>
      </Box>

      <Snackbar
        open={!!toast}
        autoHideDuration={3000}
        onClose={() => setToast(null)}
        anchorOrigin={{ vertical: "bottom", horizontal: "center" }}
      >
        {toast ? (
          <Alert severity={toast.severity} onClose={() => setToast(null)} sx={{ border: 1, borderColor: "divider" }}>
            {toast.message}
          </Alert>
        ) : undefined}
      </Snackbar>
    </Box>
  );
}
