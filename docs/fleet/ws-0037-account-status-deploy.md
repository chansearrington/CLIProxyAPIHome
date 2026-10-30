# ws-0037: deploy the account-status fix (897d9c6)

Deployed 2026-10-30 by Token Management. Chanse's saved page sfFGFL7vKO (submitted 2026-10-29 21:51 UTC) says,
"Switch to it as soon as it's ready." This superseded the question's proposed after-22:00 Central timing.
Account switching already existed; this delivery corrects misleading account status, not routing capacity.

## Shipped source and checks

- Source commit `897d9c6c4ea79b74b25a0b8bfa191df9ad8aa7ea`, containing the merged status fix:
  https://github.com/chansearrington/CLIProxyAPIHome/pull/22 (merge `7755671`). The other code delta from c632a78
  removes an unreachable return; remaining changes are documentation and tests. No database schema change.
- Ark staging name is ws0036-897d9c6; this document uses ws-0037 because ws-0036 is already assigned to another record.
- All 661 archived source files matched the approved commit byte for byte. All 64 panel assets (3,922,518 bytes)
  matched the running predecessor and were found embedded in the new binary. The tracked one-byte `.gitkeep`
  makes the physical asset directory contain 65 files; it is not a panel asset.
- Ark `docker build` with VERSION=1.1.1 and COMMIT=897d9c6 passed. Image:
  `cpa-home:1.1.1-claude-fleet-897d9c6`, immutable ID
  `sha256:c0bd56efb239136db6f2fad35235b59470e147dda48480d1a24fdae5fe925766`.
- `latestBlockingModelErrorState`: present in new binary, absent in predecessor. `latestModelErrorState`: absent
  in new binary, present in predecessor. `LiftQuotaCooldownObservedAt`: present in both (positive control).
- Docker disk gate: df displayed 87G available before build and 85G after, above the 8 GiB requirement. No image prune.
- No Go test suite was rerun during this deployment; this is a deployment of the already-merged fix, not new Go code.
  Compilation was exercised by the image build. The swap script received an independent Opus review (builder Sol);
  one review attempt ended with an API context-canceled error and did not count as a pass. The final written
  review rechecked the script and required fresh live locks; those conditions were cleared before execution.

## Backup and maintenance coordination

Home policy was paused with no active task/check lease; the exact original policy was saved and later restored.
The fresh backup used SQLite `VACUUM INTO`, not the retrying online-backup API:

- `data/backups/home-pre-897d9c6-20261030T042843Z.db`
- 18,476,654,592 bytes; mode 0600; full `PRAGMA integrity_check` = `ok` at 04:48 UTC.
- SHA256 `c97771dcd3c27247ed819338426857b32642d332c08388d79026a83a77088bdd`.

The SSH connection dropped during the large backup. Its recorded process continued and completed; no duplicate
VACUUM was started. Completion and integrity were read from the resulting receipt before the swap.

The concurrent Agent-OS canvas maintenance window was respected. Token Management yielded and released its locks
while waiting, then used the explicit home-only release slot at 06:36 UTC. A heartbeat had earlier stopped when
Moxy's lock metadata had a blank owner; Agent-OS reclaimed that stale lock using ordinary acquire and released it.
No lock was stolen. All four normal acquisitions and live HELD checks passed again before the actual swap, with
bounded heartbeat renewal. Canvas engine and volume were never changed by this deployment.

## Swap and rollback target

- Reviewed script SHA256 `c6ebf28c7e1054ee1ff9c29efa93371b52354dc28312467d1479bdf19384ec19` was copied to the Ark,
  hash-compared and run detached, so loss of the observing SSH connection could not strand the swap.
- Preconditions checked: current old immutable image, new immutable image, root HTTP200, tools present, home
  service with the expected image and no build/always-pull override, paused policy/no task, verified backup.
- `docker compose up -d home` began 06:39:43.245 UTC; only the home image reference changed.
- New container started 06:39:43.531 UTC; first script readiness confirmation was 06:41:46.474 UTC.
- Root HTTP checks every 0.2 seconds observed non-200 samples from 06:39:43.460 to 06:41:45.379 UTC, about 122 seconds.
  These are sampling bounds, not an assertion about the exact outcome of every in-flight request.
- Script exited 0. Container running, zero restarts. Log retention preserved: json-file, max-size 50m, max-file 10.
- Compose backup: `docker-compose.yml.pre-897d9c6-20261030T063943Z.bak`.
- Retained image-only rollback: `cpa-home:1.1.1-claude-fleet-c632a78`, immutable ID
  `sha256:fe690fc3094b17741ba6e7c96c693006cf99be39f9cff5accdbc15a1f60a4863`.
  Rollback was not needed or exercised. Do not restore an old database over rotating credentials.

## Live acceptance and limits

- Running panel: all 64 served files remained byte-identical after the swap.
- `/nodes`: all five memberships healthy with plugin state `reported_ok`; current heartbeats observed.
- Container logs: no panic/fatal matches since startup; running with zero restarts.
- `/auth-files` joined to saved quota state: enabled quota-healthy accounts returned active, not error.
  Disabled accounts remained disabled. No account reset, re-login or clearance was used to obtain that result.
- Real persisted traffic in [06:41:46Z,06:44:31Z): Codex 172 records = 167 Sol HTTP200,4 Terra HTTP200,1 Sol HTTP502;
  Claude 93 records = 81 Opus HTTP200,12 Sonnet HTTP200. These are saved records, not complete client-side traffic.
  The Sol 502 at 06:43:08.657Z on node dc2db59d remains unclassified; it was not hidden or described as a passed request.
- No synthetic request was sent: this shell did not have a native client key. Actual successful fleet requests
  supplied both-family acceptance. Authenticated visual panel inspection was NOT RUN; account data and assets
  were checked. The exact retired-model regression was not recreated on live accounts; its merged tests cover it.
- Original Home policy restored exactly at 06:45:29Z. All four locks released and heartbeat stopped by 06:46Z.
  Both coordinators received build-done/final-done notices; no deployment build, compose, rsync or waiter remained.
- HYPE-2291 marked Done after verifying the merged fix and live status data. This does not claim to fix network
  outages, account allowance exhaustion, all provider failures or persisted-usage completeness.

Private operational evidence: `/mnt/user/appdata/cpa-home-build/ws0036-897d9c6/` on the Ark, including
`panel-check.json` (pre-build free bytes), `post-swap-check.json` (panel, zero panic/fatal matches, running image),
`backup.json`, `swap.log` and `probe.log`; Token Management's
`.owner/receipts/ws0036-live-acceptance.txt` and `ws0036-swap-review.txt`. No credentials or account identities are
included in this record.
