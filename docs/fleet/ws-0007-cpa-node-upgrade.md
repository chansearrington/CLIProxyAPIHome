# ws-0007 — CPA nodes 8.0.4 → 8.0.5+

Status: done — all five nodes on CPA 8.0.7 (2026-10-01 01:44Z), proven; agent-os #772 merged

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

### #6225 fix proof (AC4)

**Offline reproduction, on the Ark** (`golang:1.26-bookworm`, Go 1.26.8, private dir
`cpa-home-build/ws0007-cpa`, deleted afterwards — verified by `ls`), run by a subagent and checked
against the fix diff I read myself (`needsLoadMarking`, `tryDequeueLatest`):

- **v8.0.7 (`97f244b8`)**, upstream's three regression tests unmodified
  (`go test -count=1 -run Issue6225 -v ./sdk/cliproxy/`): all **PASS** —
  `TestStageHomeOverlay_InstalledPluginAtRuntime_Issue6225`,
  `TestHomeConfigWorkQueue_TryDequeueLatest_Issue6225`,
  `TestHomeConfigWorkerPreemptsFailingConfigWithNewerPayload_Issue6225`. Full `./sdk/cliproxy/`
  package: `ok` (24.9 s).
- **Pre-fix code** (`097511b8`, bfa5aed's parent; and **v8.0.4** `d33f63f8`, what the fleet ran):
  the same tests, minus two assertions on the new `needsLoadMarking` field and minus the
  `tryDequeueLatest` unit test (that method doesn't exist pre-fix), both **FAIL** with the live
  symptom:
  - `stageHomeOverlayWithClient() error = load home plugins: home plugins: plugin myplugin installed
    but not loaded` (the exact loop message ws-0002 saw on every node);
  - `worker never recovered to strategy 'recovered', current="initial"` after ~20
    `failed to stage home config; retrying` lines — the worker never picks up the newer config.
  - Control: the adapted file passes at v8.0.7, so the adaptation didn't weaken the tests.

**Why no live reproduction:** the only real trigger is adding a *brand-new* plugin to Home's
config, which goes to Home itself and all five nodes at once. That needs a second, harmless plugin
with darwin/arm64 + linux artifacts and a pinned store manifest (none exists), a `PUT /config.yaml`
(soft-deletes any API key not carried), and — if the fix misbehaved — the failure mode is every node
refusing further config changes until restarted. Removing and re-adding the Copilot plugin instead
would cut Copilot off and risks its stored credential. Neither is a "safe reproduction that does not
disturb the live fleet", so I did not do one. Residual risk: low (the fix is small and its tests
reproduce the exact message); **follow-up:** the next real new-plugin add is the live proof — watch
Home's log for `installed but not loaded` retry lines and restart a node in place if one loops.

## Close-out (2026-10-01 ~01:55Z)

**Outcome:** all five nodes (MacBook Pro, Moxy, Hyper, Lara, Chip) moved CPA **8.0.4 → 8.0.7**
(`97f244b8`, binary sha `c46f68ee…1365`) between 01:40Z and 01:44Z, one box at a time with no
lasting failure. Home was not changed.

| AC | Evidence |
|---|---|
| 1. Release choice | v8.0.7 = newest; `bfa5aed` contained (compare `ahead`), absent from 8.0.4; plugin ABI 1 / schema 6, cloak `2.1.280`, `internal/home/` all unchanged; one optional config key; 8.0.5 skipped for its twice-corrected Codex change. See "Release choice". |
| 2. Canary order, locks, rollback | MacBook (all four locks) → Moxy → Hyper → Lara → Chip, each mini under its own `fleet-lock hold`, released after; `launchctl kickstart -k` only (no stop); `.bak` = `cli-proxy-api.8.0.4.bak`. The rollback was exercised live once on the MacBook (`RESULT=ROLLED_BACK`, 200 after), caused by an over-strict check of mine, not by 8.0.7. |
| 3. Per-box proof | Each box: `--version` 8.0.7 `97f244b8`; Home `healthy True reported_ok`, `cliproxyapi-copilot 0.3.7 skipped loaded` (`skipped` = already on disk, healthy); own key 200 'ok' on `claude-sonnet-5-5`; MacBook key 200 'ok' on `copilot/gpt-5-mini` (max_tokens 400) and `gpt-5.6-sol`. Re-checked at 01:52Z: all five still 200, Home ledger 756 rows / **0 failed** since 01:38Z. |
| 4. Fix proven | Upstream's three Issue6225 tests PASS at v8.0.7 and the portable ones FAIL at 8.0.4 / the fix's parent with the exact live message (`installed but not loaded`; worker never recovers). No safe live repro exists (adding a plugin hits Home + all nodes at once) — reasoning recorded above. |
| 5. No `pgrep` polling | The driver ran every remote step synchronously and read completion from `upgrade-node-core.sh`'s own `RESULT=` line; every wait loop was bounded (≤ 6 tries, ≤ 90 s); locks were held only for 33–73 s per box. |
| 6. Facts updated | This brief; `FLEET.md` (node version, fix status, `skipped loaded`); agent-os PR **#772** merged (`7ff95d03`): `CORE_VERSION/CORE_SHA256` → 8.0.7 in `enroll-fleet-node.mjs` / `enroll-chip.mjs` (bumped after Hyper was on the new sha), rollout notes in `cpa-model-currency.md`, version lines in `cpa-home-pilot.md`; reviewed by `local-pr-review` (gpt-6-astra): no findings, 292/292 tests. |

**What is live now:** CPA 8.0.7 on all five nodes; Copilot plugin v0.3.7 loaded everywhere; Home
unchanged (`cpa-home:1.0.73-claude-fleet-16f609d`). Rollback per node if ever needed:
`ROLLBACK_ONLY=1` with the same FROM/TO env restores the verified `cli-proxy-api.8.0.4.bak`.

**Follow-ups:**
1. The first real *new* plugin added through Home is the live proof of the #6225 fix — watch Home's
   log for `installed but not loaded` retry lines; restart a node in place if one still loops.
2. agent-os's release watcher still holds a stale `cardStatus: open` for the 8.0.4 card in
   `~/.openclaw/state/cpa-release-watcher.json` (its own runs read it as `stored_card_actioned`, so
   it is harmless). Its next daily check (2026-10-01 14:13Z) should find the fleet on the latest
   release and raise nothing.
3. Upstream 8.0.5–8.0.7 are hours old; `82f8e92b` (shared uTLS client) and `81756a57` (credential
   auto-refresh loop) are the changes to suspect if odd upstream-connection errors appear.

**Cleanup:** agent-os worktree/branch `ws-0007/cpa-core-pin-8.0.7` removed (local + remote); the
Ark scratch dir `cpa-home-build/ws0007-cpa` deleted (verified); laptop scratch `/tmp/ws0007`
deleted; staged `.new` binaries consumed by the script on every box. Kept on purpose: each node's
`cli-proxy-api.8.0.4.bak` (rollback target).
