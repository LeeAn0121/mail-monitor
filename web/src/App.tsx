import { useState } from "react";
import Box from "@mui/material/Box";
import Chip from "@mui/material/Chip";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import LogTable from "./components/LogTable";
import NavRail from "./components/NavRail";
import type { View } from "./components/NavRail";
import RankingList from "./components/RankingList";
import StatusStrip from "./components/StatusStrip";
import { eventColors } from "./theme";
import { useDashboard } from "./useDashboard";
import { useHistorySearch } from "./useHistorySearch";

export default function App() {
  const [view, setView] = useState<View>("live");
  const { connected, events, counts, senderRanking, receiverRanking, alertActive, refresh } = useDashboard();
  const history = useHistorySearch();

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
          <Typography variant="h6" sx={{ fontWeight: 700, letterSpacing: "-0.01em" }}>
            mail-monitor
          </Typography>
          <Stack direction="row" spacing={1} alignItems="center">
            {alertActive && (
              <Chip
                size="small"
                label="BOUNCE/REJECT 급증"
                sx={{ bgcolor: "transparent", color: eventColors.BOUNCE, border: 1, borderColor: eventColors.BOUNCE, fontWeight: 700 }}
              />
            )}
            <Chip
              size="small"
              label={connected ? "실시간 연결됨" : "연결 끊김"}
              sx={{
                bgcolor: "transparent",
                border: 1,
                borderColor: connected ? eventColors.RECV : "divider",
                color: connected ? eventColors.RECV : "text.secondary",
                fontWeight: 600,
              }}
            />
          </Stack>
        </Stack>

        <Box sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2, minHeight: 0, flex: 1 }}>
          {view === "live" ? (
            <>
              <StatusStrip counts={counts} />
              <Stack direction={{ xs: "column", lg: "row" }} spacing={2} sx={{ flex: 1, minHeight: 0 }}>
                <Box sx={{ flex: 3, minWidth: 0, display: "flex" }}>
                  <LogTable
                    title="실시간 로그"
                    events={events}
                    mode="live"
                    onRefresh={refresh}
                    emptyHint="이벤트를 기다리는 중..."
                    filename="mail-monitor_live"
                  />
                </Box>
                <Stack sx={{ flex: 1, minWidth: 260, minHeight: 0 }} spacing={2}>
                  <RankingList title="발신 랭킹" entries={senderRanking} color={eventColors.SENT} />
                  <RankingList title="수신 랭킹" entries={receiverRanking} color={eventColors.RECV} />
                </Stack>
              </Stack>
            </>
          ) : (
            <Box sx={{ display: "flex", flex: 1, minHeight: 0 }}>
              <LogTable
                title="이력 검색"
                events={history.results}
                mode="history"
                onSearch={history.search}
                onRefresh={() => history.search(history.searchedFor ?? history.initialQuery)}
                initialQuery={history.initialQuery}
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
        </Box>
      </Box>
    </Box>
  );
}
