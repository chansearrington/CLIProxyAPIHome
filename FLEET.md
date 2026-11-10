# FLEET.md — how this fork is used by the agent-os fleet

This is Chanse Arrington's fork of CLIProxyAPIHome. It runs one instance of Home on the Ark (an
Unraid server) as the central model router for the agent-os fleet. Everything upstream's
`AGENTS.md` says still applies; this file only adds the fleet-specific rules.

## Branches

- `main` and `dev` mirror upstream (`router-for-me/CLIProxyAPIHome`). Never commit fleet changes to
  them; fast-forward them from `upstream` only.
- `fleet` is the deployed line. It is `dev` plus the fleet's own fixes, rebased forward as upstream
  moves. The live image on the Ark is always built from a commit on this branch.
- A change meant for upstream gets its own `feat/…` or `fix/…` branch off `dev`, and a PR against
  upstream `dev`. Open upstream PRs from this fork today: #115, #116, #123 (ws-0002 plugin-login fix) and #124 (ws-0004
  plugin model-discovery fix, fleet commit `95ba2db`).

## Building and testing — the Ark only, always inside Docker

- No Go toolchain is installed on the laptop, and none should be. `go build`, `go vet`, `go test`
  and `gofmt` run only inside the `golang:1.26-bookworm` container on the Ark.
- Source, Go build cache, module cache and an output dir live on the Ark's data array under
  `/mnt/user/appdata/cpa-home-build/{src,gocache,gomod,tmp}` and are bind-mounted into the
  container by `/mnt/user/appdata/cpa-home-build/run-go.sh '<command>'`. That keeps every build
  artifact off the Ark's small Docker disk image (measured 2026-09-16: a full gofmt + vet + test +
  compile run grew the Docker disk by zero).
- Inside the container `/tmp` is an exec-enabled tmpfs (`--tmpfs /tmp:exec,size=3g`; 1g ran out of
  room linking cgo test binaries in parallel on 2026-09-30, measured in ws-0002), not a bind
  mount. Go's test temp dirs on the bind-mounted Unraid array (a FUSE share) hit "directory not
  empty" on cleanup and produced three false FAILs in `internal/cluster` on 2026-09-16; on tmpfs the
  full suite is clean. Docker's default tmpfs is `noexec`, which breaks `go test` outright, so the
  `exec` option is required. Write compile output to `/out/<name>` (the array), never to `/tmp`.
- `go vet ./...` reports exactly one finding on `fleet`: `internal/cluster/refresh.go:172:2:
  unreachable code`. It is upstream's (a duplicated `return` after an if/else), untouched by this
  fork, and present on `dev` too. Do not fix it here; a vet run whose only line is that one passes.
- The Dockerfile compiles with `CGO_ENABLED=1` (glibc-linked). A binary compiled outside the
  Dockerfile must use the same builder image and run on `debian:bookworm`; a static build is a
  deliberate change that must be documented, not an accident.
- The management panel is not in this repo (`internal/managementasset/static/` holds only a
  `.gitkeep`). A deployable image needs the panel mirrored from the running instance first. A binary
  compiled from a bare checkout ships a blank panel and is a smoke test only, never a deploy.
- `docker build` on the Ark is gated on the Ark's Docker disk having enough free space. The gate,
  the tag convention (`cpa-home:<upstream-version>-claude-fleet-<short-sha>`, Chanse 2026-09-24; earlier images used `1.0.72-claude-<line>-<sha>`) and the rollback procedure are
  in the agent-os repo: `docs/runbooks/cpa-home-build-and-rollback.md`.

## Deploying

- The Ark's `cpa-home` container is live for the whole fleet. A new image is deployed only on a
  decided `ship` card in the fleet Inbox, following the runbook above. Never restart or replace it
  ad hoc.
- Never stop the fleet's `cpa-home-node` services on the minis; restart in place only.

## Plugins (Home-managed)

