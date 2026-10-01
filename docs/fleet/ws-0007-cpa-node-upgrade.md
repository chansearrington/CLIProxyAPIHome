# ws-0007 — CPA nodes 8.0.4 → 8.0.5+

Status: started 2026-10-01

- Branch `ws-0007/cpa-node-upgrade` off `fleet`; worktree `.claude/worktrees/ws-0007-cpa-node-upgrade`
- herdr: `CLIProxyAPIHome | ws-0007 | CPA node upgrade`, alias `cpahome-ws-0007`

## Goal

All five nodes (Chip, Moxy, Lara, Hyper minis + this MacBook Pro) run the newest suitable CPA
release (8.0.5 or later), which contains the fix for the plugin "installed but not loaded" loop
(router-for-me/CLIProxyAPI#6225, fix `bfa5aed`). Chanse asked for this on 2026-10-01.

## Facts at start (verify, they go stale)

- Nodes run CPA **8.0.4** (`d33f63f8`) darwin/arm64 — verified on the MacBook node and by ws-0003 on all five.
- Node upgrades are done by agent-os `control-plane/cpa-home/upgrade-node-core.sh` (parameterised
  since WS-547 T1); procedure in agent-os `docs/runbooks/cpa-model-currency.md` (lines ~120-140:
  `RESULT=` lines, exit codes, `.bak` restore). The 7.2.159 → 7.3.16 upgrade (agent-os PR #743,
  2026-09-24) is the precedent.
- The Copilot plugin v0.3.7 (darwin/arm64 dylib, ABI v1) is loaded on every node and must stay loaded.
- Home's own CPA SDK pin (`go.mod`) is separate; do NOT change Home in this workstream unless the
  upgrade proves it necessary.

## Acceptance criteria

1. Release choice justified in this brief: newest 8.0.x (or later) tag, its changelog read for
   breaking changes between 8.0.4 and it (config, plugin ABI/schema, RESP/Home contract), and
   confirmation that it contains `bfa5aed`.
2. Canary first: the MacBook node, then one mini, then the rest — one box at a time, each under
   its fleet-lock, never stopping `cpa-home-node` (restart in place only), with the script's rollback.
3. Per box after upgrade: `cli-proxy-api --version` shows the new version; Home sees the node;
   plugin `cliproxyapi-copilot 0.3.7 installed loaded`; the agent key gets 200 on a native model
   (`claude-sonnet-5-5`); the MacBook key also gets 200 on `copilot/gpt-5-mini` (use max_tokens ≥ 400 —
   small caps return empty because the model spends them on reasoning).
4. Fix proven: evidence that #6225 no longer bites (e.g. reading the fix + a safe reproduction if one
   exists that does not disturb the live fleet; if none is safe, say so and record the reasoning).
5. Lesson from ws-0005: never poll a remote job with `pgrep -f <name>` from a shell whose own command
   line contains `<name>` — it matches itself forever while you hold the locks. Use a pid file or a
   DONE marker.
6. Update memory-adjacent facts in this brief (new version, commit) and in FLEET.md if it names a version.

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
