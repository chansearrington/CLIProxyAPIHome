# ws-0004 — Upstream contributions

Status: started 2026-09-30

- Branch `ws-0004/upstream-contrib` off `fleet` @ `70aa888` (for this brief); the upstream
  PR branch goes off **upstream `dev`**, not `fleet`. Worktree `.claude/worktrees/ws-0004-upstream-contrib`
- herdr: `CLIProxyAPIHome | ws-0004 | Upstream contributions`, alias `cpahome-ws-0004`
- Plugin repo: `/Users/chansearrington/GitHub/personal/cliproxyapi-copilot-plugin` (remotes
  `origin` = Chanse's fork, `upstream` = arthur-sommer-etc). Use your own worktree there too;
  ws-0006 is also working in that repo on branch `ws-0006/plugin-polish`.

## Goal

Give back the fixes the fleet carries so we stop maintaining them alone (ws-0002 follow-ups).

## Acceptance criteria

1. **Home PR** to `router-for-me/CLIProxyAPIHome` `dev` with the plugin model-discovery fix
   (fleet commit `95ba2db`, "load the full auth for plugin per-auth model discovery"). Clean
   branch `fix/plugin-model-discovery` off current `upstream/dev`, only this change + its tests,
   upstream's conventions (AGENTS.md), full suite green on the Ark against upstream `dev`. If it
   depends on PR #123 (plugin login identity), say so in the PR and base it so it stands alone or
   stacks cleanly. Link the PR here and in FLEET.md's open-PR list.
2. **Plugin contributions** to `arthur-sommer-etc/cliproxyapi-copilot-plugin`: the general patches
   the fork carries (402→429, `/v1/messages` preference, native OpenAI chat format, Responses
   `max_output_tokens` clamp, darwin build). The fork's `FORK.md` lists them. Split into sensible
   PRs against upstream's default branch (small, one concern each, each with tests), leaving out
   anything fleet-specific. If upstream looks inactive, open one issue offering them with links
   rather than a PR flood. Record links here.
3. CPA issue #6225 and Home PR #123: check for maintainer replies; answer any questions.
4. Every PR text is plain, factual and contains no secrets, hostnames or fleet internals.
5. Never self-merge upstream PRs. Only touch fleet/live systems if a review requests a change
   that the fleet also needs (then follow the ORCHESTRATION.md guards).

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
