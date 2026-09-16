#!/usr/bin/env bash
set -euo pipefail

REPO="LeeAn0121/mail-monitor"

case "$(uname -m)" in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "unsupported arch: $(uname -m)" >&2; exit 1 ;;
esac

# Goes straight through github.com's releases/latest/download redirect
# instead of calling api.github.com/repos/.../releases/latest — the asset
# filename has no version in it specifically so this URL never changes.
# That avoids api.github.com's unauthenticated rate limit, which is easy to
# hit from a shared/NAT IP and shows up as a plain curl 403.
DEB_URL="https://github.com/${REPO}/releases/latest/download/mail-monitor_linux_${ARCH}.deb"

TMP_DEB=$(mktemp --suffix=.deb)
echo "==> downloading ${DEB_URL}"
curl -fsSL "$DEB_URL" -o "$TMP_DEB"

echo "==> installing (sudo required)"
sudo dpkg -i "$TMP_DEB" || sudo apt-get install -f -y
rm -f "$TMP_DEB"

echo "==> done. installed as a systemd service (mail-monitor.service), running now."
echo "    web dashboard: http://localhost:18080"
echo "    TUI: run 'mail-monitor' directly in a terminal"
