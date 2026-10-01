# ws-0003 — Sonnet 5.5 across the fleet

Status: done 2026-10-01 — Sonnet 5.5 was already served everywhere; the gap was Claude Code's own version (fixed on the 4 minis, MacBook already current). No Home change.

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

## Findings (2026-10-01, all times UTC)

### Layer-by-layer

| Layer | Sonnet 5.5 | Fable 5.1 | Evidence |
|---|---|---|---|
| 1. Upstream (Anthropic via Home's Claude credential) | present | present | Real `/v1/messages` through the MacBook node 00:00:46: `claude-sonnet-5-5` answered `ok` (end_turn), `claude-fable-5-1` answered; Home ledger has 538 `claude-sonnet-5-5` rows since 2026-09-29 13:36 and 4,463 `claude-fable-5-1` rows since 2026-09-08, all `provider=claude` |
| 2. Home catalog | present | present | Home refreshes its catalog every 3 h from `internal/registry/model_updater.go:22-25`; both URLs list `claude-sonnet-5-5` and `claude-fable-5-1` under `claude`. (The embedded fallback `internal/registry/models/models.json` lacks them, but it is only used when both fetches fail.) No model/channel group or exclusion hides them — every node serves them. |
| 3. Nodes `/v1/models` | present on 5/5 | present on 5/5 | All five nodes are on CPA **8.0.4** (d33f63f8), not 7.3.16; each lists 103 models incl. both |
| 3b. Real request through a mini | 200 | 200 | Chip agent key, `/v1/chat/completions` 00:01:34 → ledger `100.89.64.86 claude claude-sonnet-5-5 200`; later also Moxy, Lara, Hyper all 200 |
| 4a. MacBook Claude Code `/model` | present | present | Claude Code 2.1.286 picker (captured in a throwaway tmux session): "4. Sonnet — Sonnet 5.5", "3. Fable — Fable 5.1". `claude -p --model sonnet` → ledger `claude-sonnet-5-5` |
| 4b. Minis' OpenClaw pickers | present 4/4 | present 4/4 | `openclaw.json` has both in `models.providers.cpa-gui.models[]` and the allow-gate (edited 2026-09-29 10:33 by an earlier job); `openclaw config validate` passes (only the pre-existing disabled `approval-router` warning); `openclaw models list --json` → 54 models incl. `cpa-gui/claude-sonnet-5-5`, `cpa-gui/claude-fable-5-1` |
| 4c. Minis' Claude Code (WS-548) | **absent → fixed** | present | All four were on Claude Code 2.1.284, whose "Sonnet" means `claude-sonnet-5` |

### Root cause

Not Home, not the nodes, not the accounts. Claude Code's "Sonnet" choice is hard-wired per Claude
Code version: 2.1.285 and older send `claude-sonnet-5`; 2.1.286 (installed on the MacBook by
auto-update 2026-09-30 14:20 local / 21:20 UTC) sends `claude-sonnet-5-5`. Proof: running the old
binary `~/.local/share/claude/versions/2.1.285 -p --model sonnet` produced a ledger row for
`claude-sonnet-5` (00:03:39), while 2.1.286 produced `claude-sonnet-5-5` (00:03:09 run). The
MacBook's hourly ledger shows both models side by side since 21:00 UTC — sessions opened before
the update keep the old binary and its old picker until restarted. Chanse looked from a session
started before the update. The minis had not auto-updated (Claude Code updates only when run).

### Fix (live, 2026-10-01 00:04–00:05)

- `claude update` on Chip, Moxy, Lara, Hyper, one at a time, each under `fleet-lock hold <agent>`:
  2.1.284 → 2.1.286. Rollback = re-point `~/.local/bin/claude` at `versions/2.1.284` (kept).
- Proof per box: `claude -p --model sonnet` → ledger row from that box's IP with
  `claude-sonnet-5-5` HTTP 200 (chip 00:04:32, moxy 00:04:48, lara 00:04:59, hyper 00:05:10);
  agent-key requests for `claude-sonnet-5-5` and `claude-fable-5-1` → 200 on all four.
- No Home code, config, image or node change; no backup/rollback image needed (criterion 5 n/a).
  No picker files edited, so no picker backups taken.

## Close-out

- **Outcome:** Sonnet 5.5 (and Fable 5.1) work everywhere: Anthropic → Home → all five nodes →
  MacBook Claude Code → all four minis' Claude Code and OpenClaw pickers.
- **Acceptance:** 1 table above; 2 root cause above with the 2.1.285-vs-2.1.286 proof; 3 MacBook
  and Chip (and the other three) real requests 200, `/v1/models` lists it on 5/5, ledger
  attributes `provider=claude model=claude-sonnet-5-5`; 4 MacBook picker shows "Sonnet 5.5",
  minis' pickers validated; 5 n/a (no Home change); 6 n/a (available).
- **For Chanse:** in an older Claude Code window, quit and reopen it (or run `/model` in a new
  session) — the "Sonnet" entry will read "Sonnet 5.5".
- **Follow-ups (not done here):**
  - Facts drift: nodes run CPA 8.0.4, not 7.3.16 (memory and FLEET-adjacent notes say 7.3.16).
  - `gpt-6.1-sol` is served by every node but missing from the OpenClaw pickers — picker drift,
    belongs to WS-547's one-tap card, not added here.
  - Claude Code on the minis only updates when someone runs it; consider whether the minis
    should update it on a schedule so the next model bump doesn't repeat this.
