import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import { EVENT_TYPES } from "../types";
import { eventColors, monoFont } from "../theme";

// A thin data strip, not stat cards — each count is a label:number pair
// separated by hairlines, closer to a status bar than a dashboard tile.
// Built as a CSS grid (1px gap showing the divider color through) rather
// than a Stack so it can reflow to fewer columns on narrow screens instead
// of forcing six items into a horizontal scroll.
export default function StatusStrip({ counts }: { counts: Record<string, number> }) {
  return (
    <Box
      sx={{
        display: "grid",
        gridTemplateColumns: {
          xs: "repeat(2, 1fr)",
          sm: "repeat(3, 1fr)",
          md: `repeat(${EVENT_TYPES.length}, 1fr)`,
        },
        gap: "1px",
        bgcolor: "divider",
        border: 1,
        borderColor: "divider",
      }}
    >
      {EVENT_TYPES.map((type) => (
        <Box
          key={type}
          sx={{
            display: "flex",
            alignItems: "baseline",
            gap: 1,
            px: 2,
            py: 1.25,
            minWidth: 0,
            bgcolor: "background.default",
            borderTop: 2,
            borderTopColor: eventColors[type],
          }}
        >
          <Typography variant="caption" color="text.secondary" sx={{ fontWeight: 600 }}>
            {type}
          </Typography>
          <Typography sx={{ fontFamily: monoFont, fontSize: 20, fontWeight: 500 }}>
            {(counts[type] ?? 0).toLocaleString()}
          </Typography>
        </Box>
      ))}
    </Box>
  );
}
