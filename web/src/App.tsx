import Alert from "@mui/material/Alert";
import AppBar from "@mui/material/AppBar";
import Box from "@mui/material/Box";
import Chip from "@mui/material/Chip";
import Container from "@mui/material/Container";
import Grid from "@mui/material/Grid";
import Toolbar from "@mui/material/Toolbar";
import Typography from "@mui/material/Typography";
import CountsGrid from "./components/CountsGrid";
import EventFeed from "./components/EventFeed";
import RankingList from "./components/RankingList";
import { useDashboard } from "./useDashboard";
import { eventColors } from "./theme";

export default function App() {
  const { connected, events, counts, senderRanking, receiverRanking, alertActive } =
    useDashboard();

  return (
    <Box sx={{ minHeight: "100vh", bgcolor: "background.default" }}>
      <AppBar position="static" color="transparent" elevation={0} sx={{ borderBottom: 1, borderColor: "divider" }}>
        <Toolbar>
          <Typography variant="h6" fontWeight={700} sx={{ flexGrow: 1 }}>
            mail-monitor
          </Typography>
          <Chip
            size="small"
            label={connected ? "실시간 연결됨" : "연결 끊김"}
            color={connected ? "success" : "default"}
            variant={connected ? "filled" : "outlined"}
          />
        </Toolbar>
      </AppBar>

      <Container maxWidth="lg" sx={{ py: 3 }}>
        {alertActive && (
          <Alert severity="error" sx={{ mb: 2 }}>
            BOUNCE/REJECT 급증 감지됨
          </Alert>
        )}

        <Box sx={{ mb: 3 }}>
          <CountsGrid counts={counts} />
        </Box>

        <Grid container spacing={2}>
          <Grid item xs={12} md={8}>
            <EventFeed events={events} />
          </Grid>
          <Grid item xs={12} md={4}>
            <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
              <RankingList title="발신 랭킹" entries={senderRanking} color={eventColors.SENT} />
              <RankingList title="수신 랭킹" entries={receiverRanking} color={eventColors.RECV} />
            </Box>
          </Grid>
        </Grid>
      </Container>
    </Box>
  );
}
