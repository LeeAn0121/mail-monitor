import { createTheme, alpha } from "@mui/material/styles";

// mail-monitor's web dashboard mirrors the TUI it sits beside: a dense,
// terminal-native console rather than a generic SaaS panel. The palette
// leans into that — a phosphor-green-tinted charcoal, not the generic
// blue-black of most dark dashboards — and accentSignal (the same green as
// the RECV glyph) doubles as the app's one signature color: primary
// buttons, focus rings, the live-connection dot. Status colors stay
// separate and only ever mean what they mean in the TUI.
export const eventColors: Record<string, string> = {
  LOGIN: "#8b9490",
  RECV: "#3ddc84",
  SENT: "#4ea1ff",
  FWD: "#b58cff",
  BOUNCE: "#ff5c5c",
  REJECT: "#ffb454",
};

export const accentSignal = eventColors.RECV;
export const monoFont = "'JetBrains Mono', 'SFMono-Regular', Consolas, monospace";

const bg = "#0a0f0d";
const panel = "#101613";
const line = "#1c2622";
const textPrimary = "#dbe8e2";
const textSecondary = "#7c928a";

const theme = createTheme({
  palette: {
    mode: "dark",
    background: { default: bg, paper: panel },
    text: { primary: textPrimary, secondary: textSecondary },
    primary: { main: accentSignal },
    error: { main: eventColors.BOUNCE },
    warning: { main: eventColors.REJECT },
    success: { main: accentSignal },
    info: { main: eventColors.SENT },
    divider: line,
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
        outlined: { borderColor: line },
      },
    },
    MuiOutlinedInput: {
      styleOverrides: {
        root: {
          borderRadius: 3,
          "& .MuiOutlinedInput-notchedOutline": { borderColor: line },
          "&:hover .MuiOutlinedInput-notchedOutline": { borderColor: textSecondary },
          "&.Mui-focused .MuiOutlinedInput-notchedOutline": {
            borderColor: accentSignal,
            borderWidth: 1,
          },
        },
      },
    },
    MuiDialog: {
      styleOverrides: {
        paper: { border: `1px solid ${line}`, boxShadow: "none" },
      },
    },
    MuiMenu: {
      styleOverrides: {
        paper: { border: `1px solid ${line}`, boxShadow: "none" },
      },
    },
    MuiChip: {
      styleOverrides: {
        root: { borderRadius: 3 },
      },
    },
    MuiTooltip: {
      styleOverrides: {
        tooltip: { border: `1px solid ${line}`, backgroundColor: panel, color: textPrimary, fontSize: 11.5 },
      },
    },
    MuiIconButton: {
      styleOverrides: {
        root: { borderRadius: 3, "&:hover": { backgroundColor: alpha(accentSignal, 0.1) } },
      },
    },
  },
});

export default theme;