- Plugins reach the nodes through Home's config (`plugins.configs.<id>` with a pinned `store`
  manifest); every node downloads and loads them, so access is fenced with channel groups, not
  per node. The GitHub Copilot plugin is the fork `chansearrington/cliproxyapi-copilot-plugin`
  (ws-0002; its `FORK.md` lists the patches it carries).
- Nodes run CPA **8.0.64** on all five machines (verified native requests, running-process
  banners and artifact hashes on 2026-11-10; ws-0047). On CPA ≤ 8.0.4, adding a new
  plugin while nodes ran made every node loop on "installed but not loaded" (CPA issue #6225);
  8.0.5+ carries the fix (`bfa5aed`). It is proven by upstream's regression tests, not yet by a
  live plugin add — watch the first one, and if a node still loops, restart it in place. Bumping
  the version of an already-loaded plugin hot-reloads cleanly with no restart.
- After a node restart, Home's `GET /nodes` shows a plugin as `skipped loaded` (library on disk
  already identical); that is healthy. Check for `loaded` + `reported_ok`, not `installed`.

## Live SQLite maintenance index

- On 2026-10-07, the usage-token-accounting maintenance query was measured at **3.18 s** on
  the Ark's approximately 150,000 usage rows, all already at accounting version 2. It scanned
  the full table every minute despite the existing version index. Home uses one SQLite
  connection; the scan delayed subscription heartbeats beyond the 3 s lifecycle timeout,
  disconnecting nodes and causing Claude Code 503s.
- The live database now has this additional index, applied under all four fleet-locks without
  restarting Home or changing its image:

  ```sql
  CREATE INDEX idx_usage_pending_token_accounting_v2
    ON usage(id) WHERE token_accounting_version <> 2;
  ```

  The parameterized maintenance query uses it, returns the same result, and took **0.027 ms**
  on the first check after creation. A synthetic check also verified that a late legacy row is
  still found. This is a persistent database tuning change, not yet an application migration;
  retain it when restoring or replacing the database, and reassess it when the accounting
  schema version changes. Rollback is `DROP INDEX idx_usage_pending_token_accounting_v2`.
- Pre-change backup on the Ark:
  `/mnt/user/appdata/cpa-home/data/backups/home-pre-pending-token-index-20261007T021207Z.db`,
  3,461,537,792 bytes, full integrity check `ok`, SHA-256
  `595c33ef8bb1baec302286c5719d035120f2a7d3da5a48e9cff4572c4987fc97`.

## Account additions

- User-selected rule (2026-11-07): give a newly added account the next highest priority
  number for its provider (`max(existing priorities) + 1`). Higher numbers run first.
  Leave every existing account's priority unchanged; do not renumber accounts to insert
  a new one near the fallback. Add shared accounts to channel group 1 and the existing
  CodexBar access-token sync, preserving Home's sole refresh-token ownership.

## Backup retention

- User-selected scope (2026-11-07): configuration-only changes use a targeted backup of
  the affected settings and access bindings, with a scoped rollback. Do not copy the
  full usage/history database for routine configuration edits. Full database snapshots
  remain appropriate for image upgrades, schema migrations and database maintenance.
- User-selected rule (2026-11-07): keep only the two newest verified Home database
  backup versions in `/mnt/user/appdata/cpa-home/data/backups`, across all operation
  names. Keep at most two versions of each other backup/archive type. A new database
  snapshot counts only after its integrity check and checksum complete successfully.
  Remove obsolete SQLite sidecars and logs with their superseded snapshots. This rule
  supersedes older workstream instructions to retain individual historical backups.

## Secrets

- Nothing in this repo may contain the Home management password, any node's `home_jwt`, an API
  key, or an OAuth token. Test fixtures use obviously fake values.

## Current verified runtime (ws-0037 account-status deploy, 2026-10-30 UTC)

