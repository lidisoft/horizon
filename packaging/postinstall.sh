#!/bin/sh
# Picks up the unit. A first install isn't started: it needs its
# configuration. An upgrade restarts a running service.
set -e
command -v systemctl >/dev/null || exit 0
systemctl daemon-reload || true
if systemctl is-active --quiet horizon; then
	systemctl restart horizon
else
	echo "Configure /etc/horizon/horizon.env, then: systemctl enable --now horizon"
fi
