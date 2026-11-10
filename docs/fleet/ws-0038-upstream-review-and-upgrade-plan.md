# Upstream review and proposed Home upgrade

Status: investigation and plan only — no deployment, runtime configuration write, image pull,
service restart, node upgrade, source merge, or database migration performed.

User instruction, November 3, 2026: inspect upstream updates for CPA Home and the nodes,
assess whether they are worthwhile, and make a plan; do not perform the upgrade.

## Recommendation

**Integrate released Home 1.1.7 into the fleet fork, after migration/rollback and patch-preservation
checks. Keep nodes on their already-current CPA 8.0.53 and the Copilot plugin on 0.3.7.**

Home is materially behind upstream; its custom fleet patches make a stock-image replacement
inappropriate. The principal benefits are reliability, security hardening, correct quota
reporting and access visibility. This is a planned maintenance change, not an emergency response
to exhausted provider quotas. Genuine quota exhaustion will remain provider-controlled.

## Verified baseline and release availability

Checked November 3 around 17:28–17:42 UTC using read-only Management API calls, container
metadata, upstream GitHub releases/compare APIs and source inspection.

| Component | Deployed | Latest released | Decision |
| --- | --- | --- | --- |
| Home fleet fork | `1.1.1-claude-fleet-897d9c6` | upstream `1.1.7`, Oct 23 | Worth integrating |
| CPA nodes: Moxy, Hyper, Lara, Chip, MacBook | all `8.0.53`, same accepted binary hash | `8.0.53`, Nov 3 | Already current; no node rollout needed now |
| Fleet Copilot plugin | established deployed line `0.3.7`; node plugin reports healthy | fork release `0.3.7` | No plugin upgrade proposed |
| HMC frontend | existing embedded fleet panel | bundled separately during official Home builds | Include a verified matching Home panel bundle in the candidate |

Read from the running Ark container, not inferred from the stale FLEET.md baseline:

- Image tag: `cpa-home:1.1.1-claude-fleet-897d9c6`.
- Immutable ID: `sha256:c0bd56efb239136db6f2fad35235b59470e147dda48480d1a24fdae5fe925766`.
- Started October 30 at 06:39:43 UTC; zero restarts at review.
- Home fleet maintenance is enabled; latest recorded CPA candidate/accepted rollout is 8.0.53.
  The policy was not paused or otherwise edited for this review.
- All five memberships are healthy with required plugin reports `reported_ok`.
- The running SQLite database is approximately **26.6 GB**. `cluster.yaml` is present, but
  the live database inspected is SQLite. Do not assume PostgreSQL-only performance gains apply.
- Current routing uses `fill-first`. Session affinity is not established as enabled by this review.
- CPA 8.0.53's public tag and default branch are identical; there are no additional main-branch
  commits beyond that release at review. Home 1.1.7's tag and main are also identical.

Release sources:

- Home: https://github.com/router-for-me/CLIProxyAPIHome/releases/tag/v1.1.7
- Full Home delta: https://github.com/router-for-me/CLIProxyAPIHome/compare/v1.1.1...v1.1.7
- Nodes: https://github.com/router-for-me/CLIProxyAPI/releases/tag/v8.0.53
- Plugin: https://github.com/chansearrington/cliproxyapi-copilot-plugin/releases/tag/v0.3.7

## What is worth bringing in

| Change | Released in | Value for this fleet / qualification |
| --- | --- | --- |
| Correct xAI weekly-quota collection; ignore zero monthly windows | Home 1.1.2, PR 140 | **High, directly observed:** fixes the Grok exhausted `$0 / $0` badge while inference still works. Backend and matching HMC rendering should be validated together. |
| Bounded protocol-setup workers | Home 1.1.6, PR 163 | **High:** prevents a client stalled before sending bytes or completing TLS from blocking all Home accepts. Uses at most 128 setup workers and a five-second setup budget; established request timeouts are not broadly tightened. |
| RESP parsing limits and connection panic recovery | Home 1.1.7 | **High:** limits malformed frames and isolates handler failures. Check our authenticated fleet worker traffic against the limits. |
| SQLite write-lock acquisition before config reads | Home 1.1.3/1.1.4, PR 150 | **High:** reduces stale WAL-snapshot/concurrent-write failures; overlaps our configuration-safety concerns. |
| Per-client-key model listings; cleanup of deleted group bindings | Home 1.1.3/1.1.4, PR 151 | **High:** aligns model menus with actual key permissions. Must preserve the shared/private provider fences and our six original keys. |
| OAuth exclusion cleanup and runtime-projection persistence fixes | Home 1.1.2 onward, PRs 131/139/144 | **Medium/high:** avoids baking derived exclusions into saved credentials. Requires migration and import/export compatibility tests. |
| Lower lock contention, batched queries and shared cluster broadcasts | Home 1.1.6 | **Useful:** potential scheduler/heartbeat efficiency. Measure on SQLite and do not substitute PostgreSQL benchmark claims for evidence. |
| Usage, token and billing display fixes; log search/cursors | Home 1.1.2–1.1.6 | **Useful:** clearer diagnosis and accounting. The usage-overview aggregation optimization is explicitly PostgreSQL-oriented; a fix for our existing SQLite 24-hour overview timeout is not established. |
| Claude quota metadata/reset-credit visibility; catalog-source configuration; newer image-model definitions | Home 1.1.2–1.1.5 | **Useful:** quota/menu visibility. Some Claude plan functionality is already present in our fork; reconcile duplicate functionality instead of stacking both implementations. Do not spend a reset credit during validation. |
| DB-backed session affinity; automatic/latency-aware CPA-to-Home rebalancing | Home 1.1.6/1.1.7 | **Lower immediate priority:** optional affinity should retain the current setting. Rebalancing helps multi-Home topologies and is different from our Home-owned binary updater. Do not change account-routing strategy or topology as part of this upgrade. |
| Antigravity OAuth/project discovery and collector fingerprint fixes | Home 1.1.3–1.1.6 | **Conditional:** may help future investigation, but do not reenable the intentionally disabled account automatically. |