- Home is `cpa-home:1.1.1-claude-fleet-897d9c6`, immutable image
  `sha256:c0bd56efb239136db6f2fad35235b59470e147dda48480d1a24fdae5fe925766` (ws-0037): non-blocking model errors
  no longer make an otherwise active account show error. The enabled quota-healthy account status data, all five
  nodes, both model families' real traffic and all 64 unchanged panel assets were verified after cutover. See
  [deployment evidence and limits](docs/fleet/ws-0037-account-status-deploy.md). The prior ws-0034 behavior remains:
  blocked credentials are
  re-checked hourly and a healthy check lifts the stale block; idle credentials are re-checked every 6 h and after
  a saved reset; container log keep is 10 x 50 MB. Current image-only rollback target is the immediately preceding
  `cpa-home:1.1.1-claude-fleet-c632a78`
  (`sha256:fe690fc3094b17741ba6e7c96c693006cf99be39f9cff5accdbc15a1f60a4863`).
  Source integrates released upstream 1.1.1 and preserves all deployed fleet patches, including Claude refresh safety,
  serialized SQLite configuration updates and four read-only dashboard connections.
  The ws-0014 quota correction treats Claude utilization consistently as percentages, so
  1% used no longer appears 100% used and exhausted.
  The ws-0015 Linux subscription transport bound prevents unacknowledged heartbeat data
  from keeping a dead node's membership alive through the kernel's long retry window.
- All five nodes report CPA 8.0.43 and healthy membership, with Copilot 0.3.7 loaded and `reported_ok`
  (Home `/fleet` and `/nodes`, read 2026-10-26 02:00 UTC). Home's automatic patch rollout carried them there
  from 8.0.29 (ws-0030, ws-0031); the last hand-verified restart-in-place rollout (8.0.29) passed each node's
  native Claude/Codex/Copilot completions, protected-key fences and Home usage attribution.
- Shared Claude order is Chip Outlook (13), Character8201 (12), Gmail (11), HypeSports (10), 82 (9), Chip (8), Developer (7), Moxy (6), Lara (5), The Vanderblooms (4), Hyper Rally (3), Hotmail (2); Microsoft (1)
  remains Chanse-only emergency fallback. All six keys and five nodes selected Hyper Rally in the ws-0025
  routing proof while higher-priority accounts were unavailable; native Opus also passed.
  Earlier proofs selected Moxy (ws-0017), Developer (ws-0016) and Lara (ws-0015).
  Eleven shared Claude accounts and the private Microsoft fallback are in CodexBar with
  Home access-token sync every 120 seconds; selection now follows the MacBook/key-6 latest
  successful Claude request. Home alone owns refresh tokens.
  On 2026-11-05, repaired the local Claude sync so expired Home access tokens cannot block
  authorized accounts. Nine Claude accounts and all four Codex accounts pass live usage;
  HypeSports Claude and Microsoft Claude are currently disabled/unauthorized in Home and need
  reauthorization there. All eleven Claude mappings remain and retry every 120 seconds.
  [CodexBar sync recovery](docs/fleet/ws-0039-codexbar-sync-recovery.md).
  The in-place OAuth Reconnect feature is deployed and verified on image `4cdbc4b`.
  [Verified artifact and cutover handoff](docs/fleet/ws-0040-credential-reconnect-acceptance.md).
  CodexBar is now 0.83.0; eleven Claude mappings are retained. Hotmail's OAuth
  usage and provider-backed Pro quota are verified. Its shared binding is 28; all six original
  keys pass Sonnet, and an isolated new-account key passed Sonnet/Opus on the four running nodes.
  Lara's separate post-upgrade setup was completed physically; her Workbench, gateway and CPA
  node are restored, all five memberships are healthy, and Lara passed native Sonnet through her
  own node (usage 1811514). [Hotmail acceptance and desktop recovery](docs/fleet/ws-0035-hotmail-claude-shared.md).
  The ws-0026 local freeze mitigation still disables CodexBar's optional OpenAI Web Extras and
  quota-warning sounds, avoiding captured WebKit/audio main-thread blocks. Silent warnings,
  all ten Claude account entries, token sync and automatic account following remain enabled.
  Fresh native Codex defaults to CPA, executes a real shell tool and shows no daemon mismatch.
