import { useEffect, useState } from "react";
import Alert from "@mui/material/Alert";
import Button from "@mui/material/Button";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogTitle from "@mui/material/DialogTitle";
import Stack from "@mui/material/Stack";
import TextField from "@mui/material/TextField";
import type { ForwardingEntry } from "../types";

interface Props {
  open: boolean;
  onClose: () => void;
  onSubmit: (source: string, destination: string) => Promise<string | null>;
  editing?: ForwardingEntry | null;
}

export default function ForwardingFormDialog({ open, onClose, onSubmit, editing }: Props) {
  const [source, setSource] = useState("");
  const [destination, setDestination] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) return;
    setSource(editing?.source ?? "");
    setDestination(editing?.destination ?? "");
    setError(null);
  }, [open, editing]);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    const err = await onSubmit(source.trim(), destination.trim());
    setSaving(false);
    if (err) {
      setError(err);
    } else {
      onClose();
    }
  }

  return (
    <Dialog open={open} onClose={onClose} maxWidth="xs" fullWidth>
      <form onSubmit={handleSubmit}>
        <DialogTitle>{editing ? "포워딩 수정" : "포워딩 추가"}</DialogTitle>
        <DialogContent>
          <Stack spacing={2} sx={{ pt: 1 }}>
            {error && <Alert severity="error">{error}</Alert>}
            <TextField
              label="원본 주소 (source)"
              size="small"
              value={source}
              onChange={(e) => setSource(e.target.value)}
              autoFocus
              required
              fullWidth
            />
            <TextField
              label="전달 주소 (destination)"
              size="small"
              value={destination}
              onChange={(e) => setDestination(e.target.value)}
              required
              fullWidth
            />
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={onClose}>취소</Button>
          <Button type="submit" variant="contained" disabled={saving || !source.trim() || !destination.trim()}>
            {editing ? "저장" : "추가"}
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}