Directly relevant upstream explanations:

- Grok quota: https://github.com/router-for-me/CLIProxyAPIHome/pull/140
- Idle connection accept blockage: https://github.com/router-for-me/CLIProxyAPIHome/pull/163
- Latency rebalance: https://github.com/router-for-me/CLIProxyAPIHome/pull/167

### Node changes already received

Today's CPA 8.0.53 release includes disabled/rejected-token refresh-loop prevention,
Fable-only 429 scoping, preservation of disabled plugin credentials, plugin library crash
preflight, Claude/Gemini tool-result image translation fixes, and `max_output_tokens`
forwarding. These are already on all five nodes through the existing Home-controlled rollout;
do not schedule a redundant upgrade or assume they are still missing.

## Integration risks established by the source review

An isolated `git merge-tree` preview of deployed commit `897d9c6` with released tag
`a8d691bc3782fc8500e9de166375bff6e6ffed7d` found **nine conflicting files**. The preview
created objects only in a temporary bare repository; it did not merge or edit the working
source branch. Conflicts are in:

- `cmd/home/main.go`.
- `internal/cluster/repository.go`, `internal/cluster/quota_snapshots.go`.
- `internal/cluster/management/quota_recollect.go`.
- `internal/quota/collector.go`, `collector_test.go`, `providers.go`.
- `docs/management/api.md` and `api_CN.md`.

The concentrated quota conflicts are exactly where the fleet's useful recovery behavior can
be accidentally lost. Textually clean merges elsewhere do not establish semantic correctness.

Other concrete compatibility gates:

1. **Schema/snapshot definitions advance from 6 to 12.** Upstream adds indexes, database-backed
   session affinity, rebalance and latency runtime tables. It says portable table shapes still
   match v6, but an old binary's acceptance of the new database/export format must be tested.
   The live SQLite schema-migration table has no version row; do not describe it as a recorded
   PostgreSQL schema version or skip SQLite AutoMigrate testing.
2. **Home's SDK dependency moves from CPA 8.0.4 to 8.0.33.** Use the upstream release's tested
   dependency first. It need not equal the separately deployed node version 8.0.53. Validate
   protocol compatibility before considering any further SDK bump.
3. **The pending-token-accounting index already exists under a fleet-specific name.** Upstream
   adds an equivalent partial index with another name. Decide how to avoid duplicate indexes
   through a proper migration; retain query-plan protection for heartbeat-sensitive writes.
4. **RESP limits are new.** Authenticated frames/bulk values have a 64 MiB budget and arrays a
   1,024-element bound; unauthenticated limits are much tighter. Exercise snapshots, config,
   logs, mTLS handshakes and fleet worker commands under these limits.
5. **The embedded panel must be intentional.** Official Home builds clone the separately
   maintained Home-Management-Center and embed its output. Home 1.1.3 explicitly shipped an
   incomplete frontend and 1.1.4 corrected it. Use the complete panel from a pinned, verified
   official 1.1.7 build, or build HMC from a recorded compatible commit. Do not clone today's
   floating panel main and call that a reproducible 1.1.7 bundle.

## Proposed implementation and rollout plan — not executed

### 1. Prepare an isolated candidate

- At execution time, reread the live image and current `origin/fleet`; preserve the existing
  uncommitted fleet documentation. Work on a dedicated branch/worktree from that exact fleet
  baseline and record the upstream release tag/hash. Follow the current fleet ship-card and
  local PR-review procedure; this review is not a ship authorization.
- Integrate Home 1.1.7 as one reviewed candidate. Resolve the nine conflict areas deliberately.
  Do not replace the custom image with the stock upstream image.
- Preserve and test these fleet behaviors explicitly: Claude refresh safety and unified
  rate-limit handling; stale quota-block lifting; hourly blocked/6-hour idle/reset-triggered
  probes; truthful nonblocking/disabled account-status display; SQLite write serialization
  and bounded dashboard reads; plugin OAuth UUID identity and complete-auth discovery;
  private Copilot/Microsoft scopes; protected `__home:fleet:` state, scoped mTLS worker commands,
  serial leases/receipts and restart acceptance. Keep the laptop coordinator retired.
