# ws-0048: Home 44a3c27 -> 13860b9 (account-save fix + 503-burst fix)

Status: **DEPLOYED and accepted** 2026-11-13 19:47 CT (window 19:40-20:15 CT, agreed with the Hype Orchestrator).
Ship card `sdgVXYb7Nt` (fleet Inbox), resolved with Chanse's decision in the coordinator pane
(2026-11-13 ~16:15 CT: "Go ahead now, BUT ... have a recoverable approach").

Origin: Hype handed the relay build over on 2026-11-13 (Hype decision 1104, HYPE-6232). Their
handoff and references live in the hype-sports repo under `brain/agents/cpa-relay-handoff/`.

## What ships

| | Tag | Immutable ID |
|---|---|---|
| Running (rollback target) | `cpa-home:1.1.9-claude-fleet-44a3c27` | `sha256:41924ea4eed843db70113a44f48ca5c8dd4370a9520e00df0c877df0522dc51e` |
| Candidate | `cpa-home:1.1.9-claude-fleet-13860b9` | `sha256:0a4c3a7c21e139dc177d70daeb4885db2a9e163fa18de0b04e619f859927c018` |

Fleet `13860b9` = `44a3c27` + two fixes, no schema change:

1. **PR #34 (`2d5fbf1`)**: `PATCH /credentials/fields` applies only the supplied fields to the locked
   current row, so a panel priority/note save no longer writes back a stale sign-in.
2. **PR #35 (`30577e8`)**: the dashboard capabilities readiness probe used SQLite's single writer
   connection with an unordered query (3.36 s on production). Node subscription-liveness refreshes
   queued behind it, missed CPA's 3 s heartbeat deadline, and nodes answered every route with fast
   empty 503s (2026-11-13 bursts at 10:14, 10:26, 10:50, 11:34 CT). The probe now uses the read-only
   pool and `ORDER BY id`. Diagnosis: [ws-0049-503-bursts.md](ws-0049-503-bursts.md). Independent
   review (GPT-6 Astra): ship, no blockers. Bursts 3-4 also coincided with 23-39 s dashboard COUNTs
   on the read pool; that shared-I/O pressure is the ws-0051 usage-aggregate work, not claimed fixed here.

Build: Ark, 37.2 s, from a clean GitHub clone at `13860b9`, pinned panel tree
`53a3fc87…efaf6` (1,060 files) and `models.json` `4a52db3f…37bd0e9` from the accepted ws-0047 build.

## Why Hype's 11-12 test start "never became ready"

Not the build. It published `127.0.0.1:18328` from a Docker `--internal` network and probed from the
host, which cannot reach a port published from an internal network. The ws-0047 method probes from
inside the container's own network namespace. Rerun that way, the exact images reached ready in
65.7 s (2d5fbf1), 58.8 s (2d5fbf1 + fix proof) and **52.9 s (13860b9)** with zero restarts and
panel hashes matching.

## Pre-swap evidence (all passed)

- **Isolated cold start** of the exact candidate on a btrfs reflink clone of the verified 11-10
  snapshot, `--network none` (the clone holds real credentials and must never refresh them),
  read-only root, 64 MiB `/tmp`, production env/mounts shape.
- **Save-fix proof on real data in the clone**: a priority PATCH returned 200 and changed only
  `attributes.priority`, `metadata.priority`, `updated_at`; access/refresh/id tokens, expiry, last
  refresh, status, disabled, unavailable and quota fields byte-identical. Priority `1.5` returned 400
  and changed nothing.
- **Targeted configuration backup** (FLEET.md 2026-11-07 rule; same schema and image-only rollback
  keep live data, so a full 38.8 GB copy would protect nothing): `data/backups/home-config-20261113T222157Z.json`,
  38,152 bytes, SHA-256 `a88c03fa8113e10f1cbc4000dd86465e7317f05001f1800b63036728607a0c3b`. Accounts,
  priorities, 31 bindings, 6 keys, 18 config rows, lifecycle; secrets only as fingerprints. Pruned to two
  config backups (removed 11-09). A full backup started first was stopped at Chanse's direction and its
  partial file and journal deleted; the two verified DB snapshots are untouched.
- **Independent recovery route** (Chanse 2026-11-13): the MacBook's native ChatGPT login is out of
  usage until Nov 18 and the 11-10 `/private/tmp` launcher was wiped by a reboot. New launcher
  `~/.cpa-recovery/claude-direct` reads a healthy Home Claude account's current **access** token
  read-only from the Ark DB file over SSH (works with Home down), holds it only in the process
  environment and starts Claude Code with an isolated config dir and no CPA base URL. It never reads a
  refresh token, so Home stays sole refresh owner. Proof: `DIRECT_RECOVERY_OK`, and `lsof` during a
  request showed only `160.79.104.10:443` (Anthropic), never `127.0.0.1:18317`.
