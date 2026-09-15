import Box from "@mui/material/Box";
import Chip from "@mui/material/Chip";
import Paper from "@mui/material/Paper";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import type { WebEvent } from "../types";
import { eventColors } from "../theme";

export default function EventFeed({ events }: { events: WebEvent[] }) {
  return (
    <Paper variant="outlined" sx={{ borderColor: "divider", overflow: "hidden" }}>
      <Box sx={{ px: 2, py: 1.5, borderBottom: 1, borderColor: "divider" }}>
        <Typography variant="subtitle2" color="text.secondary">
          실시간 로그
        </Typography>
      </Box>
      <Box sx={{ maxHeight: 560, overflowY: "auto" }}>
        {events.length === 0 ? (
          <Typography variant="body2" color="text.disabled" sx={{ p: 2 }}>
            이벤트를 기다리는 중...
          </Typography>
        ) : (
          <Stack divider={<Box sx={{ borderBottom: 1, borderColor: "divider" }} />}>
            {events.map((ev, i) => (
              <Stack
                key={i}
                direction="row"
                spacing={1.5}
                alignItems="center"
                sx={{ px: 2, py: 1 }}
              >
                <Typography
                  variant="caption"
                  sx={{ fontFamily: "monospace", color: "text.secondary", flexShrink: 0 }}
                >
                  {ev.when}
                </Typography>
                <Chip
                  size="small"
                  label={`${ev.glyph} ${ev.type}`}
                  sx={{
                    bgcolor: `${eventColors[ev.type] ?? "#666"}22`,
                    color: eventColors[ev.type] ?? "text.primary",
                    fontWeight: 600,
                    flexShrink: 0,
                  }}
                />
                <Typography variant="body2" noWrap title={ev.text}>
                  {ev.text}
                </Typography>
              </Stack>
            ))}
          </Stack>
        )}
      </Box>
    </Paper>
  );
}
