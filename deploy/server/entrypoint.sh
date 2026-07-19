#!/bin/sh
set -eu

# Existing releases ran as root, so an already-populated named volume may be
# root-owned. Repair only the dedicated data volume before dropping privilege.
mkdir -p /data
chown -R rmm:rmm /data

exec su-exec rmm:rmm "$@"
