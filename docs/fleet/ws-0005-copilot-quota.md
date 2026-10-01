# ws-0005 — Copilot on Home's quota page

Status: research done — implementing a Home-side Copilot quota collector (2026-09-30)

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

