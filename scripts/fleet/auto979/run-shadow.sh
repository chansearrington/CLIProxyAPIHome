#!/bin/bash
# Intentionally fixed to shadow. Enabling writes requires an operator's separate change.
set +x
set -euo pipefail
umask 077
cd /
set -a
. /mnt/cache/appdata/cpa-home/.env
set +a
exec /usr/bin/python3 -I /mnt/cache/appdata/cpa-home/fleet-jobs/auto979/auto979.py \
  --config /mnt/cache/appdata/cpa-home/fleet-jobs/979.json --shadow