- CPA 8.0.27 corrects the node's second availability check after Home dispatch. A canceled
  Claude request with no retry deadline no longer makes the node skip Home's eligible
  higher-priority credential. Higher numeric priorities run first; Microsoft at 1 remains last
  among eligible accounts. At the ws-0020 daytime acceptance all seven shared accounts were
  genuinely quota-blocked: all six keys passed native Codex; Chanse's Claude/Copilot backup
  requests passed and the other five keys preserved their quota/private-provider fences.
  [Verified correction and rollback record](docs/fleet/ws-0020-claude-priority-node-filter.md).
- SQLite's writer remains single-connection. Dashboard reads use a separate bounded read-only
  pool so they cannot queue scheduler/heartbeat operations behind analytics. The tested 24-hour
  overview now exceeds the unchanged query budget on both old and new images; the one-hour
  overview passes, and 24-hour aggregates, usage records and request events pass. This existing
  scaling limit remains a separate optimization follow-up, not a claimed update regression or fix.
- Retain the verified fresh production backup and each node's `cli-proxy-api.8.0.7.bak`.
  The historical ws-0011 image-only Home rollback target is immutable ID
  `sha256:76499632b410686ec14a68abbd26360369bd82cbace1ed77bc4617a2c207ba72`.
  The earlier major-upgrade rollback is recorded in ws-0011.
  Do not restore a stale database over newer rotating OAuth authorizations.
- Evidence: [upgrade record](docs/fleet/ws-0011-upstream-fleet-upgrade.md).
  Plain-language offline guide: [what changed](docs/fleet/ws-0011-updates.artifact.html).

- Latest account sharing, quota correction, backup, rollback, and UI instructions:
  [Developer Claude record](docs/fleet/ws-0014-developer-claude-shared.md).
- Reboot-related empty 503 diagnosis, reconnect fix, latest backup/rollback, and system key:
  [reconnect record](docs/fleet/ws-0015-claude-reboot-reconnect.md).

- Latest shared account, five-account CodexBar sync, backup and UI instructions:
  [Chip Claude record](docs/fleet/ws-0016-chip-claude-shared.md).

- Sixth shared account, immediate scope activation and actual Moxy routing proofs:
  [Moxy Claude record](docs/fleet/ws-0017-moxy-claude-shared.md).

- Automatic menu-bar account selection, private fallback mapping, and manual-mode instructions:
  [CodexBar routing follow record](docs/fleet/ws-0018-codexbar-follow-claude.md).

- Account 82, eight-account CodexBar setup, visible menu order, and timestamp runtime fix:
  [82 Claude record](docs/fleet/ws-0019-82-claude-shared.md).

- Codex renewal without losing settings, permissions or history:
  [identity-preserving reauthorization record](docs/fleet/ws-0021-codex-authorization-preserve-identity.md).

- Automatic Claude subscription badges, source mapping, validation and image rollback:
  [plan badge record](docs/fleet/ws-0022-claude-plan-badges.md).

- Current update, verified 900,341-token Sol request, client versions, panel, rollback and known dashboard limitation:
  [ws-0027 deployment record](docs/fleet/ws-0027-latest-sol-context.md).
  Plain-language [latest update guide](docs/fleet/ws-0027-updates.artifact.html).
  Claude Code now offers **GPT 6.1 Sol — High · 1M**, model `gpt-6.1-sol(high)[1m]`,
  with high reasoning and 900K automatic compaction. All five Claude Code clients are 2.1.300.

- Since 2026-10-15 15:30 UTC, Vanderblooms and Hyper Rally also have fresh **Max 20x**
  Home badges: all nine shared Claude subscriptions are verified Max 20x; private Microsoft
  remains Enterprise. All ten CodexBar Claude display names and sync mapping labels are
  their email addresses. Account UUIDs, tokens, ordering, priorities and permissions are
  preserved. [Plan/label verification](docs/fleet/ws-0028-max20-email-labels.md).

