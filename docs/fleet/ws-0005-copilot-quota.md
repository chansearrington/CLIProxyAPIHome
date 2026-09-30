# ws-0005 — Copilot on Home's quota page

Status: started 2026-09-30

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
