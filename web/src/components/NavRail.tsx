import Box from "@mui/material/Box";
import Stack from "@mui/material/Stack";
import Tooltip from "@mui/material/Tooltip";
import Typography from "@mui/material/Typography";
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

// Grouped into what you watch vs. what you change — six items is enough
// that a flat list stops scanning well, and this split is the one that
// actually matches how the two halves get used (monitoring is glanced at
// continuously; management is a deliberate visit).
const GROUPS: { label: string; items: Item[] }[] = [
  {
    label: "모니터링",
    items: [
      { id: "live", label: "실시간", glyph: "▸" },
      { id: "ranking", label: "발신/수신 랭킹", glyph: "▦" },
      { id: "history", label: "이력 검색", glyph: "⌕" },
    ],
  },
  {
    label: "관리",
    items: [
      { id: "block", label: "발신자 차단", glyph: "⊘" },
      { id: "users", label: "사용자 계정", glyph: "@" },
      { id: "forwarding", label: "포워딩 관리", glyph: "↪" },
    ],
  },
];

const RAIL_WIDTH = { xs: 56, sm: 200 };

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
        width: RAIL_WIDTH,
        flexShrink: 0,
        borderRight: 1,
        borderColor: "divider",
        display: "flex",
        flexDirection: "column",
        alignItems: { xs: "center", sm: "stretch" },
      }}
    >
      <Stack
        direction="row"
        alignItems="center"
        spacing={1.25}
        sx={{
          width: "100%",
          height: 52,
          px: { xs: 0, sm: 2 },
          justifyContent: { xs: "center", sm: "flex-start" },
          borderBottom: 1,
          borderColor: "divider",
        }}
      >
        <Box
          sx={{
            width: 26,
            height: 26,
            flexShrink: 0,
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
      </Stack>

      <Stack sx={{ pt: 1, overflowY: "auto" }} className="thin-scroll">
        {GROUPS.map((group) => (
          <Box key={group.label} sx={{ mb: 1 }}>
            <Typography
              variant="caption"
              sx={{
                display: { xs: "none", sm: "block" },
                px: 2,
                pt: 1,
                pb: 0.5,
                color: "text.disabled",
                fontWeight: 600,
              }}
            >
              {group.label}
            </Typography>
            <Stack spacing={0.25} alignItems={{ xs: "center", sm: "stretch" }}>
              {group.items.map((item) => {
                const active = item.id === view;
                return (
                  <Tooltip
                    key={item.id}
                    title={item.label}
                    placement="right"
                    slotProps={{ popper: { sx: { display: { sm: "none" } } } }}
                  >
                    <Box
                      component="button"
                      onClick={() => onChange(item.id)}
                      aria-label={item.label}
                      aria-current={active}
                      sx={{
                        width: { xs: 44, sm: "100%" },
                        height: { xs: 44, sm: 36 },
                        display: "flex",
                        alignItems: "center",
                        justifyContent: { xs: "center", sm: "flex-start" },
                        gap: 1.25,
                        px: { xs: 0, sm: 2 },
                        border: "none",
                        cursor: "pointer",
                        bgcolor: active ? "action.selected" : "transparent",
                        color: active ? accentSignal : "text.secondary",
                        borderLeft: 2,
                        borderLeftColor: active ? accentSignal : "transparent",
                        transition: "color .12s, background-color .12s",
                        "&:hover": { bgcolor: "action.hover", color: "text.primary" },
                        "&:focus-visible": { outline: "2px solid", outlineColor: accentSignal },
                      }}
                    >
                      <Box component="span" sx={{ fontFamily: monoFont, fontSize: 18, lineHeight: 1, flexShrink: 0 }}>
                        {item.glyph}
                      </Box>
                      <Typography
                        variant="body2"
                        sx={{
                          display: { xs: "none", sm: "block" },
                          fontWeight: active ? 700 : 500,
                          whiteSpace: "nowrap",
                        }}
                      >
                        {item.label}
                      </Typography>
                    </Box>
                  </Tooltip>
                );
              })}
            </Stack>
          </Box>
        ))}
      </Stack>
    </Box>
  );
}
