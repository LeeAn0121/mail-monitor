import { createTheme } from "@mui/material/styles";

// mail-monitor's web dashboard mirrors the TUI it sits beside: a dense,
// terminal-native console rather than a generic SaaS panel. Status color
// carries meaning (per event type) rather than decorating cards, dividers
// stay hairline-thin, and corners stay sharp — the vernacular of a log
// viewer, not a marketing dashboard.
export const eventColors: Record<string, string> = {
  LOGIN: "#7c8894",
  RECV: "#3ddc84",
  SENT: "#4ea1ff",
  FWD: "#b58cff",
  BOUNCE: "#ff5c5c",
  REJECT: "#ffb454",
};

export const monoFont = "'JetBrains Mono', 'SFMono-Regular', Consolas, monospace";

const theme = createTheme({
  palette: {
    mode: "dark",
    background: {
      default: "#0a0e12",
      paper: "#10151b",
    },
    text: {
      primary: "#d6dee5",
      secondary: "#6b7885",
    },
    primary: { main: "#4ea1ff" },
    error: { main: "#ff5c5c" },
    warning: { main: "#ffb454" },
    success: { main: "#3ddc84" },
    divider: "#1e2730",
  },
  shape: { borderRadius: 3 },
  typography: {
    fontFamily: [
      "Pretendard Variable",
      "-apple-system",
      "BlinkMacSystemFont",
      "system-ui",
      "sans-serif",
    ].join(","),
    button: { textTransform: "none", fontWeight: 600 },
  },
  components: {
    MuiPaper: {
      styleOverrides: {
        root: { backgroundImage: "none" },
      },
    },
    MuiButton: {
      styleOverrides: {
        root: { borderRadius: 3 },
      },
    },
  },
});

export default theme;
