#!/bin/sh
# Forgets the removed unit.
set -e
command -v systemctl >/dev/null || exit 0
systemctl daemon-reload || true