- Since 2026-10-18 (after the 05:53 UTC backup, ws-0033), shared Codex order is `acct-90c281` (Gmail, priority **4**, binding 23),
  `acct-f4ba26` (HypeSports, priority **3**, binding 24), `acct-fdfbfe` (Developer, priority **2**,
  binding 26), then `acct-126999` (Hotmail, priority **1**, original UUID/binding preserved); priorities
  re-read from Home 2026-10-26. At each addition all six original keys passed native Codex requests (served by the
  top account); at the third and fourth additions all five nodes also passed with a temporary key
  bound only to the new account.
  At ws-0033 (2026-10-18) CodexBar monitored all four with Home-owned access-token sync, following successful
  MacBook/key-6 Codex usage every 120 seconds (not re-checked since). Home alone owns refresh tokens; native Codex auth and all ten Claude
  mappings remain intact. Verification and rollback:
  [second](docs/fleet/ws-0029-second-codex-account.md),
  [third](docs/fleet/ws-0032-third-codex-account.md),
  [fourth](docs/fleet/ws-0033-fourth-codex-account.md) Codex account.

- Current model-menu/node currency: all four OpenClaw gateways expose the 55 shared current
  CPA models, including Opus 5.5 and Sol 6.1. All five nodes run CPA 8.0.43 (Home-reported, 2026-10-26).
  Home now owns catalog sync and guarded patch rollout through its DB-backed `/fleet` API;
  the initial laptop coordinator from ws-0030 is retired.
  User-confirmed defaults are Sol 6.1 for Moxy/Chip and Opus 5.5 for Lara/Hyper;
  corresponding fallbacks are updated. Existing session pins stay intact. OpenClaw core remains 2026.9.4.
  [Current Home automation and pause instructions](docs/fleet/ws-0031-home-native-fleet.md).

## Home-owned fleet maintenance

The laptop fleet coordinator is superseded by Home's DB-backed `/fleet` control plane and
certificate-bound node-local executors. See [Home-native migration](docs/fleet/ws-0031-home-native-fleet.md).
The always-on canary/required nodes continue while the optional MacBook sleeps. Model defaults
remain explicit operator choices; menu sync does not promote generations automatically.
Before manual fleet maintenance, pause Home fleet policy and let any active task finish/reconcile,
then follow the normal fleet-lock procedure. Do not install a competing laptop coordinator.

## November 6 Home recovery

Home 1.1.8 fleet image `87210da` is deployed and accepted. SQLite index creation had
exhausted the production container's 64 MB `/tmp`; Compose now places SQLite temporary
files on the data volume. All five nodes and all six keys passed live acceptance,
account/access/history preservation checks passed, and the saved maintenance policy
was restored. See [upgrade and recovery evidence](docs/fleet/ws-0041-home-118-recovery.md).

## November 7 account setup and backup policy

Character8201 (`character8201@gmail.com`) is shared at priority **12** (binding 30),
with every existing priority unchanged. All six original keys and all five nodes passed
native Claude requests attributed to the new account; Sonnet and Opus passed. CodexBar
has twelve Claude mappings and follows this account from MacBook/key-6 usage. Main and
fleet are synchronized with upstream main and pushed. Configuration-only changes now
use targeted configuration backups; the backup directory retains two verified database
snapshots and at most two versions of each other backup type. See the
[account setup and retention record](docs/fleet/ws-0042-character8201-claude-shared.md).

## November 9 urgent account addition

`chip.bullard@outlook.com` is shared at Claude priority **13** (binding 31), with all
existing priorities unchanged. All six keys passed Sonnet and key 6 passed Opus through
the MacBook node; all seven usage records identify the new account. Only a 13 KB targeted
configuration backup was created. Maintenance was restored and locks released. This request
was scoped to CPA Home. See the [account record](docs/fleet/ws-0043-chip-outlook-claude-shared.md).

