# ws-0004 — Upstream contributions

Status: in progress 2026-09-30 — Home PR #124 open; plugin offer branches being built/tested; #6225 fixed upstream (CPA v8.0.5)

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

## Log and evidence

### 1. Home PR — router-for-me/CLIProxyAPIHome#124 (opened 2026-09-30)

- https://github.com/router-for-me/CLIProxyAPIHome/pull/124 — branch `fix/plugin-model-discovery`
  (fork), base upstream `dev` @ `e16d6f1` (the v8 merge, newer than `fleet`'s base), one commit
  `de84951` = fleet `95ba2db` cherry-picked cleanly (trailer normalized only; tree identical to the
  tested `d5ded00`). Files: `internal/home/plugin_runtime.go` (+`pluginDiscoveryAuth`) and
  `internal/home/plugin_discovery_auth_test.go` (2 tests, fake token).
- Ark run, `golang:1.26-bookworm`, 3g exec tmpfs, separate source dir
  `/mnt/user/appdata/cpa-home-build/ws-0004-src` (so the shared `src` checkout other workstreams use
  was untouched); log `/mnt/user/appdata/cpa-home-build/logs/ws0004-pr-d5ded00.log`:
  `gofmt -l` empty; `go vet` only `internal/cluster/refresh.go:172:2: unreachable code`;
  `go test ./...` TESTEXIT=0, 33 packages ok; `go build ./cmd/home` OK. New tests re-run with
  `-count=1 -v`: both PASS.
- Independent of #123 (#123 touches `internal/cluster/{management/plugin_oauth.go,uuid.go,...}`; no
  shared files). Stated in the PR.
- Note: `run-go.sh` on the Ark still says `--tmpfs /tmp:exec,size=1g`; FLEET.md says 3g. I passed 3g
  explicitly rather than editing the shared script.
- Criterion 4 sweep of my own open PR texts: removed "the fleet's Ark" from #123 and "the fleet's own
  nodes" from #115; #116 and #124 had none.

### 3. Upstream replies (checked 2026-09-30)

- CPA issue #6225: **fixed and closed by the maintainer (luispater)** with commit `bfa5aed`
  ("defer plugin load marking and preempt failing config on retry", `sdk/cliproxy/home_plugins.go`,
  `service_home.go`), shipped in CPA **v8.0.5** (2026-09-30; compare `bfa5aed...v8.0.5` = ahead/0
  behind). No questions to answer. Our nodes run CPA 7.x, so the fleet only gets the fix by
  upgrading nodes to v8.0.5+ (a major-version jump) — follow-up, not done here.
- Home PR #123: open, mergeable, no comments or reviews. #115 and #116: no comments or reviews.
