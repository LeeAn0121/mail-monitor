import Grid from "@mui/material/Grid";
import Paper from "@mui/material/Paper";
import Typography from "@mui/material/Typography";
import { EVENT_TYPES } from "../types";
import { eventColors } from "../theme";

export default function CountsGrid({ counts }: { counts: Record<string, number> }) {
  return (
    <Grid container spacing={1.5}>
      {EVENT_TYPES.map((type) => (
        <Grid item xs={6} sm={4} md={2} key={type}>
          <Paper
            variant="outlined"
            sx={{
              p: 1.5,
              borderColor: "divider",
              borderLeft: 3,
              borderLeftColor: eventColors[type],
            }}
          >
            <Typography variant="caption" color="text.secondary">
              {type}
            </Typography>
            <Typography variant="h5" fontWeight={600}>
              {(counts[type] ?? 0).toLocaleString()}
            </Typography>
          </Paper>
        </Grid>
      ))}
    </Grid>
  );
}
