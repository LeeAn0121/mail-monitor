import Box from "@mui/material/Box";
import LinearProgress from "@mui/material/LinearProgress";
import Paper from "@mui/material/Paper";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import type { RankEntry } from "../types";

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
    <Paper variant="outlined" sx={{ p: 2, height: "100%", borderColor: "divider" }}>
      <Typography variant="subtitle2" color="text.secondary" gutterBottom>
        {title}
      </Typography>
      {entries.length === 0 ? (
        <Typography variant="body2" color="text.disabled">
          데이터 없음
        </Typography>
      ) : (
        <Stack spacing={1}>
          {entries.map((e) => (
            <Box key={e.addr}>
              <Stack direction="row" justifyContent="space-between">
                <Typography variant="body2" noWrap sx={{ maxWidth: "70%" }}>
                  {e.addr}
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  {e.count.toLocaleString()}
                </Typography>
              </Stack>
              <LinearProgress
                variant="determinate"
                value={(e.count / max) * 100}
                sx={{
                  height: 6,
                  borderRadius: 3,
                  bgcolor: "action.hover",
                  "& .MuiLinearProgress-bar": { bgcolor: color },
                }}
              />
            </Box>
          ))}
        </Stack>
      )}
    </Paper>
  );
}
