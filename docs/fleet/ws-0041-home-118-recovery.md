# Home 1.1.8 upgrade and startup recovery

Recovered November 6, 2026. The previous conversation ended during cutover before
readiness or acceptance was established. The first candidate attempt timed out and
its automatic rollback also failed to start.

## Cause and durable correction

The production Compose service mounts `/tmp` as a 64 MB tmpfs and makes the root
filesystem read-only. SQLite exhausted that temporary workspace while building
billing indexes, reporting `database or disk is full (13)` despite ample free space
on the data volume. Both candidate and rollback entered startup failure loops.
The rehearsal had not reproduced this production temporary-storage limit.

Stopped the still-running first deployment controller before recovery. Retained the
live database and rotating authorizations. Created `data/sqlite-tmp` and persisted
both `SQLITE_TMPDIR` and `TMPDIR` as `/CLIProxyAPIHome/data/sqlite-tmp` in Ark's
`/mnt/user/appdata/cpa-home/docker-compose.yml`. The temporary workspace therefore
uses the existing writable data bind mount. The 64 MB `/tmp` mount remains in place.
Keep these environment settings in future deployments and rollback configurations.

First restored the previous image, allowing its pending indexes and usage scans to
finish. Then completed the authorized candidate cutover using a new Compose backup
that includes the temporary-storage fix. The final candidate became ready at
16:24:00 UTC, approximately 3 minutes 50 seconds after its start. Database startup
scans take minutes on this large SQLite database even after index creation finishes;
container `running` status alone is insufficient evidence of readiness.

## Accepted deployment

- Home image: `cpa-home:1.1.8-claude-fleet-87210da`.
- Immutable image ID: `sha256:75ede4deb4e4d4cbe2f0a33b456dfbf57c55792ba9018d4ea04bf1df40fd6283`.
- Backend commit: `87210dae574bc7fabe8f2d751cf48a1d5cfb20f8`.
- Panel commit: `2d40bd5ae60c31068122b8ce7665f37bc2b9d446`.
- Final container start: 16:20:10 UTC; zero restarts at acceptance.
- All 18 account identities/priorities, channel bindings, original six API keys,
  inference configuration, prices, sampled usage and billing charges preserved.
- Billing compatibility explicitly retains `allow_unpriced_models: true`.
- All six original keys passed native Claude and Codex requests through the MacBook.
  Moxy, Hyper, Lara and Chip each passed a native Claude request through their own
  loopback listener. All five memberships were healthy with `reported_ok` plugins.
- Restarted the MacBook node supervisor after its reconnect attempt returned
  `membership_takeover_unavailable`; its membership and live requests recovered.
- Home landing, management and user pages plus all 17 directly referenced assets
  returned HTTP 200. These are HTTP asset checks, not a browser interaction test.
- Restored the exact saved fleet maintenance policy after acceptance. Signaled the
  inherited four-lock guard to release its locks. Stopped the isolated API rehearsal
  container; retained rehearsal databases, backup and recovery evidence.

No node binary upgrades were performed during this recovery. The prior conversation
had established node 8.0.58 and Copilot 0.3.7 as its reviewed targets/current versions.

Private Ark evidence: `/mnt/user/appdata/cpa-home-build/ws0041-home-118/deploy/`,
including `baseline-first-attempt.json`, `swap-result.json`,
`recovery-panel-proof.json` and `recovery-final.json`. Native request evidence is
`/tmp/cpa-upgrade-20261106/recovery-native-proof.json` on the MacBook. Existing
backup/rehearsal verification is retained in the parent Ark directory. No secrets
are included in this document.
