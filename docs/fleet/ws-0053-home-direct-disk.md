# ws-0053: Home's database moved off the Unraid share layer (emergency, 2026-11-15)

Status: **DEPLOYED** 2026-11-15 20:52 CT. The same image (`13860b9`) and the same files; only the bind path changed.

## What went wrong

From 2026-11-14 03:15 CT on, the fleet saw repeated relay bursts. The relay answered instant empty 503s or
`home_unavailable: home dispatch bundle unavailable`, and its log showed "home config subscription lifetime
ended". On 11-15 Hype counted about 10,900 of 112,900 message attempts refused (9.6 %). One run on 11-15
lasted 3 h 19 min (04:56-08:15 CT; it ended when the Ark rebooted at about 08:06).

## Cause (measured 2026-11-15 19:31-20:20 CT)

- Compose bound `./data` from `/mnt/user/appdata/cpa-home`. `/mnt/user` is Unraid's FUSE share layer
  (`shfs`), so every SQLite page read and write went through FUSE.
- `shfs` ran at 200-290 % CPU. Load came from the Unraid **mover**, which is scheduled hourly
  (`0 */1 * * *`) and runs 1-2.5 h at a time (11-15: 14:00-15:50, 16:00-16:54, 17:00-19:40, 20:00 on), plus two
  Duplicacy `check` jobs and a backup over `/mnt/user`, plus Plex.
- Inside Home, trivial primary-key lookups (`api_key`, `kv_store`, `cpa_node_membership`, even
  `SELECT CURRENT_TIMESTAMP`) took 0.2-2.1 s: 8,861 slow-SQL lines in one hour and 24,046 in six. Subscription-liveness
  refreshes failed (`context canceled`). CPA's 3 s heartbeat lapsed and nodes refused requests. That is
  the same failure path as ws-0049 (writer starvation); this time the trigger was host I/O.
- Every burst Hype reported on 11-15 afternoon fell inside a mover run. After the mover was stopped at 20:14, the
  2-minute window showed 0 liveness failures and 0 503s, but `shfs` stayed busy (Duplicacy, Plex), and Hype still
  saw short bursts at 20:16, 20:18 and 20:21. So the layer, not only the mover, was the bottleneck.

## Fix

The `appdata` share is `shareUseCache="only"` on pool `cache`, so `/mnt/cache/appdata/cpa-home` holds exactly the
same files with no FUSE in between (verified: no copy exists on any `/mnt/disk*`). The four compose binds changed from
`./x` to `/mnt/cache/appdata/cpa-home/x` (cluster.yaml, data, logs, plugins).

The controller (`cpa-home-build/ws0053/rebind.py`) ran under all four fleet-locks. The first attempt at 20:45 was
blocked by Hyper's lock (a Home catalog task) before it changed anything; the retry ran at 20:45:29. Steps:
pause the fleet policy, back up compose (`docker-compose.yml.pre-ws0053-*.bak`), edit and validate it,
`docker compose up -d --no-deps home`, verify the new mounts, wait until 5/5 nodes are healthy plus 60 s
stability, then restore the policy (read back: enabled, `patch_updates=false`). It would have restored the old
compose automatically on failure. Result `DONE` in 396 s. Relay outage about 20:45:35-20:51 CT.

Just after: 0 liveness failures, 0 "membership not active", 0 slow SQL, 0 503s, 0 errors, 5/5 nodes. `shfs` CPU 0 %.

Rollback: copy the `.pre-ws0053` compose back and `docker compose up -d --no-deps home`.

## Follow-on changes

- `~/.cpa-recovery/claude-direct` now reads `/mnt/cache/appdata/cpa-home/data/home.db`. Proven again
  (`DIRECT_RECOVERY_OK`). Every reader of the live DB must use the same `/mnt/cache` path as the writer.
  Mixing a FUSE path with a direct path for SQLite WAL shared memory is unsafe.
- The ws-0048 targeted-backup script used by decision-979 reorders reads via `/mnt/cache`.
- The mover was stopped once by hand (20:14). Its schedule was left unchanged; it runs hourly again.

## Open

- **Verified under load:** 21:00-21:15 CT with the hourly mover running: 0 liveness failures, 0 "not active",
  0 503s, 3,429 200s; `shfs` at 9 % CPU; only 15 slow-SQL lines, all dashboard `COUNT(DISTINCT usage.id)`
  at 0.66-0.74 s (they took about 10 s through FUSE). Hype Operations' relay-side counts for 21:00-22:00 are pending.
- The ws-0048 "Results" claim that 13860b9 fixed the bursts was premature: the capabilities probe was one
  writer-starvation trigger and host I/O through `shfs` was another.
