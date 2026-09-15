#!/bin/sh
set -e

if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload || true
  if systemctl is-active --quiet mail-monitor.service; then
    systemctl try-restart mail-monitor.service || true
  else
    systemctl enable --now mail-monitor.service || true
  fi
fi
