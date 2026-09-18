import { useEffect, useState } from "react";
import Alert from "@mui/material/Alert";
import Button from "@mui/material/Button";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogTitle from "@mui/material/DialogTitle";
import Stack from "@mui/material/Stack";
import TextField from "@mui/material/TextField";
import type { DirectoryUser } from "../types";

interface Props {
  open: boolean;
  onClose: () => void;
  onSubmit: (email: string, password: string, name: string) => Promise<string | null>;
  /** Present for edit mode (email locked); absent for create mode. */
  editing?: DirectoryUser | null;
}

// One dialog handles both create and edit — email is only editable in
// create mode (it's the table's key; changing it elsewhere would be a
// delete-and-recreate, not an update, since every other table here
// references users by email).
export default function UserFormDialog({ open, onClose, onSubmit, editing }: Props) {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [name, setName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) return;
    setEmail(editing?.email ?? "");
    setPassword(editing?.password ?? "");
    setName(editing?.name ?? "");
    setError(null);
  }, [open, editing]);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    const err = await onSubmit(email.trim(), password, name.trim());
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
        <DialogTitle>{editing ? "계정 수정" : "새 계정 등록"}</DialogTitle>
        <DialogContent>
          <Stack spacing={2} sx={{ pt: 1 }}>
            {error && <Alert severity="error">{error}</Alert>}
            <TextField
              label="이메일"
              size="small"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              disabled={!!editing}
              autoFocus={!editing}
              required
              fullWidth
            />
            <TextField
              label="비밀번호"
              size="small"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoFocus={!!editing}
              required
              fullWidth
              helperText="이 테이블은 평문으로 저장됩니다."
            />
            <TextField
              label="이름"
              size="small"
              value={name}
              onChange={(e) => setName(e.target.value)}
              fullWidth
            />
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={onClose}>취소</Button>
          <Button type="submit" variant="contained" disabled={saving || !email.trim() || !password}>
            {editing ? "저장" : "등록"}
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}
