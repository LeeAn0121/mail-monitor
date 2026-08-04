#!/usr/bin/env bash
set -euo pipefail

REPO="LeeAn0121/mail-monitor"

case "$(uname -m)" in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "unsupported arch: $(uname -m)" >&2; exit 1 ;;
esac

echo "==> fetching latest release info"
API_URL="https://api.github.com/repos/${REPO}/releases/latest"
DEB_URL=$(curl -fsSL "$API_URL" | grep -o "\"browser_download_url\": *\"[^\"]*_linux_${ARCH}\.deb\"" | sed -E 's/.*"(https[^"]+)"/\1/')

if [ -z "$DEB_URL" ]; then
  echo "could not find a .deb asset for arch ${ARCH} in latest release" >&2
  exit 1
fi

TMP_DEB=$(mktemp --suffix=.deb)
echo "==> downloading ${DEB_URL}"
curl -fsSL "$DEB_URL" -o "$TMP_DEB"

echo "==> installing (sudo required)"
sudo dpkg -i "$TMP_DEB" || sudo apt-get install -f -y
rm -f "$TMP_DEB"

echo "==> done. run: mail-monitor"
echo "    (needs read access to /var/log/mail.log — see README for sudoers setup)"
