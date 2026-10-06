# ws-0009 — Microsoft Claude account as Chanse-only backup

Status: AC1–4 proven (switch-back at Hype's reset 21:00Z, verified 21:07Z); AC5 (sequential order) check 01:07Z

## Ask (Chanse, 2026-10-06)

Three Claude accounts in Home: (1) `chanse@hypesports.live`, (2) `chanse.arrington@gmail.com`,
(3) `carringt@microsoft.com`. #3 must be usable only by Chanse on his MacBook Pro ("or you can lock
it to just my user"), and only while #1 and #2 are both exhausted; once either resets, traffic goes
back to it and #3 is idle again.

Follow-up ask (20:05Z): use the accounts in order instead of alternating — Hype until it hits
a 5-hour or weekly limit, then Gmail, then Microsoft only if both are out.

## Acceptance criteria

1. #3 is not in channel group 1 (fleet shared); it is in a group bound only to key 6
   (MacBook Pro, user `chanse`). Agent keys 1–5 cannot dispatch to it.
2. #3 has a lower priority than #1 and #2 (`priority: -1`; #1/#2 stay at the default 0).
3. MacBook request succeeds on #3 while #1/#2 are exhausted.
4. After #1's reset (2026-10-06 21:00Z), a MacBook request lands on #1, not #3.
5. With Hype (`priority: 1`) and Gmail (0) both available, Claude traffic goes to Hype only — no
   round-robin to Gmail — and moves to Gmail only while Hype is benched.

## Findings (fleet `a7a270d`, live image `cpa-home:1.0.73-claude-fleet-16f609d`)

- **Priority is strict.** The selection picks the highest-priority bucket that has a ready
  credential (`internal/cliproxy/auth/scheduler.go:983-1010`, order sorted descending at :1161).
  Unset priority = 0 (`selector.go:154`), so `-1` ranks below every default credential, including
  any Claude account added later.
- **Exhausted → benched until the real reset, then back automatically.** A 429 with Anthropic's
  reset hint sets the cooldown to that reset time (`result.go:1210-1244`, horizon 60 days at :23);
  `promoteExpiredLocked` puts the credential back in the ready set on the next pick after it passes
  (`scheduler.go:945-967`). Live `auth.next_retry_after`: Hype `2026-10-06 21:00Z`, Gmail
  `2026-10-07 01:00Z` (both = their 7-day window resets on the quota page).
- **Home uses the built-in scheduler fast path** (`manager.go:1341`): routing is the default
  round-robin, no session affinity, and the only plugin (Copilot) supplies no scheduler
  (`internal/home/plugin_runtime.go:35`). Consequence: the fork's `allowed_warning` de-preference
  (`selector.go:252-335`, commit `bd09641`) is NOT applied on this path, so a warned #1/#2 does not
  hand traffic to #3 early. **Caveat:** if session affinity is ever turned on, Home switches to
  the selector path, where a warned higher-priority account loses to an unwarned lower one — #3
  would then be used ~when #1/#2 near their limits, not only when exhausted.
- **Access is per API key, via channel groups** (`internal/cluster/api_keys.go:944-990`,
  `internal/home/runtime.go:891-911`). Users are an owner/billing layer only. User `chanse` (id 3)
  owns exactly one key, key 6 (MacBook Pro), so "only my user" = "only key 6" today. A new key for
  Chanse would need group 2 added to reach #3.
- **Before state:** #3 (`ca0b50b9…`) was added to group 1 at 2026-10-06 14:25Z by an earlier
  session (backup `data/backups/home-pre-third-claude-routing-20261006T142214Z.db`, integrity ok).
  With #1/#2 exhausted, it was serving the whole fleet. Group 2 "Copilot — Chanse only" holds the
  Copilot credential and is bound only to key 6. No credential had a priority set.

## Change (management API, all reversible)

1. Add #3 to channel group 2; rename group 2 to "Chanse only — Copilot + Microsoft Claude".
2. `PATCH /auth-files/fields` #3 `priority: -1`.
3. Delete channel-group-detail 7 (#3 in group 1).

Rollback: `POST /channel-group-details {channel_group_id:1, auth_id:<#3>}`, delete #3's group-2
detail, `PATCH /auth-files/fields {priority: 0}`, rename group 2 back.

Expected side effect: while #1 and #2 are both exhausted, agent keys 1–5 have no Claude credential
(until Hype resets 2026-10-06 21:00Z / 4:00 PM CDT). That is the requested behaviour.

Step 4 (20:07Z): `PATCH /auth-files/fields` Hype `priority: 1` (before-state in the Ark's
`/tmp/ws0009/before2-auths.json`). Rollback: same PATCH with `priority: 0`. Why it works: equal
priorities share one bucket and the default round-robin alternates inside it; distinct
priorities make each account its own bucket, so the highest ready one always wins and a reset
account is promoted back on the next pick.

## Evidence

- **Applied 2026-10-06 15:44:04Z** under all four fleet-locks (canonical identity, released after;
  all UNLOCKED at 15:44Z). Before-state JSON kept on the Ark in `/tmp/ws0009/before-*.json`.
  Responses: detail 8 created (group 2, #3); group 2 renamed; priority PATCH `ok`; detail 7 delete
  `ok`.
- **State after:** `/auth-files` → #3 `priority=-1`, #1/#2 unset (0). `/channel-group-details` →
  group 1 = antigravity, Hype, codex, xai, Gmail; group 2 = Copilot + #3. Keys 1–5 `channels [1]`,
  key 6 `[1, 2]` (unchanged).
- **AC3 (MacBook on #3):** MacBook key 6 → `claude-sonnet-5-5` HTTP 200 at 15:44:31Z. Ledger since
  15:44:05Z: every Claude row is node `1e3ea42b…` (MacBook, 100.88.81.10) with source
  `carringt@microsoft.com` (5 opus + 5 sonnet, 0 failed).
- **AC1 (agents fenced):** Chip's own `cpa-gui` key (read on the box, not printed) at 15:45Z →
  `claude-sonnet-5-5` HTTP 429 `model_cooldown: All credentials for model claude-sonnet-5-5 are
  cooling down` (only the exhausted #1/#2 are visible to it); same key → `gpt-5.6-sol` HTTP 200.
- **AC2:** priority `-1` confirmed in the listing; behavioural proof is AC4.
- **AC4 (switch-back) — PASS, 21:07Z.** Ledger (read-only SQL on the Ark): 20:30:05–20:59:58Z the
  MacBook's Claude traffic was 606 rows on `carringt@microsoft.com`; the first Hype row is
  21:00:04Z, 4 s after its reset, and every Claude row since (21:00:04–21:07:10Z, 205 rows incl. a
  proof request, HTTP 200) is `chanse@hypesports.live`, 0 failed, **0 on Microsoft**. No restart or
  config change was involved. `auth` at 21:07Z: Hype `active`, no retry time; Gmail `error`,
  unavailable until `2026-10-07 01:00Z`; Microsoft `active` (idle).
- **Step 4 applied 20:07Z** under all four fleet-locks (released, all UNLOCKED): PATCH `ok`;
  listing → Hype 1, Gmail unset, Microsoft -1. MacBook `claude-sonnet-5-5` HTTP 200 after it (still
  on Microsoft, as both others are benched).
- **AC5 (sequential):** pending — scheduled check at 01:07Z (Gmail resets 01:00Z). Resume: same
  helper as below; SQL `select source, count(*) from usage where provider='claude' and timestamp >
  '2026-10-07 01:00:30' group by 1;` Pass = only Hype (unless Hype is benched, then only Gmail).

## Resume after a laptop restart (paused 2026-10-06 19:44Z)

The change is live on the Ark and needs nothing from the laptop; no locks are held. Only the AC4
proof is left. The session-only check (cron) and the laptop's `/tmp/ws0009/ark` helper do not
survive a reboot. The Ark's `/tmp/ws0009/` (mgmt.sh, before-*.json) does.

1. Recreate the helper: a script `exec ssh -o IdentitiesOnly=yes -o IdentityAgent=none
   -i ~/.ssh/cronos_ark root@100.110.133.6 "$@"` (see memory `ark-access-and-build-facts`).
2. If it is after 21:00Z: send one MacBook request (apiKeyHelper key →
   `http://127.0.0.1:18317/v1/messages`, `claude-sonnet-5-5`), then on the Ark run (SQL from a
   file, read-only): `select source, count(*) from usage where
   cpa_node_id='1e3ea42b-169e-474c-9c4e-4527c72654fe' and provider='claude' and timestamp >
   '2026-10-06 21:00:30' group by 1;` Pass = `chanse@hypesports.live`, zero
   `carringt@microsoft.com`. Also read `auth.next_retry_after` for the three Claude auths.
   Before 21:00Z: schedule the same check for 21:07Z.
3. Record the result here, flip the Status line, update the ORCHESTRATION row, commit to `fleet`.
