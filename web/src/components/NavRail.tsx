import Box from "@mui/material/Box";
import Stack from "@mui/material/Stack";
import Tooltip from "@mui/material/Tooltip";
import { accentSignal, monoFont } from "../theme";

export type View = "live" | "ranking" | "history" | "block" | "users" | "forwarding";

// Monospace glyphs instead of stock Material icons — rhymes with the TUI's
// own glyph system (●▼▲↪✕■ for event types) rather than reaching for a
// generic icon set, so the web dashboard reads as the same tool.
interface Item {
  id: View;
  label: string;
  glyph: string;
}

// Grouped into what you watch vs. what you change — a thin divider marks
// the split; the rail itself stays icon-only and fixed-width at every
// breakpoint (a wider labeled sidebar was tried and ate too much of the
// content area on ordinary desktop widths).
const GROUPS: Item[][] = [
  [
    { id: "live", label: "실시간", glyph: "▸" },
    { id: "ranking", label: "발신/수신 랭킹", glyph: "▦" },
    { id: "history", label: "이력 검색", glyph: "⌕" },
  ],
  [
    { id: "block", label: "발신자 차단", glyph: "⊘" },
    { id: "users", label: "사용자 계정", glyph: "@" },
    { id: "forwarding", label: "포워딩 관리", glyph: "↪" },
  ],
];

export default function NavRail({
  view,
  onChange,
}: {
  view: View;
  onChange: (v: View) => void;
}) {
  return (
    <Box
      component="nav"
      sx={{
        width: 56,
        flexShrink: 0,
        borderRight: 1,
        borderColor: "divider",
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
      }}
    >
      <Box
        sx={{
          width: "100%",
          height: 52,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          borderBottom: 1,
          borderColor: "divider",
        }}
      >
        <Box
          sx={{
            width: 26,
            height: 26,
            border: 1,
            borderColor: accentSignal,
            color: accentSignal,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            fontFamily: monoFont,
            fontSize: 14,
            fontWeight: 700,
          }}
        >
          &gt;
        </Box>
      </Box>

      <Stack spacing={0.5} alignItems="center" sx={{ pt: 1.5, width: "100%" }}>
        {GROUPS.map((items, gi) => (
          <Stack key={gi} spacing={0.5} alignItems="center" sx={{ width: "100%" }}>
            {gi > 0 && <Box sx={{ width: 28, borderTop: 1, borderColor: "divider", my: 0.5 }} />}
            {items.map((item) => {
              const active = item.id === view;
              return (
                <Tooltip key={item.id} title={item.label} placement="right">
                  <Box
                    component="button"
                    onClick={() => onChange(item.id)}
                    aria-label={item.label}
                    aria-current={active}
                    sx={{
                      width: 44,
                      height: 44,
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "center",
                      border: "none",
                      cursor: "pointer",
                      bgcolor: active ? "action.selected" : "transparent",
                      color: active ? accentSignal : "text.secondary",
                      borderLeft: 2,
                      borderLeftColor: active ? accentSignal : "transparent",
                      fontFamily: monoFont,
                      fontSize: 18,
                      transition: "color .12s, background-color .12s",
                      "&:hover": { bgcolor: "action.hover", color: "text.primary" },
                      "&:focus-visible": { outline: "2px solid", outlineColor: accentSignal },
                    }}
                  >
                    {item.glyph}
                  </Box>
                </Tooltip>
              );
            })}
          </Stack>
        ))}
      </Stack>
    </Box>
  );
}
