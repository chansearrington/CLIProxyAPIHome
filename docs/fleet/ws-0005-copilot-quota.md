# ws-0005 — Copilot on Home's quota page

Status: done 2026-10-01 — live in cpa-home:1.0.73-claude-fleet-16f609d; Copilot on the quota page with real numbers

- Branch `ws-0005/copilot-quota` off `fleet` @ `70aa888`; worktree `.claude/worktrees/ws-0005-copilot-quota`
- herdr: `CLIProxyAPIHome | ws-0005 | Copilot quota`, alias `cpahome-ws-0005`

## Goal

Copilot (plugin provider `copilot`, credential `e3b5f1d8-…`) is missing from Home's quota page
because of a provider allowlist (`quota_snapshots.go`); usage and cost already work. Make the
Copilot allowance visible on the quota page like the other accounts, using what Copilot actually
reports (the plugin or GitHub's Copilot API — find out what quota data exists; don't invent it).

## Acceptance criteria

1. Findings in this brief: where the allowlist is (file:line), what quota data Copilot exposes and
   through which path (plugin / Home / upstream API), and the smallest correct change. If the
   plugin must report quota, coordinate with ws-0006 (same plugin repo) by writing the plugin
   need into this brief and into `docs/fleet/ws-0006-plugin-polish.md` under "Requests from other
   workstreams"; do not edit the plugin concurrently.
2. Home change with tests; generic for plugin providers where that is no harder than
   Copilot-specific. `docs/management/api.md` updated if the Management API changes.
3. Ark: gofmt clean, vet only the known line, full suite green.
4. Deployed under the ORCHESTRATION.md guards (rebase on current `origin/fleet` right before the
   image build — ws-0003 may ship first), tag `cpa-home:1.0.73-claude-fleet-<sha>`, rollback ID
   recorded.
5. Proof: the quota page / its Management API response shows the Copilot credential with real
   numbers; the other providers' quota entries unchanged.

## How to work

- Read `docs/fleet/ORCHESTRATION.md` first; its guards apply to you in full.
- Also read `FLEET.md`, `AGENTS.md`, and the memory notes loaded into your session.
- **No stop points.** Chanse gave full autonomy for this round, including deploys. Decide, act
  under the guards and report results with evidence. Stop for Chanse only if something is
  irreversible AND destroys data or money.
- Use subagents for research and review, and check their work yourself.
- Track your work with the task list (TaskCreate/TaskUpdate) from the start.
- Keep the `Status:` line below current, and commit brief updates to your branch as you go.
- Close-out: outcome, evidence per acceptance criterion, what is live, follow-ups. Then remove your
  worktree/branch once merged, and set Status to `done`.

## Findings (2026-09-30)

**Why Copilot is missing.** Home lists every credential on the quota page, but a credential whose
provider is not on the quota allowlist is returned with `quota_status: unsupported` and never
probed. The allowlist exists twice:
- `internal/cluster/quota_snapshots.go:1814` `quotaProviderPlanned` — `claude, antigravity, codex,
  kimi, xai`. Gates the list/detail status (`:1028`), probe claims (`:650`) and header ingestion
  (`internal/cluster/quota_ingestion.go:173,188,196`).
- `internal/quota/collector.go:570` `quotaProbeEligible` (same five) and the dispatch in
  `probeCredential` (`collector.go:377`).
Live DB before the change: five `quota_snapshot` rows (codex, antigravity, 2x claude, xai), none for
copilot credential `e3b5f1d8-…` (`carringt_microsoft`, `auth_kind=oauth`).

**What quota data Copilot exposes, and where.**
- The plugin: none. It only calls `GET /copilot_internal/v2/token` to swap the GitHub token for a
  short-lived Copilot token (`cliproxyapi-copilot-plugin/internal/provider/token.go:86`). The CPA
  plugin API (`sdk/pluginapi`, v7.2.83 and v7.2.118) has no quota hook at all, and plugins run on the
  nodes, never inside Home — so "ask the plugin" is not a path that exists. No ws-0006 request.
