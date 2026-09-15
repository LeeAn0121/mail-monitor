import { createTheme } from "@mui/material/styles";

// Event-type accent colors, kept in sync with the TUI's own palette
// (main.go's lipgloss styles) so the web dashboard reads as the same tool.
export const eventColors: Record<string, string> = {
  LOGIN: "#8a8fa3",
  RECV: "#4caf82",
  SENT: "#5b9bd5",
  FWD: "#c98bd9",
  BOUNCE: "#e5484d",
  REJECT: "#e8912d",
};

const theme = createTheme({
  palette: {
    mode: "dark",
    background: {
      default: "#0d1117",
      paper: "#151b23",
    },
    primary: { main: "#5b9bd5" },
    error: { main: "#e5484d" },
    warning: { main: "#e8912d" },
    success: { main: "#4caf82" },
    divider: "rgba(255,255,255,0.08)",
  },
  typography: {
    fontFamily: [
      "Pretendard Variable",
      "-apple-system",
      "BlinkMacSystemFont",
      "system-ui",
      "sans-serif",
    ].join(","),
  },
  shape: { borderRadius: 10 },
  components: {
    MuiPaper: {
      styleOverrides: {
        root: { backgroundImage: "none" },
      },
    },
  },
});

export default theme;
