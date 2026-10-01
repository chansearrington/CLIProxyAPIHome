# ws-0007 — CPA nodes 8.0.4 → 8.0.5+

Status: in progress — release chosen (v8.0.7), canary next

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

## Findings and decisions

### Release choice (AC1) — v8.0.7, decided 2026-10-01 01:4xZ

- Releases after 8.0.4: v8.0.5 (2026-09-30 16:09Z), v8.0.6 (18:05Z), **v8.0.7 (19:14Z, latest)**.
- `bfa5aed` (fix for #6225): `compare bfa5aed...v8.0.7` = `ahead`, `bfa5aed...v8.0.4` = `behind`
  → contained in 8.0.5/8.0.6/8.0.7, absent from 8.0.4.
- v8.0.4 → v8.0.7 = 30 commits, 116 files. Checked for the things that could break us:
  - **Plugin ABI/schema:** `sdk/pluginabi/types.go:7,20` `ABIVersion = 1`, `SchemaVersion = 6` at both
    tags; no `sdk/pluginabi` file changed. `b7f321f2` adds an optional `path` field to two
    `sdk/pluginapi` message types (older plugins never send it) → Copilot v0.3.7 unaffected.
  - **Home/RESP contract:** no file under `internal/home/` changed. Only `sdk/cliproxy/home_plugins.go`
    and `service_home.go` (the fix itself).
  - **Claude Code cloak literal:** `"2.1.280"` at both tags (`claude_executor_cloaking.go:307`).
  - **Config:** one new optional key (`oauth-settings`, `df2774ca`); nothing renamed, no default changed.
  - **CLI/startup/`--version`:** no `cmd/server` or build-info change.
  - **Removed node routes:** `097511b8` drops the node's `/credentials/quota/{providers,fetch,reset}`
    management routes; nodes run with management disabled (empty secret-key) and Home does not call them.
  - **Why not 8.0.5:** its Codex tool-schema change (`71a5f1f6`) was corrected twice within ~3 h
    (`a8ffd5a8` in 8.0.6, `67cb32b6` + `97f244b8` in 8.0.7).
  - **Risks noted:** all three releases are hours old; `82f8e92b` touches the shared uTLS client
    (all upstream calls) and `81756a57` the shared credential auto-refresh loop → the canary is
    smoke-tested on a Claude, a Codex (GPT) and a Copilot model before any mini is touched.
- **What `bfa5aed` changes:** before, a node checked "is the new plugin registered?" *before*
  applying the config that loads it, so a newly added plugin always failed and the config worker
  retried the same payload forever. Now (a) load marking moves to the status-push finalisation,
  after the config is applied (`needsLoadMarking: didSync`), and (b) the retry loop picks up the
  newest queued Home config (`tryDequeueLatest`), so a better config can replace a failing one.
- Artifact: `CLIProxyAPI_8.0.7_darwin_aarch64.tar.gz`, archive verified against upstream
  `checksums.txt`; extracted `cli-proxy-api` **sha256
  `c46f68ee7517ccf891ffee47dc6747c5ed3fe1fdd2759fbaac7ee7f9340d1365`, 63,039,202 bytes**, Mach-O
  arm64, banner `Version: 8.0.7, Commit: 97f244b8, BuiltAt: 2026-09-30T19:15:24Z`.

### Baseline (2026-10-01 ~01:40Z)

- All five nodes: live sha `5c6e3095…ecc1` (= 8.0.4 `d33f63f8`), no `cli-proxy-api.8.0.4.bak` yet.
- Home `GET /nodes`: all five `healthy True reported_ok | cliproxyapi-copilot 0.3.7 installed loaded`.
- MacBook key: `claude-sonnet-5-5` 200 'ok'; `copilot/gpt-5-mini` 200 'ok'.
- Fleet-locks: all four UNLOCKED.
- agent-os release watcher (laptop): its daily check ran 2026-09-30 14:13Z, *before* 8.0.5 existed,
  so no card for 8.x.5+ exists; the tap poller only acts on a Ship tap of an open card. Next check
  2026-10-01 14:13Z reads real node versions — if the fleet is on 8.0.7 it raises nothing. No
  collision with this manual rollout.

### Method

Manual, per agent-os runbook `cpa-model-currency.md` "Upgrading the node binary", using
`control-plane/cpa-home/upgrade-node-core.sh` (automatic rollback to the verified `.bak` on
not-ready/banner mismatch; `ROLLBACK_ONLY=1` for a failed external proof). Not the watcher's
`cpa-rollout.mjs`: it runs all five in one go and does not check the plugin state between boxes,
which this brief requires. Order: MacBook (canary; all four locks held so no other live change runs
concurrently) → Moxy → Hyper → Lara → Chip, each mini under its own `fleet-lock hold` (canonical
session/actor). Remote completion is read from the script's own `RESULT=` line (synchronous ssh),
never `pgrep` (AC5).
