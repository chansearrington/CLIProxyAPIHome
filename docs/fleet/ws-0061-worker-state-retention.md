# Worker state retention working plan

Goal: prevent normal worker task receipts and release downloads from accumulating, without affecting replay, updates or rollback. PR only; no rollout or remote commands.

Scope: worker.py run caller and new cleanup helper; test_worker.py temp-directory checks; worker README. Home wire format, installer, settings, catalog sync and shell updater do not change.

Acceptance:
- Clean only top-level versioned release archives and terminal UUID receipts.
- Keep two newest archives, installed/manifest rollback/accepted versions, and the offered job's before/target versions.
- Preserve current Home job regardless of age. Home Plan returns its active job even after expiry; disabled plans hide it, so skip cleanup when disabled or unavailable.
- Keep terminal receipts for seven days: a full week for diagnosis and normal report retries. The current Home job is protected beyond that window. Unknown, malformed and running receipts stay.
- Never recurse or touch settings, latest/accepted records, locks, catalog backups, historical workstream directories, staged binaries or service-root backups.
- Bound deletions per poll, redact cleanup errors, continue normal polling on cleanup failure, verify repeat cleanup.
- Run all fleet-worker test modules, including suffix-named tests, sequentially under python3 and /usr/bin/python3 with -I.

Sequence: trace protocol and rollback files; implement and test cleanup; run both suites; inspect diff; commit/push/open fleet PR; report evidence to lead. No independent parallel slices are needed for this small shared-file change.
