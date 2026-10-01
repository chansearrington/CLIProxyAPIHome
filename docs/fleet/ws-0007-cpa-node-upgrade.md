# ws-0007 — CPA nodes 8.0.4 → 8.0.5+

Status: in progress — all five nodes on 8.0.7 and proven; #6225 proof + pin bump next

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

### Canary attempt 1 — upgraded, proved, then auto-rolled back by an over-strict check (01:38–01:39Z)

- Under all four locks (01:38:22Z → 01:39:35Z, released cleanly). `upgrade-node-core.sh` →
  `RESULT=UPGRADED node_version=8.0.7 restart_at=2026-10-01T01:38:24Z` (ready in ~19 s).
- Proof on 8.0.7: `claude-sonnet-5-5` 200 'ok' (first try, 12 s after restart),
  `copilot/gpt-5-mini` 200 'ok', `gpt-5.6-sol` 200 'ok'; `--version` 8.0.7 `97f244b8`; Home: healthy,
  `reported_ok`, plugin `0.3.7 skipped loaded`.
- My driver demanded the literal `installed loaded`, so it called `ROLLBACK_ONLY=1` →
  `RESULT=ROLLED_BACK node_version=8.0.4` (01:39:15Z); post-rollback `claude-sonnet-5-5` 200 'ok'.
  The rollback path is therefore proven live on this box.
- **Why `skipped` is healthy:** `internal/pluginstore/install.go:279-285` (v8.0.7) sets
  `Skipped: true` when the library already on disk is byte-identical to the store's; the sync then
  reports install `skipped` (`internal/homeplugins/sync.go:187-188`). Control: the MacBook back on
  **8.0.4** after its restart also reports `0.3.7 skipped loaded`. The other four show `installed`
  only because their last report dates from the 0.3.7 hot install. The check now accepts
  `(installed|skipped) loaded` and still requires `healthy True reported_ok` and `loaded`.

### Canary — MacBook on 8.0.7 (01:40:38Z)

- All four locks held 01:40:38Z → 01:41:15Z, released. `RESULT=UPGRADED node_version=8.0.7
  restart_at=2026-10-01T01:40:38Z` (`.bak` = `cli-proxy-api.8.0.4.bak`, sha-verified by the script).
- `--version`: `CLIProxyAPI Version: 8.0.7, Commit: 97f244b8, BuiltAt: 2026-09-30T19:15:24Z`.
- Home: `connected 01:40:54Z healthy True reported_ok | cliproxyapi-copilot 0.3.7 skipped loaded`.
- Key 6: `claude-sonnet-5-5` 200 'ok' (first try, 11 s after restart); `copilot/gpt-5-mini`
  (max_tokens 400) 200 'ok'; `gpt-5.6-sol` (Codex path, exercises the changed uTLS/Codex code) 200 'ok'.
- Node log since the banner: only the documented handoff noise (`certificate is already owned by
  an active membership` 20:40:49–54 CDT, a burst of 503s on the laptop's own Claude Code traffic in
  the same 5 s); after 20:40:55 no warn/error lines, steady 200s on `/v1/messages`.

### Minis on 8.0.7 (each under its own `fleet-lock hold`, released after; one box at a time)

| node | lock held | restart (UTC) | result | sonnet 200 | Home (connected / state / plugin) |
|---|---|---|---|---|---|
| Moxy | 01:42:00–01:42:35Z | 01:42:01 | `RESULT=UPGRADED` | 'ok' 1st try | 01:42:19Z, healthy, reported_ok, `0.3.7 skipped loaded` |
| Hyper | 01:42:53–01:43:25Z | 01:42:55 | `RESULT=UPGRADED` | 'ok' 1st try | 01:43:09Z, healthy, reported_ok, `0.3.7 skipped loaded` |
| Lara | 01:43:33–01:44:06Z | 01:43:35 | `RESULT=UPGRADED` | 'ok' 1st try | 01:43:49Z, healthy, reported_ok, `0.3.7 skipped loaded` |
| Chip | 01:44:13–01:44:46Z | 01:44:16 | `RESULT=UPGRADED` | 'ok' 1st try | 01:44:29Z, healthy, reported_ok, `0.3.7 skipped loaded` |

- Every box: `--version` → `CLIProxyAPI Version: 8.0.7, Commit: 97f244b8, BuiltAt: 2026-09-30T19:15:24Z`;
  `.bak` = `cli-proxy-api.8.0.4.bak` (sha-verified against `5c6e3095…` by the script).
- Proof keys: each mini's own OpenClaw `cpa-gui` key (read on the box, never printed) via
  `127.0.0.1:18317`. Moxy's node log after the restart: no warn/error lines, only 200s.

### Fleet-wide check after the last box (01:45Z)

- Home `GET /nodes`: all five `healthy True reported_ok | cliproxyapi-copilot 0.3.7 skipped loaded`,
  each `connected` after its own restart. `cpa_node_membership`: five rows `active`, each
  `connected_at` = that node's reconnect time.
- Ledger (`usage`, `created_at > '2026-10-01 01:38:00'`): **338 rows, 0 failed** — MacBook 334
  (Chanse's live Claude Code traffic + proofs), Chip 3, Moxy/Hyper/Lara 1 each (the proofs; their
  agents were idle). The ~5 s of 503s during the MacBook handoff were node-local (no membership yet)
  and never reached Home, as the runbook says.