- **Controller dry run** (`swap.py check`, read-only) against live: 5/5 node memberships healthy with
  Copilot 0.3.7 loaded, 20/20 panel assets, protected-state preservation round trip, Docker disk 77.6 GB.

## Controller

Ark-side `ws0048-2d5fbf1/swap.py`: ws-0047's reviewed `home-remote.py` checks (fingerprint
`a6e089c1…`) plus a new self-contained header and orchestration. Launched with `setsid` under four
nested `fleet-lock hold`s (canonical pair); the laptop only polls a terminal marker with a 35-minute
limit. Sequence: pause fleet policy, preflight, compose image-line compare-and-swap, recreate, readiness
(720 s: exact image, 0 restarts, footprint, five memberships + plugin, Fable catalog, panel hashes,
preservation), real request acceptance (>= 3 successful usage rows above the pre-swap high-water,
>= 80 % success, 300 s), 60 s stability, preservation again, no newly disabled account, restore policy
with `patch_updates=false`. Any failure after the swap: disable restart, kill the managed writer,
restore compose bytes, recreate the old image on today's data, re-verify, restore policy.

First independent review found six real defects (stopped-old rejection, crash-loop pause race,
evidence writes blocking recovery, mutation misclassification, account deterioration not gating,
no real-request proof); all fixed in v2. The v2 re-review found three more (pre-swap request rows could
satisfy acceptance, diagnostic writes could still gate readiness/finish, pre-recreate restore could
overwrite foreign compose drift); fixed in v3 (request boundary taken after readiness on both id and
request timestamp; restore only our own change via compare-and-swap). **v3 re-review: no blockers**
(SHA-256 `6f7c9b46d0f2a4c4bdf0d8e4567fa59de2aee838a09e909d63df84430252d190`, identical on the Ark).
Reviewer's retained limit: 3 fresh successes is not all-node/all-provider coverage, so the operator's
native requests and post-swap observation remain part of acceptance. Mac/Ark/NTP clocks agree within 1 s.

## Policy decision

Node auto-patch updates stay **off** after the swap (Chanse 2026-11-13: upgrades must be scheduled and
announced, not silent). Catalog sync and the hourly interval are restored as they were. Follow-up: ws-0050.

## Results

Controller result `DONE` (no rollback), run under all four fleet-locks (acquired 19:40:07, released
19:47:43 CT). Hype was warned at 19:28 CT and held all 22 leads from 19:35 until the 19:50 CT all-clear.

- **Live:** container `29b9d778…`, image `sha256:0a4c3a7c…927c018`, running, 0 restarts, restart policy
  `unless-stopped`. Rollback target is unchanged: `cpa-home:1.1.9-claude-fleet-44a3c27` (`41924ea4…`).
- **Outage:** about 19:40:47 to 19:46:12 CT (325 s to ready, 411 s total). Most of it was not the new code.
  Two startup backfill UPDATEs ran over the whole table and matched zero rows:
  `usage.service_tier` (241.5 s) and `billing_charge.base_amount` (18.8 s). The cold start on the 11-10
  clone did not show this (52.9 s). Follow-up: make these one-time, because every Home restart
  currently pays this cost.
- **Acceptance:** 5/5 node memberships, 20/20 credentials, 20/20 panel assets byte-identical.
  Preservation passed twice: accounts, stable fields, billing settings, usage and charge high-water marks,
  ids and samples. 7/7 fresh requests succeeded above the post-readiness boundary (2 nodes; Claude and
  Codex). 60 s stability passed. No account became newly unavailable.
- **Account health:** identical before and after the swap: Claude 9 active, 3 error, 1 disabled; Codex 2
  active, 2 error. The 16:30 CT baseline was Claude 10/2/1, so one account changed before 19:40. None of
  the three Claude errors needs a sign-in. Two are out of weekly quota until Nov 19-20. The third is
  labeled error only because someone requested the retired `claude-3-7-sonnet` model; it is still
  available.
- **503 fix:** in the first minutes after ready, the logs showed 538 responses of 200 and 2 of 404, zero
  503s, no slow token-accounting readiness query, no heartbeat or subscription loss, no `level=error`.
  The remaining slow SQL is dashboard `COUNT(DISTINCT usage.id)` at 2.8-4.5 s on the read pool (the
  ws-0051 / PR #36 work). The four earlier bursts were hours apart, so one quiet evening is early
  evidence, not proof; re-check after a full day.
- **Native checks from the MacBook through Home:** Haiku x2 (in the driver), Sonnet 5.5, Opus 5.5 and
  Codex all returned their exact reply. Real fleet traffic: 200 usage rows (109 Claude, 91 Codex)
  within minutes.
- **Policy restored:** enabled, catalog sync on, interval 3600 s, the same five nodes, **patch_updates
  false**.
- **Decision 979 (Hype):** no priority changed. The live Claude order is not pure soonest-renewal:
  priority 10 (renews Nov 20) serves ahead of 9 (renews Nov 14). Hype Operations will send the exact order
  to apply, with a targeted backup.
