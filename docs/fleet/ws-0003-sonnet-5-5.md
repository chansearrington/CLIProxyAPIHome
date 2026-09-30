# ws-0003 — Sonnet 5.5 across the fleet

Status: started 2026-09-30

- Branch `ws-0003/sonnet-5-5` off `fleet` @ `70aa888`; worktree `.claude/worktrees/ws-0003-sonnet-5-5`
- herdr: `CLIProxyAPIHome | ws-0003 | Sonnet 5.5`, alias `cpahome-ws-0003`

## Goal

Chanse (2026-09-30): "Sonnet 5.5 is released, but I don't see it available as a model here on this
MacBook Pro, and so I'm not sure if it exists anywhere else either." Make Claude Sonnet 5.5
(`claude-sonnet-5-5`) usable everywhere the fleet uses Claude models — or prove exactly why it
cannot be, if the accounts do not have it.

## Where a model can be missing (check every layer, don't assume)

1. **Upstream**: do Chanse's Claude credentials in Home actually get `claude-sonnet-5-5` from
   Anthropic? (A real request through Home's credential, not a docs claim.)
2. **Home's catalog / model registry** (`internal/registry/`, model definitions, any update
   source, model groups, channel groups, excluded models, aliases). agent-os runbook
   `docs/runbooks/cpa-model-currency.md` describes how model currency works — read it.
3. **Each node's `/v1/models`** (4 minis + the MacBook's node, CPA 7.3.16 — re-verify).
4. **Clients**: this MacBook's Claude Code model picker / settings (which uses CPA), and each
   mini's OpenClaw picker (`~/.openclaw/openclaw.json`, a separate hand-kept list — memory
   agent-model-picker-is-separate; `cpa-model-sync` stays paused).
5. Also check whether other recent models are missing the same way (e.g. Fable 5.1
   `claude-fable-5-1`), and fix them in the same pass if the cause is shared.

## Acceptance criteria

1. A layer-by-layer table in this brief: for each layer above, present/absent, with evidence
   (file:line, command output, request result).
2. Root cause stated with evidence, not guessed.
3. Fix live: a real `claude-sonnet-5-5` request succeeds through the MacBook's node AND through
   one mini's node; `/v1/models` on all five nodes lists it; Home's usage attributes it correctly.
4. Selectable in the MacBook's Claude Code and in every mini's OpenClaw picker (validate with
   `openclaw config validate` + `openclaw models list --json`, back up the file first).
5. Any Home code change: tests on the Ark, gofmt/vet clean (known `refresh.go:172` excepted),
   deployed under the ORCHESTRATION.md guards, rollback image recorded.
6. If Sonnet 5.5 is genuinely unavailable on Chanse's accounts, criterion 3-4 become: a clear
   plain-English explanation plus the exact evidence, and nothing half-added to pickers.

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
