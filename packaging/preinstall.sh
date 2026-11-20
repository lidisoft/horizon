#!/bin/sh
# The service's system user.
set -e
getent group horizon >/dev/null || groupadd --system horizon
getent passwd horizon >/dev/null ||
	useradd --system --gid horizon --home-dir /nonexistent --no-create-home --shell /usr/sbin/nologin --comment Horizon horizon
