import MenuItem from "@mui/material/MenuItem";
import Select from "@mui/material/Select";
import type { SelectChangeEvent } from "@mui/material/Select";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import UpdateIcon from "@mui/icons-material/Update";
import { REFRESH_INTERVAL_OPTIONS } from "../useRefreshInterval";
import { monoFont } from "../theme";

export default function RefreshIntervalPicker({
  value,
  onChange,
}: {
  value: number;
  onChange: (ms: number) => void;
}) {
  function handleChange(e: SelectChangeEvent<number>) {
    onChange(Number(e.target.value));
  }

  return (
    <Stack direction="row" spacing={0.75} alignItems="center">
      <UpdateIcon fontSize="small" sx={{ color: "text.disabled" }} />
      <Typography variant="caption" sx={{ color: "text.disabled", flexShrink: 0 }}>
        새로고침
      </Typography>
      <Select
        size="small"
        value={value}
        onChange={handleChange}
        variant="outlined"
        sx={{ fontFamily: monoFont, fontSize: 12.5, height: 30, minWidth: 76 }}
      >
        {REFRESH_INTERVAL_OPTIONS.map((opt) => (
          <MenuItem key={opt.ms} value={opt.ms} sx={{ fontFamily: monoFont, fontSize: 12.5 }}>
            {opt.label}
          </MenuItem>
        ))}
      </Select>
    </Stack>
  );
}
