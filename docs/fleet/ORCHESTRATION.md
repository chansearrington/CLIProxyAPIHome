# Fleet CPA orchestration — index

One orchestration pane (herdr space `CLIProxyAPIHome`, the parent checkout on `fleet`) starts
the workstreams, tracks them and checks their evidence. Each workstream is its own herdr space +
git worktree + Claude agent, and its brief is its record. Started 2026-09-30.

## Chanse's decisions for this round (2026-09-30)

- Work: the ws-0002 follow-ups, plus "Sonnet 5.5 is released but I don't see it on this MacBook
  Pro, and I'm not sure it exists anywhere else either."
- Sessions = herdr sessions, one per workstream.
- **Autonomy: full, including deploys** to the live Home on the Ark and to the nodes. This is
  Chanse's standing decision for this round and stands in for the per-deploy `ship` card that
  FLEET.md asks for. The guards below still apply without exception.
- Records: one brief per workstream in `docs/fleet/` plus this index.

## Workstreams

| ws | Title | Repo / branch | herdr alias | Status |
|---|---|---|---|---|
| ws-0003 | Sonnet 5.5 across the fleet | CLIProxyAPIHome `ws-0003/sonnet-5-5` | `cpahome-ws-0003` | **done** — no Home change; minis' Claude Code 2.1.284→2.1.286 |
| ws-0004 | Upstream contributions | CLIProxyAPIHome `ws-0004/upstream-contrib` (+ plugin repo) | `cpahome-ws-0004` | **done** — Home PR #124, plugin offer issue #3; nothing live changed |
| ws-0005 | Copilot on the quota page | CLIProxyAPIHome `ws-0005/copilot-quota` | `cpahome-ws-0005` | **done** — live `cpa-home:1.0.73-claude-fleet-16f609d` |
| ws-0006 | Copilot plugin polish | cliproxyapi-copilot-plugin `ws-0006/plugin-polish` | `cpaplugin-ws-0006` | **done** — plugin v0.3.7 live on Home + 5 nodes |

| ws-0007 | CPA nodes 8.0.4 → 8.0.5+ | CLIProxyAPIHome `ws-0007/cpa-node-upgrade` | `cpahome-ws-0007` | **done** — all five nodes on CPA 8.0.7 |
| ws-0008 | CodexBar ↔ CPA menu bar usage | CLIProxyAPIHome `ws-0008/codexbar` | `cpahome-ws-0008` | done, then **removed** at Chanse's request (app not wanted) |

Round 1 (ws-0003..0006) ran in parallel; round 2 (ws-0007, ws-0008) started 2026-10-01. The only thing they share is the live system, so deploys are one at a
time (see below).

## Guards that every workstream follows (no approval gates, but these are not optional)

1. **Builds and tests only on the Ark**, inside `golang:1.26-bookworm` via
   `/mnt/user/appdata/cpa-home-build/run-go.sh` (FLEET.md). Full suite needs
   `--tmpfs /tmp:exec,size=3g`. No Go on the laptop.
2. **One live change at a time.** Before touching the live Home, its config, or any node, take the
   fleet-locks as in the agent-os runbook
   `docs/runbooks/cpa-home-build-and-rollback.md` (canonical `cpa-home-runtime-20260907-root` /
   `cpa-home-root`, heartbeat every 60 s). If a lock is held by another workstream, wait for it;
   never break it.
3. **Before a Home image build**: `git fetch origin`, rebase your branch onto the CURRENT
   `origin/fleet` (another workstream may have shipped since you started), re-run the full suite on
   the rebased tree, then fast-forward `fleet` with `--force-with-lease`. The image must contain
   everything already live.
4. **Backup before a Home change**: `VACUUM INTO` from a script run with nohup; record size,
   integrity and sha256 in your brief. Know the rollback image ID before you swap.
5. **Config writes**: a `PUT /config.yaml` soft-deletes any API key missing from the upload —
   always carry the full `api-keys` list (memory: home-plugins-and-copilot).
6. **Nodes**: never stop `cpa-home-node` on a mini; restart in place only, one box at a time.
7. **Proof after every live change**: real requests through a node, every agent key HTTP 200 on a
   native model, and the change's own proof. On a failed proof, roll back first, then investigate.
