import Box from "@mui/material/Box";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import { EVENT_TYPES } from "../types";
import { eventColors, monoFont } from "../theme";

// A thin data strip, not stat cards — each count is a label:number pair
// separated by hairlines, closer to a status bar than a dashboard tile.
export default function StatusStrip({ counts }: { counts: Record<string, number> }) {
  return (
    <Stack
      direction="row"
      divider={<Box sx={{ width: "1px", bgcolor: "divider" }} />}
      sx={{ border: 1, borderColor: "divider" }}
    >
      {EVENT_TYPES.map((type) => (
        <Stack
          key={type}
          direction="row"
          alignItems="baseline"
          spacing={1}
          sx={{ px: 2, py: 1.25, flex: 1, minWidth: 0, borderTop: 2, borderTopColor: eventColors[type] }}
        >
          <Typography variant="caption" color="text.secondary" sx={{ fontWeight: 600 }}>
            {type}
          </Typography>
          <Typography sx={{ fontFamily: monoFont, fontSize: 20, fontWeight: 500 }}>
            {(counts[type] ?? 0).toLocaleString()}
          </Typography>
        </Stack>
      ))}
    </Stack>
  );
}
