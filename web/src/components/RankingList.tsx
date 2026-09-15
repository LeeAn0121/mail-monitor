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
    <Box sx={{ border: 1, borderColor: "divider" }}>
      <Typography
        variant="caption"
        sx={{ fontWeight: 700, display: "block", px: 2, py: 1.25, borderBottom: 1, borderColor: "divider" }}
      >
        {title}
      </Typography>
      <Box sx={{ p: entries.length === 0 ? 3 : 0 }}>
        {entries.length === 0 ? (
          <Typography variant="body2" color="text.disabled" textAlign="center">
            데이터 없음
          </Typography>
        ) : (
          <Stack divider={<Box sx={{ borderBottom: 1, borderColor: "divider" }} />}>
            {entries.map((e) => (
              <Box key={e.addr} sx={{ position: "relative", px: 2, py: 1 }}>
                <Box
                  sx={{
                    position: "absolute",
                    inset: 0,
                    width: `${(e.count / max) * 100}%`,
                    bgcolor: color,
                    opacity: 0.12,
                  }}
                />
                <Stack direction="row" justifyContent="space-between" sx={{ position: "relative" }}>
                  <Typography variant="body2" noWrap sx={{ maxWidth: "70%", fontFamily: monoFont, fontSize: 12.5 }}>
                    {e.addr}
                  </Typography>
                  <Typography variant="body2" sx={{ fontFamily: monoFont, fontWeight: 600 }}>
                    {e.count.toLocaleString()}
                  </Typography>
                </Stack>
              </Box>
            ))}
          </Stack>
        )}
      </Box>
    </Box>
  );
}