8. **PRs to Chanse's own repos**: review with agent-os `shared/scripts/local-pr-review`, fix
   MUSTs, merge yourself with the evidence. Upstream PRs (router-for-me/*, arthur-sommer-etc/*)
   are opened normally and never self-merged.
9. **No secrets** in any repo, brief, log or PR. Fake values in fixtures.
10. **Write as you go** in your brief: findings with file:line, decisions, evidence, close-out.
    Keep a one-line `Status:` at the top of the brief current — the orchestrator reads it.

## Log

- 2026-09-30 — round started; ws-0003..ws-0006 briefs written and sessions launched.
- 2026-10-01 — ws-0003 done, verified by the orchestrator: MacBook `claude -p --model sonnet` → `claude-sonnet-5-5` end_turn via the node; all four minis report Claude Code 2.1.286; worktree/branch gone; space closed. Side fact: nodes run CPA 8.0.4 (d33f63f8), not 7.3.16.
- 2026-10-01 — ws-0004 done, verified: router-for-me/CLIProxyAPIHome#124 open on `dev` (2 files, no internal names in the body); arthur-sommer-etc/cliproxyapi-copilot-plugin#3 open; CPA #6225 closed upstream (fix in v8.0.5); Ark `run-go.sh` now 3g tmpfs with backup `.bak-20260930`; worktrees/branches gone. Corrected its follow-up: nodes are on CPA 8.0.4, so #6225's fix is a patch bump to 8.0.5.
- 2026-10-01 — ws-0005 deployed 00:14Z but its driver hung in a `pgrep -f` self-match loop for ~46 min holding all four fleet-locks (blocking ws-0006) and skipping the post-swap proof. Orchestrator ran the proof under the still-held locks (4 minis + MacBook 200 'ok'; Copilot quota healthy/fresh, Enterprise, 1,000,000 premium; others unchanged), then killed the driver to release the locks. ws-0005 recorded the incident and closed out; worktree/branch removed. **Lesson for all drivers: never poll a remote job with `pgrep -f <name>` from a shell whose own command line contains `<name>` — use a pid file or a DONE marker in the log.**
- 2026-10-01 — ws-0006 done, verified: release v0.3.7 has darwin/arm64 + linux assets; Home log shows every node hot-reloading 0.3.6 → 0.3.7 at 20:07 CDT with no restart; after its `PUT /config.yaml`, all four agent keys + the MacBook key still get 200 (`claude-sonnet-5-5`), and the MacBook gets `copilot/gpt-5-mini` 'ok'. Spaces closed.
- 2026-10-01 — **round closed.** All herdr workstream spaces closed (only the orchestrator remains); no ws-0003..0006 worktrees or branches left in either repo. Kept on purpose: `origin/fix/plugin-model-discovery` (head of upstream PR #124), the fork's `offer/*` branches (plugin issue #3), Home backups `home-pre-ws0005-*` / `home-pre-ws0006-*`.

## Open follow-ups from this round

1. ~~Nodes CPA 8.0.4 → 8.0.5+~~ — done in ws-0007 (8.0.7).
2. Non-streamed `/v1/responses` on Copilot is recorded in Home's usage with 0 tokens (client gets the real counts) — likely the node's usage extraction for plugin Responses JSON (ws-0006).
3. Claude Code on the minis only updates when run; consider a scheduled update (ws-0003).
4. `gpt-6.1-sol` missing from the minis' OpenClaw pickers — WS-547's picker-drift card (ws-0003).
5. When upstream merges #124 / #123, drop those commits from `fleet` at the next rebase (ws-0004).
- 2026-10-01 — round 2: Chanse asked for the 8.0.5 node upgrade (follow-up 1) and a CodexBar/CPA menu-bar integration; ws-0007 and ws-0008 briefs written and launched. Same full-autonomy decision applies.
- 2026-10-01 — ws-0007 done, verified by the orchestrator: `--version` 8.0.7 `97f244b8` on all five nodes; each mini's agent key 200 on `claude-sonnet-5-5`; MacBook key 200 'ok' on `claude-sonnet-5-5` and `copilot/gpt-5-mini`; agent-os #772 merged; worktree/branch gone; space closed. Rollback per node = `cli-proxy-api.8.0.4.bak`.
- 2026-10-01 — ws-0008 done, verified by the orchestrator: `~/Applications/Home Usage.app` running under LaunchAgent `com.chansearrington.home-usage`; Keychain item `cpa-home-management`/`home-usage` present; no secrets in `tools/home-menubar/` or the proof folder; dropdown screenshot shows all six Home credentials and its compare file says 10/10 windows match the API at the same second. Note: the app holds Home's full management secret (no read-only key exists) — follow-up below. A Claude window Anthropic labels "Iguana Necktie" (0%, resets Nov 5) shows under its raw name. Space closed; no worktrees left.

### Follow-ups added in round 2
6. ~~Read-only management key for the menu bar app~~ — moot, app removed.
7. First real new-plugin add is the live proof of the #6225 fix; watch Home's log for `installed but not loaded` (ws-0007).
- 2026-10-01 — Chanse rejected Home Usage (wants multiple accounts inside CodexBar). App, LaunchAgent, Keychain key, Bartender entries and `tools/home-menubar/` removed; researching CodexBar multi-account support.
