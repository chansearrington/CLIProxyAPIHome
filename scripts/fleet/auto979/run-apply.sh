#!/bin/bash
# Apply mode (Chanse decision 979; Hype Orchestrator OK 2026-11-19). Writes only Claude priorities,
# backup first, read-back, STOP latch; refuses to write while Home's fleet policy is paused.
set +x
set -euo pipefail
umask 077
cd /
exec /usr/bin/python3 -I /mnt/cache/appdata/cpa-home/fleet-jobs/auto979/auto979.py \
  --config /mnt/cache/appdata/cpa-home/fleet-jobs/979.json --apply
