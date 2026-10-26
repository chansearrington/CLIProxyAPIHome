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
| ws-0008 | CodexBar ↔ CPA menu bar usage | CLIProxyAPIHome `ws-0008/codexbar` | `cpahome-ws-0008` | **done** — app removed; both Claude accounts now inside CodexBar (cookie accounts, Stacked layout) |
| ws-0009 | Microsoft Claude account = Chanse-only backup | config only (brief `ws-0009-claude-backup-account.md`) | — | **applied** 2026-10-06 15:44Z; Hype priority 1 (sequential) 20:07Z; **done** — switch-back proven 21:07Z, sequential order proven 01:07Z |
| ws-0010 | Claude OAuth recovery and Codex routing | fleet documentation | — | **done** — independent Home OAuth restored; Codex default CPA and daemon warning fixed |
| ws-0011 | Home 1.1.0 and CPA 8.0.20 fleet upgrade | `ws-0011/upstream-fleet-upgrade` | — | **deployed and verified** — Home 1.1.0 fork + all five CPA 8.0.20 nodes; native routing and protected scopes pass |
| ws-0012 | Home access tokens into CodexBar Claude accounts | `fix/codexbar-claude-access-sync` | — | **installed and verified** — Hype/Gmail OAuth in CodexBar, access-token-only sync every 120 seconds |
| ws-0013 | Third shared Claude subscription | configuration + fleet record | — | **done** — Lara shared with all users, before Microsoft; all six keys verified; added to CodexBar and automatic token sync |
| ws-0014 | Developer Claude subscription | configuration + fleet record | — | **done** — Developer shared and in CodexBar token sync; all six keys used it; reviewed quota percentage fix deployed and verified |
| ws-0015 | Claude Code 503 after MacBook reboot | `fix/resp-subscription-tcp-timeout` | — | **deployed and verified** — bounded Linux subscription TCP acknowledgements, all-six-key and five-node native proofs; system key traced |
| ws-0016 | Chip Claude subscription | configuration + fleet record | — | **done** — Chip shared with all users ahead of Microsoft; fifth CodexBar account with token sync; all-six-key and five-node proofs |
| ws-0017 | Moxy Claude subscription | configuration + fleet record | — | **done** — immediate shared scope activation, sixth CodexBar account and sync; actual Moxy selection for all six keys across five nodes |
| ws-0018 | CodexBar follows CPA Claude routing | `fix/codexbar-claude-routing-follow` | — | **installed and verified** — reviewed PR #15, 17 Ark tests; Mac/key-scoped successful usage selects menu account; Microsoft fallback added locally |
| ws-0019 | Shared Claude account 82 | configuration + fleet record | — | **done** — shared scope and latest user priorities preserved; eight CodexBar accounts; visible menu reordered; timestamp-compatible sync runtime; actual 82 inference across all five nodes |
| ws-0020 | Claude priority and node availability | official CPA 8.0.27 + agent-os enrollment pins | — | **deployed and verified** — captured cancellation regression fails on 8.0.20 and passes on 8.0.27; all five nodes updated, all six keys checked; priorities and genuine quota/private-provider fences preserved |
| ws-0021 | Codex reauthorization on original credential | configuration + fleet record | — | **done** — fresh OAuth fields transferred onto original UUID; duplicate removed; settings/access and 98,290 historical rows preserved; all six keys and four models pass; active entry and fresh quota verified |
| ws-0022 | Automatic Claude plan badges | `fix/claude-plan-badges`, PR #16 | — | **deployed and verified** — seven Max 20x and one Enterprise after 82's automatic upgrade detection on 14 October; full Ark tests/build and reviewed merge; all six keys and five nodes passed deployment; settings and 64 panel assets preserved |
| ws-0023 | Account 82 routing after Max 20x upgrade | operational verification + fleet record | — | **resolved and verified** — earlier 429 cooldown cleared during investigation; native Sonnet/Opus select 82 over Microsoft; all six original keys select available higher-priority Gmail; settings/history preserved and temporary verification scopes removed |
| ws-0024 | The Vanderblooms Claude subscription | configuration + fleet record | — | **done** — Pro account shared with all users, priority 3 after Lara and before Microsoft; ninth CodexBar account with token sync and automatic following; actual Sonnet selection across six keys/five nodes and Opus success |
| ws-0025 | Hyper Rally Claude subscription | configuration + fleet record | — | **done** — Max 5x account shared with all users, priority 2 after The Vanderblooms and before Microsoft; tenth CodexBar account with token sync and following; actual Sonnet selection across six keys/five nodes and Opus success |
| ws-0026 | CodexBar unresponsive menu | local preference mitigation + fleet record | — | **mitigated and verified** — captured WebKit/audio main-thread blocks; optional OpenAI Web Extras and quota-warning sounds disabled; all ten Claude accounts and sync preserved; normal event-loop samples across refresh/sync observation |
| ws-0027 | Released fleet updates and Sol 1M in Claude Code | Home PR #17 + staged fleet/client rollout | — | **deployed and verified** — Home 1.1.1, five CPA 8.0.29 nodes, five Claude Code 2.1.300 clients, CodexBar 0.75; actual 900K Sol request and tool call pass; existing overview timeout confirmed on both images |
| ws-0028 | Max 20x badges and CodexBar email labels | scoped quota collection + local config | — | **done** — both provider-backed Max 20x snapshots fresh; ten email labels, OAuth usage and scheduled following pass; all six keys native Claude 200; locks released |
| ws-0029 | Second Codex subscription | configuration + local CodexBar helper | — | **done** — Gmail priority 2/shared, Hotmail 1; six keys/five nodes native Codex pass; both CodexBar accounts, access-only sync and automatic following verified |
| ws-0030 | Current model menus and automatic CPA patch currency | agent-os controller + fleet configuration | — | **done** — 55 shared current models selectable on four agents; five CPA 8.0.30 nodes; native Opus/Sol and six-key proofs; hourly catalog and guarded patch automation verified |
| ws-0031 | Home-native fleet maintenance | CPA Home fleet service + certificate-bound local workers | — | **done** — Home owns policy/scheduling/history; four mini tasks verified with laptop coordinators absent; native inference/scopes/panel preserved |
| ws-0032 | Third Codex account: HypeSports | database routing + local CodexBar mapping | — | **done** — Gmail 3/HypeSports 2/Hotmail 1; shared group 1, six original keys and new-account proofs on all five nodes; three-account OAuth/scheduled sync pass; temporary access removed and maintenance restored |
| ws-0033 | Fourth Codex account: Developer | database routing + local CodexBar mapping | — | **done** — Gmail 4/HypeSports 3/Developer 2/Hotmail 1; shared group 1, six original keys and new-account proof on all five nodes; fresh quota/four-account OAuth/scheduled sync pass; temporary access removed and maintenance restored |

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
- 2026-10-01 — CodexBar solved inside CodexBar: two Claude `sessionKey` accounts + Stacked layout; verified via the codexbar CLI and confirmed by Chanse.
- 2026-10-06 — ws-0009: `acct-63b680` Claude moved from group 1 to group 2 ("Chanse only — Copilot + Microsoft Claude", key 6 only) with priority -1, so it serves only the MacBook and only while Hype + Gmail are exhausted. Config-only, under all four locks; agents verified fenced (Chip 429 cooldown on Claude, 200 on GPT).
- 2026-10-06 — ws-0009 AC4 proven: at Hype's 21:00Z reset the MacBook moved off the Microsoft account within 4 s (606 Microsoft rows 20:30–21:00Z, then 205 Hype rows, 0 Microsoft, 0 failed). Hype set to priority 1 at 20:07Z so accounts are used in order; that proof waits for Gmail's 01:00Z reset.
- 2026-10-07 — ws-0009 done: after Gmail's 01:00Z reset all MacBook Claude traffic stayed on Hype (6/6 rows, none on Gmail or Microsoft), so the order Hype → Gmail → Microsoft holds.

