# Refresh and execute the upstream fleet upgrade

Status: integration, tests and isolated migration/rollback rehearsals pass; production cutover pending, 2026-11-06 UTC. Chanse authorized rechecking, adjusting the plan and completing the upgrade in chat.

## Updated targets

The November 3 plan is superseded by this execution record.

| Component | Current baseline | Target / action |
| --- | --- | --- |
| Home | custom 1.1.1, deployed source `4cdbc4b`, image `sha256:05509e8110d9229e3ed96897e0e913b90b5e32a3cd62313c094b6d779efe7884` | Merge stable **1.1.8**, tag `454c7fc7eeec8f24834bff93a9bec10811d440f6`, into the fleet fork |
| CPA nodes | all five **8.0.58**, executable SHA `7c0b6f98bef85319d371a3f110c9e904de7896e6259cf1b492d8b62ef7263fa2` | Already latest: Home's canary/serial updater completed the rollout at 13:29 UTC; verify rather than reinstall |
| Copilot plugin | all five **0.3.7**, loaded/reported healthy | Already latest; preserve the existing fork and access fences |
| Embedded panel | fleet Reconnect on the 1.1.1 panel generation | Merge matching upstream HMC `61a6bb34814f5da024fd51f850a6c807e2b6d831` into the panel fork, retaining Reconnect |
| Home SDK / Go | prior SDK 8.0.4 | Use upstream's tested SDK **8.0.53** and required Go **1.26.10**, with all builds on Ark |

The live MacBook/key-6 model menu contained **92** entries before the change, including Opus
5.5, Sol 6.1 and GPT-6 chat thinking. This catalog is saved for post-cutover comparison.

## Integration decisions

- Preserve Home-owned maintenance, scoped mTLS worker commands, quota cooldown lifting,
  hourly blocked/6-hour idle/reset-triggered probes, private account fences, identity-preserving
  plugin OAuth and Reconnect, and SQLite writer/read-pool protections.
- Healthy quota checks must also remain current at their reset/expiry boundary before lifting
  a routing block. Boundary regression tests reject ended observations.
- Reconcile upstream structured stream-truncation classification with the fleet's Claude
  unified quota headers. A combined result constructor preserves both instead of dropping
  one at the Home ingestion boundary; regression coverage verifies the composed metadata.
- Preserve Claude's percentage interpretation and organization-plan precedence. Adopt the
  new reset-grant observations without consuming grants. Valid unknown profiles clear stale
  plan labels, while failed profile reads retain the last verified plan.
- Retain the already installed, equivalent SQLite pending-accounting partial index instead
  of building a duplicate over the entire usage table. A query-plan regression test confirms
  it remains usable. Keep the upstream index for new databases and non-equivalent indexes.
- Keep account-routing strategy, session affinity, keys, priorities, labels and model defaults.
- New billing settings reject unpriced models by default. Explicitly retain this installation's
  previous allow-unpriced behavior during cutover via `PATCH /billing/settings`, preserving
  existing pricing rules, timezone, service-tier source and access boundaries. Do not silently
  turn existing model calls into missing-price errors.

## Validation so far

All Go/Bun/Node work runs on Ark inside the established build wrapper. The official
`golang:1.26-bookworm` image was refreshed from Go 1.26.8 to required Go 1.26.10; no Go or
package toolchain was installed on the laptop. SDK versions stay at the upstream tested pin.

- Full **`go vet ./...` and `go test ./...` pass**, followed by compilation.
- All **727 frontend tests** pass, including Reconnect and delayed-session cancellation.
  Frontend typecheck, lint, formatting and embedded production build pass.
- A detached snapshot of the real SQLite database was migrated with external networking
  disabled. Migration/export took **142.08 seconds** on the isolated physical SSD path.
  Original credential metadata/token fingerprints, UUIDs, channel bindings, key IDs, historical
  usage samples/high-water mark, charge count/total and user balances all matched afterward.
- The previous live image reopened/migrated/exported the newer physical SQLite schema in
  **5.54 seconds**, with the same preservation checks passing. In a separate offline-only
  API exercise, that old image created, patched, read and deleted a fake credential on the
  migrated database while retaining all original account IDs. Plugins/refresh were disabled
  only in that disposable API-test copy to prevent external acquisition.
- SQLite snapshot/schema definitions advance from **6 to 16**, rather than the earlier plan's
  version 12. The measured rehearsal used the physical SSD path; live migration through
  Unraid's shared filesystem may be slower. Allow several minutes of Home unavailability.

## Verified backup

`/mnt/cache/appdata/cpa-home/data/backups/home-pre-118-20261106T1430Z.db`

- **24,946,569,216 bytes**, mode **0600**, full integrity check **ok**.
- SHA-256: `34efadbf0b49f5a5d1cdc0acee2da256a67e94c4fb59d84699a4090f762d0b7f`.
- The source database was read through its normal shared path with SQLite locking; only
  the distinct backup output and offline rehearsal copies used the physical SSD path.
- Never restore this snapshot over later rotating authorizations; the rollback path is the
  old image opening the migrated live database, as rehearsed.

## Remaining execution

Finish independent local PR review, merge both fork changes, build and pin the complete image
and panel manifest, then deploy under the already-authorized ship decision. Pause maintenance
only after its active job finishes; hold all four canonical locks with independent 60-second
heartbeats. Take a final state baseline, retain original compose and immutable rollback image,
apply the billing compatibility setting, and require real Claude/Sol requests for all six keys,
five healthy node/plugin memberships, preserved model menus and private-provider fences,
working Reconnect, fresh xAI quota observations, unchanged identities/bindings/history, and
resumed Home-owned maintenance. Roll back on functional or data-integrity regression.

Private evidence: `/mnt/user/appdata/cpa-home-build/{ws0041-home-118,tmp/ws0041-home-118}/`.