- Use the upstream-compatible Home SDK pin first. Preserve current account UUIDs, priorities,
  scopes, labels, native logins and routing/default-model choices. Record a complete matched
  HMC asset manifest in the image build inputs.

### 2. Validate on Ark before scheduling production work

- All Go formatting, focused tests, full suite and compile run on Ark in the established
  `golang:1.26-bookworm` wrapper, array caches and 3 GiB executable tmpfs. No Go toolchain on
  the laptop. Relevant protocol/cluster/auth/quota concurrency tests should include races
  where practical; run any upstream PostgreSQL integration subset separately if required,
  without implying the live fleet uses PostgreSQL.
- Use a fresh detached database copy, privately and without production listeners, live node
  certificates or provider refresh/network access. Rehearse migration from the current SQLite
  shape, measure migration duration/disk/WAL growth, verify integrity, and compare bounded
  usage-history/account/config fingerprints before and after.
- Test old-image operation against the migrated copy, including config/token writes and
  import/export/snapshot-format compatibility. Produce an explicit compatible rollback
  procedure, or a forward recovery build if the old image cannot use the migrated database.
  Do not treat an untested old-image swap as a rollback guarantee.
- Validate fresh real quota fixtures and deterministic regression cases for the Grok weekly
  window, all-zero monthly skip, Claude plan/reset metadata, optional Codex details, stale
  cooldown lift, disabled accounts and nonblocking model failures. Genuine exhausted quotas
  must remain blocked.
- Verify full HMC management/user/login pages, credential/quota display, account/key editing,
  runtime-log pagination, model menus and existing CodexBar API projections. Measure 1-hour
  and 24-hour overview behavior against baseline; require no regression and report the
  existing SQLite limit honestly.

### 3. Establish production guards when deployment is separately authorized

- Pause Home maintenance only after its current task finishes/reconciles. Acquire all four
  canonical locks with fresh `acquired:` ownership and independently maintain heartbeats.
  Do not steal or proceed on an unreachable host. Make this future pause explicit; no pause
  was made during this investigation.
- Create and verify a fresh detached `VACUUM INTO` backup immediately before the change,
  recording integrity, byte size, mode and SHA256. Keep ample array space for source, image,
  the approximately 26.6 GB live DB, rehearsal copies, backup and migration/index overhead.
  A prior backup does not capture the freshest rotating provider authorizations.
- Pin the then-current image by immutable ID, preserve compose/log-retention settings and
  the existing panel bundle. Build reproducibly as `cpa-home:1.1.7-claude-fleet-<candidate-sha>`
  and verify source/assets/image hashes. Recheck Ark Docker-image storage before building.
- Set a realistic interruption estimate from the migration rehearsal. There is one shared
  Home control plane, so do not promise zero downtime or reuse the previous 33-second
  restart duration for an unmeasured migration.

### 4. Cut over Home once, then prove the fleet

- Deploy the approved Home candidate without changing any node version. Check startup,
  migrations, image provenance, panel health and log errors before broad acceptance.
- Require five healthy mTLS memberships and plugin reports; run native Claude and Sol
  completions through all six original client keys and each node's loopback listener, plus
  MacBook-only Copilot/private-provider checks. Confirm Home usage attribution and unchanged
  access fences. Verify correct behavior for quota-exhausted accounts using deterministic
  fixtures instead of clearing real quotas to force a live test.
- Confirm new Grok weekly quota reporting matches provider observations and live inference,
  and no zero-dollar window produces a false exhaustion badge. Check account priorities,
  credential identities, optional Codex quota details and original usage/history bounds.
- Exercise isolated worker/catalog acceptance and a maintenance lease/reconciliation cycle;
  ensure no second coordinator is introduced. Resume the saved Home fleet policy and release
  locks after acceptance. Observe a subsequent scheduled check and idle/reset quota cycle.

### 5. Rollback / stop conditions

- Stop rollout for credential refresh loops, missing/changed scopes, UUID/history loss,
  broken RESP/worker protocol, missing HMC assets, unexplained quota changes, migration errors,
  or scheduler/heartbeat regression. Preserve diagnostic evidence privately.
- Use the database-compatible image rollback proven during rehearsal, preserving live tokens.
  If incompatibility remains, deployment is a no-go until a forward recovery path is ready.
  Never restore a stale whole database over newer rotating authorizations as an ad hoc fix.
- Recheck release availability before execution. If a new node patch exists then, review it
  as a separate change and let Home canary it serially; do not combine an untested node release
  with the Home migration. Major/minor node upgrades continue to require explicit review.

## Investigation evidence and limits

Private source/release evidence is under `/tmp/cpa-upstream-review-20261103/`: published
release JSON, authenticated API comparisons, a temporary shared bare repo, merge-preview
conflict list and patch-overlap listing. Only read-only production API/container/SQLite
inspection was used. No build/test result is claimed for a merged candidate: no conflicts
were resolved and no candidate was compiled, tested or deployed during planning.