## 2026-10-07 — Claude Code 503 incident follow-up

Status: mitigated and verified at 02:19Z — index applied, all six active API keys passed
streaming Claude proofs, 61/61 catalog checks passed across subsequent maintenance cycles.

- Earlier remote incident: an orphaned MacBook subscription socket retained an active Home
  membership and rejected replacement subscriptions with `CPA certificate is already owned
  by an active membership`. Clearing that exact stale socket restored service at 00:32:38Z.
- At-home recheck: Sonnet and Opus streaming requests passed, but a fresh outage occurred at
  02:09:59Z and recovered at 02:10:03Z. All five memberships reconnected together. Home logged
  a 4.29 s usage-accounting maintenance scan and delayed heartbeat SQL at that same time;
  its lifecycle heartbeat timeout is 3 s. This is an additional failure cause, not evidence
  that the earlier orphaned socket returned.
- `firstPendingUsageTokenAccountingRecord` in `internal/cluster/token_accounting_backfill.go`
  orders pending records by ID. SQLite chose `SCAN usage` despite the version index. The
  maintenance loop in `cmd/home/main.go` repeats this check every minute even after migration
  is complete, holding Home's sole SQLite connection (`internal/cluster/db.go`). A read-only
  reproduction took 3.06 s; all 150,354 rows were already version 2.