## November 10 Fable 5.2 and Claude Code catalog sync

Fable 5.2 now serves through all five nodes and all six original keys. Home's DB-backed
`upstream.claude.header-defaults` sets the measured Claude Code 2.1.300 software baseline
(SDK 0.128.0, runtime v26.3.0), fixing the upstream 2.1.280 rejection without rebuilding nodes.
The override controls both software headers and the signed billing version. Do not diagnose
future version-gate failures from the fallback literals alone; inspect the configured baseline.

All five machines use fleet-worker bundle `80bc72a639456aed`. Home's existing catalog tasks
now reconcile Claude Code's picker alongside OpenClaw, including Claude-only laptops. Generated
entries select the newest native Fable/Opus/Sonnet/Haiku family from the caller's own model
catalog, excluding encoded provider aliases. Defaults and custom options are preserved;
operator edits to generated rows are retained. The worker does not edit first-party caches.
All twenty focused tests pass on Ark; native Claude Code confirms Fable 5.2 with a 1M window.
See the [reviewed plan and deployment evidence](docs/fleet/ws-0045-fable52-fix.md).

Home scheduled and accepted the new MacBook Claude-only catalog target at 2026-11-10
16:32:56 UTC (task `cc3787e9-10ce-4668-a3a4-992f6f499103`). Normal maintenance policy is
restored; all four operation locks are released and all five nodes remain healthy.

## Upgrade recovery requirement (November 10)

The user explicitly requires that upgrades never strand the agent behind a failed CPA Home.
Before a manually initiated disruptive CPA/Home cutover, positively test an independent direct
OpenAI/operator recovery route and keep it available through acceptance. The recovery controller
must use SSH/Docker/launchd independently of model inference. Rehearse production mount/tmpfs
constraints, preserve both SQLite temp-directory environment variables, and allow measured full
startup time. Verify credential-refresh journal/fence safety during rollback, not only database
readability. Do not copy Home-owned refresh grants into a local fallback client.

At the November 10 review, Home is 1.1.8 and all nodes are 8.0.58; stable targets are Home 1.1.9
(schema 17) and CPA 8.0.64. Moxy records repeated `rollback_not_ready` attempts for 8.0.61/63/64.
Diagnose and prove the updater/rollback path before another retry. This review did not change
runtime policy or deploy anything. See the [assessment](docs/fleet/ws-0046-upstream-review-nov10.md).


## November 10 upstream upgrade accepted (ws-0047)

Home **1.1.9** (`cpa-home:1.1.9-claude-fleet-44a3c27`, immutable image `41924ea4…`) and
all five CPA **8.0.64** nodes are deployed and accepted. Home PR #32 and agent-os readiness
PR #850 are merged and independently reviewed. A real Moxy rollback/re-upgrade, isolated
schema/startup rehearsals, 22 final native fleet requests with usage attribution, both native
Claude Code generations, panel assets, data preservation and direct OpenAI recovery passed.
The temporary Claude software-version header override was removed after node acceptance;
No account priority or access scope was changed. Maintenance is restored and parent locks released.

Rollback must retain the current database and rotating credentials. Before starting the old
1.1.8 refresh engine, disable automatic restart first, freeze/kill the candidate and wait until it is
stopped and unpaused, then verify that the live WAL-aware `codex_refresh_journal` is empty. Do not
resume the candidate between that check and the old-image start. A missing/unreadable/nonempty journal forbids that
old-image path; recover with the journal-aware new image. Never restore an older full database
snapshot over current authorizations. The two retained DB snapshots are Nov 10 pre-1.1.9 and
Nov 6 pre-1.1.8; ordinary configuration changes still require targeted backups only.

Full evidence, recovery launcher/expiry, first-attempt limitations and deployment-check corrections:
[`ws-0047-upgrade-119-8064.md`](docs/fleet/ws-0047-upgrade-119-8064.md).
