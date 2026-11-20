#!/bin/sh
# Stops the service on removal, not on upgrade (deb: "upgrade"; rpm: $1 is 1).
set -e
case "$1" in upgrade | 1) exit 0 ;; esac
command -v systemctl >/dev/null || exit 0
systemctl disable --now horizon || true