- Under all four canonical fleet-locks, created a `VACUUM INTO` backup with nohup, verified its
  full integrity and SHA-256, then added `idx_usage_pending_token_accounting_v2` at 02:15:26Z.
  Backup, SQL and rollback are recorded in `FLEET.md`. Before/after query results were both
  empty; the plan changed to the new partial index and latency fell from 3.18 s to 0.027 ms
  (first execution). Creation took 1.58 s. A synthetic legacy-row check passed before applying.
- At 02:15:49–53Z, every active API key (IDs 1–6) completed a streaming
  `claude-sonnet-5-5` request through the MacBook node with HTTP 200 and reply `ok`. No secrets
  were printed or saved in the repository. All locks were released. Home remains on
  `cpa-home:1.0.73-claude-fleet-16f609d`, with its original container start time and zero restarts.
- Monitoring from 02:15:59Z to 02:19:03Z: 61/61 authenticated catalog probes returned HTTP 200,
  maximum response time 55.38 ms. Home had no further slow-query, heartbeat-liveness or
  fingerprint-fencing warnings across the subsequent minute maintenance cycles; all five
  membership lifetimes remained stable. A post-index Opus streaming proof also returned 200
  and completed with `ok`.
- Follow-up: add the index or an equivalent efficient existence query to the application
  migration path for fresh databases. The earlier orphaned-socket liveness issue was subsequently addressed by ws-0015
  after its reboot-triggered recurrence on October 11.

- 2026-10-08 — ws-0011 deployed and verified at 04:45:45Z: Home fork image
  `cpa-home:1.1.0-claude-fleet-9821c83`, all five CPA nodes 8.0.20 `0f96f568`.
  Reviewed source PRs #6/#10, full Ark Go suite/CGO build, exact 52-table offline migration and
  old-image rollback rehearsal, 64 preserved panel assets, all-six-key provider/usage proof,
  Copilot channel fences and five healthy plugin reports. Fresh native Codex shell-tool session
  uses CPA without daemon warning; native Claude Code selects Gmail. Four concurrent 24-hour
  dashboard panels pass while all nodes remain healthy. Seven-day overview timeout also
  reproduced on the old build and remains documented. See
  [upgrade record](ws-0011-upstream-fleet-upgrade.md) and
  [plain-language HTML guide](ws-0011-updates.artifact.html).

