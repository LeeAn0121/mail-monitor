#!/bin/sh
set -e

if command -v systemctl >/dev/null 2>&1; then
  systemctl disable --now mail-monitor.service || true
fi
