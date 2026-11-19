# ws-0054 — Home 1.1.9-fleet-7091c86 deploy record (2026-11-19)

**Result:** deployed and accepted. Home runs `cpa-home:1.1.9-claude-fleet-7091c86`
(`sha256:43bac183f39e9324a91fe80a123b72e0bd5d5be4b103c45f3f56dda87cf35bf8`). Nodes stay on CPA 8.0.69.
Image-only rollback target: `cpa-home:1.1.9-claude-fleet-13860b9` (`sha256:0a4c3a7c…927c018`).

## Contents

- PR #36: whole-day usage dashboard indexes (`idx_usage_dashboard_time_v1`, `idx_usage_dashboard_provider_time_v1`).
- Corrected PR #38: sparse `idx_usage_service_tier_pending_v1` and `idx_billing_charge_base_amount_pending_v1`;
  the two startup backfills probe these instead of scanning whole tables. Schema stays 17; no fork schema numbers.
- PR #43: the status toggle mutates the locked latest credential record, so a concurrent refresh can no longer
  make a disable silently disappear.

## Timeline (2026-11-19, Central)

| Time | Event |
|---|---|
| ~09:20 | Chanse: do the upgrade today. Hype Orchestrator told; it OK'd 10:30 CT or when the backup verifies. |
| 09:44 | First HOT backup attempt rejected immediately: `wrong_cache_mount` (btrfs mountinfo `0:43` vs subvolume `st_dev` 44). Helper fixed (`a05d014`). |
| 09:45–10:36 | HOT backup: copy 112 s, FULL integrity 491 s, cross-device copy 1,183 s, physical SHA re-verify. PASS. |
| 09:50–10:30 | Live rehearsal of the native acceptance tool against the running Home found three blockers (unset `${CPA_NODE_KEY}` entry; `anthropic-version` header hides GPT models in `/v1/models`; Claude-only MacBook has no OpenClaw providers). Fixed (`4da4c5f`); 10/10 native requests then passed. |
| 10:38 | Run started under the four fleet-lock holds. Inputs (1,100 files) PASS, policy paused, baseline PASS. |
| 10:39–10:48 | HOT re-verify PASS; IO advisory: mover idle, parity idle; old Home stopped; COLD reflink PASS; new container created. |
| 10:48–10:56 | First start built the four indexes (billing sparse 47 s; usage indexes the rest; migration completed 10:56:41). Home served from 10:56:43. |
| 10:56–11:03 | Readiness kept failing on a footprint digest comparison with the old container (deleted by recreate). |
| 11:03 | Readiness deadline (900 s) → route B. Annex R stopped the healthy new Home, then refused to start the old image: `binhex-krusader` mounts `/mnt` RW and `claude-code` mounts `/` RW. Home down. |
| 11:45 | Chanse started the container by hand. Migrations completed in about 1 s; Home served on 7091c86. |
| 11:46–11:52 | Restart policy restored to `unless-stopped`; saved fleet policy restored exactly (`patch_updates` false); acceptance below; all-clear to Hype. |

Relay impact: 10:48–10:56 planned; 11:03–11:45 unplanned (Operations' alarm: 1,367 of 1,436 calls failed
11:03–11:34). Several Hype lead sessions exited on provider errors.

## Acceptance (after manual restart)

- `verify_home ready` and `stable`: PASS. These cover:
  - the exact reviewed after-schema, with all four indexes;
  - every account, key, setting and history hash matching the pre-stop baseline;
  - all five nodes healthy with Copilot 0.3.7 `reported_ok`;
  - 1,058 panel routes byte-identical and the two non-routes returning 404.

  Two asserts were adapted for this post-recovery run only:
  - the container footprint digest, because the old container no longer existed; the live compose file was confirmed byte-identical to the reviewed candidate, which differs from the old one only in its image line;
  - `policy.enabled` must be false, because the policy had already been restored.
- Native acceptance (`ws0054-acceptance.py`) plus `verify_native`: PASS. Claude Haiku 5.5 and GPT 6.2 Sol ran through
  all five nodes, 10 requests, HTTP 200, usage joined to active credentials, and 60 s stability.
- Coordinator catalog: both GPT 6.2 models present within 10 min.
- COLD copy `home-cold-ws0054-20261119T164721Z.db`: quick check, SHA and FULL integrity all PASS.
- The last 300 usage rows after recovery: 0 failed.

## Backups (two newest verified kept)

- HOT: `/mnt/disk18/cpa-home-disaster-backups/ws0054-hot-20261119T154551Z/home-hot.db`,
  46,233,739,264 B, FULL integrity ok, SHA matched on cache and array.
- COLD: `/mnt/cache/appdata/cpa-home/data/backups/home-cold-ws0054-20261119T164721Z.db`, 46,286,286,848 B.
- Removed: `home-pre-118-20261106T1430Z.db`, `home-pre-119-20261110T184515Z.db`, and the COLD copy's empty WAL/SHM sidecars.

## Defects found in the live window and fixed (PR #48, `b5fe18d`)

1. **Readiness compared a full container footprint with the old container.** Recreate deletes that container,
   and the digest never matched a compose-made one, so a healthy Home was failed. Readiness now compares the
   live container with the reviewed compose file and names any differing field without printing values:
   - image, command, entrypoint, user, working dir and environment;
   - privileged, capabilities and security options;
   - read-only root, tmpfs, restart, network and logging;
   - ports and the exact mount set (type, source, target, RW).

   It matched the live 7091c86 container exactly. It also caught deliberately altered copies: privileged,
   an added capability, an extra `/` mount, a writable `cluster.yaml` and an extra environment value.
2. **The old-image recovery's other-writer check refused any container with a broad writable host mount.**
   The Ark always runs two: `binhex-krusader` (`/mnt`) and `claude-code` (`/`). The fix has two parts:
   - It still refuses writable mounts of the data dir, anything inside it, or Home's app folder.
   - Containers with a writable ancestor mount are **paused** from before the `fuser` proof until the old
     image has started, then unpaused in a `finally`. Today that would pause exactly those two.

The practice runs used a clone and never exercised route B against production's other containers. **Lesson:**
before trusting an automatic rollback, dry-run its fence checks against the live host.

## Scripts actually used

`docs/fleet/ws-0054-window-run/`:
- `driver.sh`: the reviewed manual blocks, verbatim, chained by the NEXT latch and run detached on the Ark;
- `local-run.sh`: the laptop side run under the fleet locks, including Annex N verbatim;
- `mksums.sh`: the INPUT_SUMS builder.

They ran detached because the operator's own inference goes through Home and is cut off during the outage.