- 2026-10-10 — ws-0014: Developer Claude bound to shared scope 1; all six keys used it.
  Added the fourth CodexBar account and automatic access-token mapping. Fixed and reviewed
  Claude quota percentage conversion (PR #13); Home image `cpa-home:1.1.0-claude-fleet-de715bd`
  deployed 03:22:35 UTC. Full Ark tests/build, 64 matching panel assets, fresh verified backup,
  six post-restart native requests with Home attribution, five healthy nodes/reported_ok,
  and released locks. HTML guide includes the verified UI sharing steps.

- 2026-10-11 — ws-0015 deployed at 05:49:04Z: Home `cpa-home:1.1.0-claude-fleet-16e6971` bounds Linux subscription TCP acknowledgements using the existing heartbeat timeout. MacBook reboot left an old membership blocking reconnects until 05:30Z; CPA heartbeat middleware explains the empty 503. PR #14 reviewed clean, merged-tree full tests and compile pass, isolated blackhole closes in 3.213 s. Fresh 4.4 GB backup integrity/SHA verified, all four locks renewed/released, six Claude keys + Codex/Copilot scopes + five native nodes + fresh Claude Code pass; 50/50 follow-up checks and stable memberships. Unassigned key 1 is Ark/Hyper’s pilot readiness key. Backup, image rollback, exact evidence, and guide update: [ws-0015](ws-0015-claude-reboot-reconnect.md).

- 2026-10-12 — ws-0016 verified at 13:13 UTC: Chip (`acct-f0ee78`) bound to shared scope 1, priority 2 preserved ahead of private Microsoft. Added fifth CodexBar account and exact token-sync mapping; all five OAuth usage reads and scheduled sync pass. Fresh integrity-checked backup, all six client keys and five nodes verified, all locks released.

- 2026-10-12 — ws-0017: Moxy (`acct-1edb83`) immediately bound to shared scope 1 at priority 2 alongside Chip, ahead of Microsoft. Six-account CodexBar OAuth reads and automatic sync pass. Real inference attribution confirms Moxy for every key and all five nodes. Targeted pre-change snapshot and inverse API rollback used to prioritize immediate restoration; existing full verified backup retained. All locks released.

- 2026-10-12 — ws-0018 installed and verified at 16:21 UTC: reviewed PR #15 merged, new local helper follows latest successful MacBook/key-6 Claude request and selects CodexBar by UUID without waiting for token rotation. Seven OAuth accounts including private Microsoft pass; default CodexBar usage and automatic sync follow Moxy. No Home/node configuration changes.

- 2026-10-13 — ws-0019 done: 82 (`acct-72bae8`) shared through binding 15, latest user priority 8 preserved. Eight-account OAuth usage and sync pass; 82 is now in the visible CodexBar prefix. Traced six-account stacked-menu cap and Python 3.9 variable-timestamp rejection; updated local menu order and used existing Python 3.14 for sync. Earlier provider quota exhaustion cleared by final verification; real 82 usage confirmed for all six keys and five nodes.

- 2026-10-15 — ws-0027: reviewed Home PR #17 merged; Home 1.1.1 fleet image, all five CPA 8.0.29 nodes, five Claude Code 2.1.300 clients and Mac CodexBar 0.75 deployed. Sol high/1M picker verified with actual 900,341-token request, recorded Bash call/result, all six scopes and all five nodes. Original settings/priorities/permissions and ten CodexBar accounts retained. Exact 539 build inputs and reproducible immutable image verified. Two guarded image rollbacks established the existing overview timeout and resolved a nonsemantic Go module annotation discrepancy; final image passes native acceptance and dashboard baseline parity. Fresh integrity/SHA backup retained, all four locks released. [Record](ws-0027-latest-sol-context.md) · [Plain-language guide](ws-0027-updates.artifact.html).

- 2026-10-15 — ws-0028: refreshed provider-backed Max 20x badges for Vanderblooms and Hyper Rally after independent profile verification and fresh integrity/SHA-verified backup. Renamed all ten CodexBar Claude accounts and sync labels to their mapped email addresses, preserving UUIDs, tokens, selection, priorities and scopes. All ten usage reads, scheduled sync/follow and six-key native Claude requests pass; five nodes healthy; four locks released. [Record](ws-0028-max20-email-labels.md).

- 2026-10-15 — ws-0031: moving fleet maintenance into Home after the user rejected laptop-owned
  scheduling. Home's existing three-hour registry refresh remains authoritative. Native DB-backed
  typed tasks and certificate-bound workers are deployed and verified on all four minis without
  laptop coordination. Larger releases require Home approval. See ws-0031-home-native-fleet.md.
