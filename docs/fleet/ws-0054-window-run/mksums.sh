#!/bin/bash
# ARCHIVE of the 2026-11-19 window run. driver.sh still holds the two defective checks fixed in b5fe18d
# (footprint digest; broad-mount writer refusal). Regenerate from the current manual; never run this copy.
echo 'ARCHIVE: regenerate from the current ws-0054 manual; do not run' >&2; exit 1
# Build INPUT_SUMS from actual staged files (explicit absolute paths), then check it.
set -eu; umask 077
B=/mnt/cache/appdata/cpa-home-build/ws0054-20261118T2055Z; W=/mnt/cache/appdata/cpa-home-build/ws0054-window-today
HB=$(cat $W/HOT_BASE); T=/mnt/disk18/cpa-home-disaster-backups/$(basename $HB)
OUT=$W/INPUT_SUMS.sha256; test ! -e $OUT
{
for f in reviewed-docs-today/ws-0054-cutover-steps.txt reviewed-docs-today/ws-0054-upgrade-1110-8076.md reviewed-docs-today/ws-0054-upgrade-1110-8076.artifact.html reviewed-docs-today/ws-0054-rehearsal.json \
  panel-manifest.json production-build.json go-checks.json go-focused.log go-full.log postgres-tests.json postgres-tests.log postgres-journal-nested.json postgres-journal-nested.log \
  candidate-embed-proof.json image-identity.json rehearsal.json parallel-seed-integrity.json final-preservation.json cleanup.json schema-before.json schema-after-candidate.json schema-after-old.json \
  index-reference.json clean-index-reference-schema-shaped.json expected-inventory.json awake-nodes.json node-physical-before.txt out/image-home-binary src/internal/registry/models/models.json \
  approved-hot-btrfsfix/ws0054-hot-backup.py approved-hot-btrfsfix/ws0054_hot_backup_test.py; do echo $B/$f; done
find $B/panel/static-full -type f | sort
echo /mnt/cache/appdata/cpa-home-build/ws0048-2d5fbf1/swap.py
echo $W/driver.sh
for f in started.json stdout.json finished.json physical-verified.json; do echo $HB.invocation/$f; done
echo $HB/hot-backup.json; echo $T/hot-backup.json; echo $T/home-hot.db.sha256
} | while read -r f; do test -f "$f" && test ! -L "$f" || { echo "MISSING $f" >&2; exit 1; }; sha256sum "$f"; done > $OUT.tmp
mv -n $OUT.tmp $OUT; chmod 600 $OUT
sha256sum -c --quiet $OUT && echo "INPUT_SUMS OK $(wc -l < $OUT) files"
