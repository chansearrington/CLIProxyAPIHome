# ws-0050 research: scheduled, announced node upgrades

Status: research only (2026-11-13, read-only analyst, Sonnet 5.5; condensed by the coordinator).
Owner direction (Chanse, 2026-11-13): no silent node upgrades. When an update is available, Home
schedules it (for example 02:00 CT), tells the agents ahead of time (especially orchestrators), and
announces when it is done. Interim: `patch_updates=false` after the ws-0048 swap.

## How patch rollout works today

- **Available:** `Check()` reads GitHub `releases/latest` for CLIProxyAPI (`internal/fleet/release.go:20,53`),
  skips drafts and prereleases, uses only the `darwin_aarch64` asset, and verifies checksums (`release.go:71-144`).
  The result goes in `State.Latest` (`service.go:514`).
- **No approval for patches:** a same-minor, higher-patch release starts by itself (`service.go:183-191,313-316`)
  once every required node has reported within 5 minutes on the same version (`service.go:297-312`).
  `POST /fleet/releases/:v/approve|hold` is only for minor/major jumps, latched failures or a manual hold.
- **Dispatch:** each node's `ai.openclaw.cpa-home-fleet-worker` polls `GET fleet-plan` over mTLS every 60 s
  (`agent-os/tools/fleet-worker/worker.py:257`, `install.py:55`). `Plan()` hands one fleet-wide job with a
  5-minute lease to the next required node in policy order (`service.go:240-395`). Optional nodes (the
  MacBook) get theirs after the required rollout completes (`service.go:347-354`).
- **Node side:** takes the local fleet-lock (busy means `blocked` and a retry), runs a native probe,
  verifies the archive, runs `upgrade-node-core.sh`. That script runs **`launchctl kickstart -k`
  (`:405`), killing the node with no drain**, then polls `/healthz` for up to 90 s and rolls back on failure.
- **Completion:** `Report()` checks the version, sha and size, then marks the node complete and adds a
  History event (ring of 100) (`service.go:397-472`). A failure queues a reverse rollback and latches the release.
- **`interval_seconds`** (300-86400) only sets how often releases are checked and catalogs resynced. A new
  release can restart a node about `interval + 60 s` after it is published. A policy PUT zeroes `LastCheck`,
  forcing a check within a minute.

## Notification channels

| Channel | Verdict |
|---|---|
| Home `cluster_events` | internal cache bus; fleet never writes it; agents can't read it |
| `GET /fleet` History | only existing record; needs Management key; no "scheduled" kind |
| RESP `fleet-plan` | node certificates only; acting on new fields needs a worker reinstall |
| `internal/usermail` | email verification and password reset only |
| Home webhooks | none exist |
| Fleet Inbox | cards plus a signed hook intake (adapter map, only `vercel-spend`), SSE stream; Mac hook client marked source-only, deployment unverified |
| Hype ask board | realistic path to the Hype Orchestrator (`ask orchestrator "..." --fyi`, local file on the Mac); bridged by the Mac hooks client reading Inbox SSE |
| OpenClaw webhooks | reaches the minis' agents; needs a hook token per mini |
| iMessage (`fleet-oob-notify`) | reaches Chanse, not sessions |

## Smallest design (proposal)

Home (`internal/fleet`):
1. Add `Schedule{version, not_before, announced flags, reason}` to `State` (`types.go:92`). The change is additive JSON, so no schema bump.
2. In `Check()`, when a new patch-allowed release appears, set `not_before` to the next 02:00
   America/Chicago that is at least N hours away.
3. In `Plan()`, give out `upgrade` and `optional_upgrade` jobs only when `now >= not_before`. Rollback stays ungated.
4. `GET /fleet` shows the schedule. Add `POST /fleet/schedule`, `/postpone` and `/now`, and document them in `api.md` and `api_CN.md`.
5. An outbound signed POST notifier for scheduled, reminder, started, done and postponed. It sends after
   the CAS commits (the closure retries up to 12x), and sets the `announced` flags in the same CAS.

agent-os / Hype: an Inbox hook adapter (signature plus field allowlist) and a Mac hooks-client handler
that posts to the Hype ask board. Workers need no change, because the gate is server-side.

Edge cases:
- **The MacBook is the biggest gap.** It is optional, so it upgrades whenever it next polls after the
  rollout. It sleeps overnight, so it upgrades on wake, during the day, with no warning (this matches
  the 2026-11-12 23:34 CT restart). It must get the same gate.
- A required node offline at the scheduled time blocks the whole rollout indefinitely; announce
  "postponed" once and keep waiting.
- Nodes go serially, each down about 90 s or more; the announcement should list the order.
- No postponement exists today (only `hold`). A busy fleet-lock adds a History event per retry and can flush the ring.
- An emergency patch needs `/now`; decide whether `approve` also bypasses the schedule.
- Home's own image upgrades are manual ship cards; reuse the notifier for those announcements.
- The image needs tzdata for America/Chicago (it installs `tzdata`; verify).
- `PUT /fleet/policy` replaces the whole policy, so keep the schedule out of `Policy` or old clients will erase it.

## Pausing only patch updates

Policy lives in `kv_store` key `__home:fleet:state:v1`, field `.policy`. GET `/v8/management/fleet`, copy
`.policy`, and PUT `/v8/management/fleet/policy` with the same body except `"patch_updates":false`. Omitted
fields become zero (catalog sync silently off; interval 0 fails validation). The PUT returns 409 while a
task is active. Re-enabling starts any pending patch within about 2 minutes. `enabled:false` also pauses catalog sync.

Open for Chanse (after the swap): which channel(s) carry the announcements, the default time and lead
time, and whether an orchestrator can postpone.
