# ws-0004 — Upstream contributions

Status: done 2026-09-30 — Home PR #124 + plugin offer issue #3 open upstream (awaiting maintainers); #6225 fixed upstream in CPA v8.0.5; nothing live changed

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

### 2. Plugin contributions — arthur-sommer-etc/cliproxyapi-copilot-plugin#3 (opened 2026-09-30)

- **Decision: one offer issue, not PRs.** Upstream is inactive: single author, last commit
  2026-08-07 (`7f16b60`), last push 2026-09-08, and both outside PRs (#1, #2 by Goodwu, 2026-08-24)
  have zero comments or reviews. The brief's rule for that case is one issue with links.
- https://github.com/arthur-sommer-etc/cliproxyapi-copilot-plugin/issues/3 links three clean
  branches on the (public) fork, each off upstream `main` `7f16b60`, neutral commit messages (no
  ws/fleet/Home-internal wording, no version bumps):
  - `offer/quota-402-as-429` `cdab327` = fork `01a885e` (402 → 429).
  - `offer/claude-messages-endpoint` `ee0587c` = fork `1ffe0ac` (`/v1/messages` first for Claude).
  - `offer/openai-chat-format` `35a45ba` = `ee0587c` + fork `132baf9`, `d8f766a`, `d43d514`
    (native OpenAI chat format, Responses `max_output_tokens` clamp to 16, chat-stream fixes,
    Responses string-input / Claude-JSON fixes). Conflicts were version lines only (kept
    upstream's 0.3.3) and `dispatch_test.go`, which on the fork was created by the schema-1 commit:
    recreated with only the OpenAI-format test.
- Left out on purpose: `f888311` schema 1 (exists only for Home's older embedded host; comment names
  Home), `e48c953` `copilot/` prefix (renames everyone's models; upstream already solved collisions
  differently in `3a8246f` and open PR #1 edits the same code), `1d51c02` darwin build (duplicates
  upstream PR #2 and carries FORK.md + fork URL); the issue only offers its `dlopen` load test.
- Ark evidence (`golang:1.26-bookworm`, 3g exec tmpfs, dirs
  `/mnt/user/appdata/cpa-home-build/ws-0004-plugin-<branch>`): each branch `gofmt -l` 0 files,
  `go vet` silent, `go test -count=1 ./...` all packages ok, `-buildmode=c-shared` build OK.
  Compare API for each link: ahead 1 / 1 / 4, behind 0; link HTTP 200.
- Audit of the fork commits (subagent, spot-checked): no secrets in any diff; fixtures use
  `gho_fake_test_token`.

### 3. Upstream replies (checked 2026-09-30)

- CPA issue #6225: **fixed and closed by the maintainer (luispater)** with commit `bfa5aed`
  ("defer plugin load marking and preempt failing config on retry", `sdk/cliproxy/home_plugins.go`,
  `service_home.go`), shipped in CPA **v8.0.5** (2026-09-30; compare `bfa5aed...v8.0.5` = ahead/0
  behind). No questions to answer. Our nodes run CPA 7.x, so the fleet only gets the fix by
  upgrading nodes to v8.0.5+ (a major-version jump) — follow-up, not done here.
- Home PR #123: open, mergeable, no comments or reviews. #115 and #116: no comments or reviews.
- Re-checked at close-out (2026-09-30): #115, #116, #123, #124 all open with 0 comments / 0
  reviews; plugin issue #3 0 comments.

## Close-out (2026-09-30)

**Outcome.** Both fleet carry-overs are now offered upstream, and the CPA plugin-install loop we
reported is fixed upstream. No live system was touched (criterion 5: no review asked for a change).

| # | Criterion | Evidence |
|---|---|---|
| 1 | Home PR, clean branch off upstream `dev`, suite green, linked | router-for-me/CLIProxyAPIHome#124, branch `fix/plugin-model-discovery` = upstream `dev` `e16d6f1` + `de84951`; Ark: gofmt clean, vet only the known `refresh.go:172`, 33 pkgs ok, build OK, new tests PASS with `-count=1`. Independent of #123 (stated). Linked here and in FLEET.md. |
| 2 | Plugin contributions | Upstream inactive → one offer issue arthur-sommer-etc/cliproxyapi-copilot-plugin#3 linking three tested branches (402→429; `/v1/messages`; OpenAI chat + Responses fixes, stacked). Fleet-only patches left out, with reasons above. |
| 3 | #6225 and #123 replies | #6225 closed by the maintainer with fix `bfa5aed`, released in CPA v8.0.5; nothing to answer. #123: no replies. |
| 4 | PR texts plain, no secrets/hostnames/internals | New texts scanned for fleet/Ark/Tailscale/agent-os/ws-/IPs: none. Also scrubbed fleet wording from my older PRs #123 and #115. |
| 5 | No self-merge; live systems untouched | No upstream merges by us. Only Ark change: build-infra script below (not the live Home or nodes). |

**What changed outside git.** The Ark's shared `/mnt/user/appdata/cpa-home-build/run-go.sh` now
uses `--tmpfs /tmp:exec,size=3g` (was 1g, which FLEET.md already says is too small). Backup
`run-go.sh.bak-20260930`; verified `df /tmp` = 3.0G and `sh -n` OK. Temp source dirs
`ws-0004-src` and `ws-0004-plugin-*` removed after the runs; the test log
`logs/ws0004-pr-d5ded00.log` is kept as evidence.

**What is live.** Unchanged: Home `cpa-home:1.0.73-claude-fleet-95ba2db`, plugin v0.3.6.

**Follow-ups (not done here).**
- The fleet gets the #6225 fix only by moving the nodes to CPA **v8.0.5+**. (Orchestrator
  correction 2026-10-01: the nodes already run CPA 8.0.4 `d33f63f8`, verified on the MacBook node
  and by ws-0003 on all five, so this is a patch bump, not a jump from 7.x.) Worth its own workstream (check Home's SDK pin and plugin schema compatibility first).
- When upstream merges #124 (and #123), drop `95ba2db` (and the plugin-login fix) from `fleet` at
  the next rebase instead of carrying them.
- If the plugin maintainer answers issue #3, open the PRs from the `offer/*` branches on the fork
  (they must stay on the fork until then). Worth adding a link to issue #3 in the plugin fork's
  `FORK.md` (left alone here because ws-0006 is working on that repo's `fleet`).