- GitHub: `GET https://api.github.com/copilot_internal/user` with the credential's GitHub OAuth token
  (`metadata.github_access_token`, a `ghu_` token, stored in Home's `auth` row) returns
  `quota_snapshots.{premium_interactions,chat,completions}` with `entitlement`, `remaining`,
  `percent_remaining`, `unlimited`, `overage_permitted`, `overage_count`, plus `quota_reset_date_utc`
  and `copilot_plan`. Called once from the Ark (read-only, token never printed): HTTP 200, plan
  `enterprise` (`copilot_enterprise_seat_quota`), premium requests 1,000,000 / month with 1,000,000
  remaining, chat and completions `unlimited: true`, reset `2026-11-01T00:00:00Z`.
  `Authorization: Bearer <token>` with plain headers also returns 200, so Home's existing probe
  request helper works unchanged.

**Smallest correct change (Home only).** Add a `copilot` active-probe collector next to the other
five: allowlist `copilot` in both places, read the token from `github_access_token`, call
`copilot_internal/user` (URL overridable in `quota.Options` for tests), and map each quota bucket to a
window — premium requests as a monthly fixed window with real limit/used/remaining and the reset date;
unlimited buckets as `is_unlimited` windows; the plan name from `copilot_plan`. A generic
plugin-provider path is not possible without a new CPA plugin-API contract, so this is
Copilot-specific by necessity. The Management API shape does not change (same fields, a new provider
value), so `docs/management/api.md` only needs `copilot` added wherever the provider list is named.


## Implementation and verification

- Commits on `ws-0005/copilot-quota` (rebased onto `origin/fleet` @ `8c2359f`, docs-only move):
  `e2ffbff` feature, `37dc241` test clock fix. PR: chansearrington/CLIProxyAPIHome#1 → `fleet`.
- New `internal/quota/copilot.go` (probe + parser + plan map); `copilot` added to
  `quotaProviderPlanned`, `quotaProbeEligible`, `probeCredential`, `quotaRecollectProviders`;
  `quotaAccessToken` also reads `github_access_token`; `Options.CopilotUserURL` for tests.
- Tests: `internal/quota/providers_copilot_quota_test.go` (parse, fallbacks, end-to-end collector →
  list API with a fake token), `TestCollectQuotaAcceptsCopilotProvider`.
- Ark, `golang:1.26-bookworm`, own clone `/mnt/user/appdata/cpa-home-build/ws-0005-src` @ `37dc241`,
  3 GB exec tmpfs (`ws5-go.sh`, a copy of `run-go.sh` — the shared script still says 1g):
  **gofmt clean; vet = only `internal/cluster/refresh.go:172:2: unreachable code`; full suite 33/33
  packages ok, 0 FAIL; compile ok.** Log: `cpa-home-build/logs/ws5-full.log`.
- Note: scheduled probes only run after recent usage on a credential (`latestQuotaUsageActivityAt`);
  usage rows are tagged for quota only once the provider is on the allowlist, so after deploy the
  first numbers come from a forced `POST /quota/collect` or the next Copilot request.

## Review and merge

- `local-pr-review` (gpt-6-astra via CPA) on PR #1 head `37dc241`: **no issues, must=0 should=0
  nice=0**. `fleet` fast-forwarded to `315c486` (code tree identical to the tested one; only docs from
  ws-0003/ws-0004 had landed). PR #1 shows MERGED.

## Deploy (2026-10-01, all times UTC)

- `origin/fleet` had moved to `16f609d` (ws-0006 docs only); built from it so the image holds
  everything already live.
- All four fleet-locks (`cpa-home-runtime-20260907-root` / `cpa-home-root`) taken via nested
  `fleet-lock hold`; the Ark script ran under nohup (`cpa-home-build/logs/ws5-deploy.log`):
  - 00:13:01 rollback gate: `cpa-home:1.0.73-claude-fleet-95ba2db` = running
    `sha256:d1d6bb3eaed4d15d4bde3dc3af2f0c6ff0e6986439a7f5378050e25a9cd725a1`; Docker disk 99 GB free.
  - 00:14:14 backup `data/backups/home-pre-ws0005-20261001T001301Z.db` (`VACUUM INTO`),
    2,395,164,672 bytes, `quick_check` ok, sha256
    `eace440cfd0988b6cbc3471ab27d35e16ca3ead6dde2c4d17e9239fa0d3758b8`.
  - Runbook Block 1 on the shared `src`: `FULL_SHA=16f609d663afcac1457bb132c049e5183b13a48a`;
    panel `static-full` 64 files / 3,919,143 bytes (removed again after the build).
  - 00:14:51 **built `cpa-home:1.0.73-claude-fleet-16f609d`, ID
    `sha256:7a92e9443088eee7a91461ad623791c8c21277c5aebec6a0de5c10897042981e`, 185,519,600 bytes.**
    Binary markers new/running: `copilot_internal/user` 1/0, `pluginDiscoveryAuth` 2, full SHA 3.
  - 00:14:52 swap (compose backup `docker-compose.yml.pre-ws0005-20261001T001301Z.bak`, one
    `image:` line); 00:14:58 management port up, restarts 0.
- **Rollback = compose back to `cpa-home:1.0.73-claude-fleet-95ba2db` (ID `sha256:d1d6bb3e…`)
  after re-verifying its ID, then `docker compose up -d home`.**

**Incident: my wait loop held the locks for ~46 min after the deploy finished.** The laptop driver
polled the Ark with `ssh ark '…; pgrep -f ws5-deploy.sh …'`. The remote shell's own command line
contains `ws5-deploy.sh`, so `pgrep -f` always matched itself and reported RUNNING. The deploy was
DONE at 00:14:58; the locks stayed held until ~01:02 and blocked ws-0006. The orchestrator (pane
w1:p1) ran the remaining proof steps while the locks were still held, then killed the driver (exit 143,
locks released). **Lesson: never detect a remote job with `pgrep -f <name>` from inside a shell
whose command line carries that name. Poll for a terminal marker in the log (`DONE` / `ABORT`) or a
pid file instead, and put a time limit on any loop that holds a fleet-lock.**

## Proof

- Orchestrator, ~01:02 (locks held): proof script = chip, moxy, lara, hyper agent keys 200 `'ok'`
  (`claude-sonnet-5-5` through each mini's own `cpa-gui` path); MacBook `claude-sonnet-5-5` 200 `'ok'`;
  MacBook `copilot/gpt-5-mini` 200 with empty content; `POST /quota/collect {"providers":["copilot"]}`
  202.
- Re-checked by me at ~01:05: the empty gpt-5-mini answer is `finish_reason: length` with all 16
  `max_tokens` spent as `reasoning_tokens` (the proof's cap, not a defect); with `max_tokens: 400` the
  same request returns `'ok'`, `finish_reason: stop`.
- `GET /v0/management/quota/credentials`: before = copilot `e3b5f1d8…` `unsupported`/`unsupported`/
  `never`, 0 windows. **After = `healthy` / `success` / `fresh`, source `active_probe`, plan
  `Enterprise`, 3 windows; primary `copilot-premium-interactions` limit 1,000,000 remaining
  1,000,000 reset 2026-11-01T00:00:00Z, and `copilot-chat` unlimited** (`copilot-completions`
  unlimited too). These match what GitHub returned for the seat. The other five credentials (canary,
  antigravity, 2x claude, codex, xai) are unchanged in ID, status, source, plan and window count; only
  live usage percentages moved.
- Home since the swap (to ~01:05): 1,374 requests, 5 failed, all HTTP 499 (client closed) from
  100.88.81.10 on Claude models: client cancels, not Home errors. 0 panics in the logs. 3 Copilot
  usage rows already carry `quota_credential_id`, so scheduled probes keep Copilot fresh without a
  forced collect.

## Close-out

- **Outcome:** the Copilot seat is on Home's quota page with GitHub's real allowance (Enterprise,
  1,000,000 premium requests/month, chat and completions unlimited, reset date), refreshed by
  Home's normal quota collector. Nothing changed for the other providers.
- **Acceptance:** 1 Findings above (allowlist at `quota_snapshots.go:1814`/`collector.go:570`, source
  = GitHub `copilot_internal/user`, plugin has no quota path, no ws-0006 request needed) ✅ ·
  2 Home change + 4 tests, API docs (EN + CN) updated; Copilot-specific because a generic plugin path
  does not exist ✅ · 3 Ark gofmt clean, vet only `refresh.go:172`, 33/33 packages ok ✅ · 4 deployed
  under the guards, `cpa-home:1.0.73-claude-fleet-16f609d`, rollback ID recorded ✅ (lock-hold overrun
  above) · 5 proof above ✅.
- **Live:** Home `cpa-home:1.0.73-claude-fleet-16f609d` (`fleet` @ `16f609d`).
- **Follow-ups:**
  - Upstream: the collector is generic enough to offer upstream once Copilot becomes a supported
    plugin there; not opened (fleet-only provider today).
  - ~~Shared `run-go.sh` 1 GB `/tmp`~~ — already fixed by ws-0004 the same evening (3 GB, backup
    `run-go.sh.bak-20260930`; orchestrator verified on the Ark 2026-10-01).
  - Copilot overage (`overage_permitted`, `overage_count`) is not shown; add it as a window only if
    the seat ever runs past its allowance.
- **Cleanup:** Ark `ws-0005-src`, `ws5-go.sh`, `ws5-deploy.sh`, `ws5-quota.py` removed (deploy and
  test logs kept in `cpa-home-build/logs/`); worktree and branch `ws-0005/copilot-quota` removed
  (local + origin).
