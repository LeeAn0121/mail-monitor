import Box from "@mui/material/Box";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import type { RankEntry } from "../types";
import { monoFont } from "../theme";

export default function RankingList({
  title,
  entries,
  color,
}: {
  title: string;
  entries: RankEntry[];
  color: string;
}) {
  const max = entries.length > 0 ? entries[0].count : 1;
  return (
    <Box sx={{ border: 1, borderColor: "divider", display: "flex", flexDirection: "column", minHeight: 0, flex: 1 }}>
      <Stack
        direction="row"
        justifyContent="space-between"
        alignItems="baseline"
        sx={{ px: 2, py: 1.25, borderBottom: 1, borderColor: "divider", flexShrink: 0 }}
      >
        <Typography variant="caption" sx={{ fontWeight: 700 }}>
          {title}
        </Typography>
        {entries.length > 0 && (
          <Typography variant="caption" sx={{ color: "text.disabled", fontFamily: monoFont }}>
            {entries.length}건
          </Typography>
        )}
      </Stack>

      <Box className="thin-scroll" sx={{ overflowY: "auto", flex: 1, minHeight: 0 }}>
        {entries.length === 0 ? (
          <Typography variant="body2" color="text.disabled" sx={{ p: 3, textAlign: "center" }}>
            데이터 없음
          </Typography>
        ) : (
          entries.map((e, i) => (
            <Box
              key={e.addr}
              sx={{
                position: "relative",
                px: 2,
                py: 0.875,
                borderBottom: 1,
                borderColor: "divider",
                transition: "background-color .12s",
                "&:hover": { bgcolor: "action.hover" },
              }}
            >
              <Box
                sx={{
                  position: "absolute",
                  inset: 0,
                  width: `${(e.count / max) * 100}%`,
                  bgcolor: color,
                  opacity: 0.1,
                }}
              />
              <Stack direction="row" spacing={1} alignItems="center" sx={{ position: "relative" }}>
                <Typography
                  variant="caption"
                  sx={{ fontFamily: monoFont, color: "text.disabled", width: 20, flexShrink: 0, textAlign: "right" }}
                >
                  {i + 1}
                </Typography>
                <Typography
                  variant="body2"
                  noWrap
                  sx={{ fontFamily: monoFont, fontSize: 12.5, flex: 1, minWidth: 0 }}
                  title={e.addr}
                >
                  {e.addr}
                </Typography>
                <Typography variant="body2" sx={{ fontFamily: monoFont, fontWeight: 600, flexShrink: 0 }}>
                  {e.count.toLocaleString()}
                </Typography>
              </Stack>
            </Box>
          ))
        )}
      </Box>
    </Box>
  );
}
