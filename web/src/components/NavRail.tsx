import Box from "@mui/material/Box";
import Stack from "@mui/material/Stack";
import Tooltip from "@mui/material/Tooltip";
import ActivityIcon from "@mui/icons-material/GraphicEq";
import HistoryIcon from "@mui/icons-material/ManageSearch";

export type View = "live" | "history";

const ITEMS: { id: View; label: string; icon: React.ReactNode }[] = [
  { id: "live", label: "실시간", icon: <ActivityIcon fontSize="small" /> },
  { id: "history", label: "이력 검색", icon: <HistoryIcon fontSize="small" /> },
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
        width: 64,
        flexShrink: 0,
        borderRight: 1,
        borderColor: "divider",
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        pt: 2,
      }}
    >
      <Stack spacing={0.5} alignItems="center">
        {ITEMS.map((item) => {
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
                  color: active ? "primary.main" : "text.secondary",
                  borderLeft: 2,
                  borderLeftColor: active ? "primary.main" : "transparent",
                  "&:hover": { bgcolor: "action.hover", color: "text.primary" },
                  "&:focus-visible": { outline: "2px solid", outlineColor: "primary.main" },
                }}
              >
                {item.icon}
              </Box>
            </Tooltip>
          );
        })}
      </Stack>
    </Box>
  );
}
