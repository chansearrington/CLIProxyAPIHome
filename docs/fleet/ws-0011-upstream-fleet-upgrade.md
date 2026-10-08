# Home and CPA fleet upgrade

Status: deployed and verified across Home and all five nodes, 2026-10-08 04:45:45 UTC.

Chanse authorized bringing the available Home and CPA updates into the fork, testing them,
and deploying across the fleet. Targets are upstream Home 1.1.0 (`c098d84`) and CPA 8.0.20,
plus the Home Claude refresh-safety port `643f6c4`. Both release targets were rechecked against
GitHub's latest release endpoints before starting.

This instruction explicitly authorizes deployment; the fleet's locks, backups, rollback,
preservation, and proof requirements still apply. Go tools run only on the Ark inside Docker.
One live change runs at a time. Nodes restart in place, one at a time.

## Required preservation and verification

- Preserve all fleet source behavior, especially Claude reset handling and sequential priority,
  quota storage, Copilot plugin OAuth/discovery/quota, and Microsoft channel isolation.
- Preserve the live partial usage-accounting index, fresh Gmail/HypeSports authorizations,
  all six API keys, credential UUIDs, priorities 3 → 2 → 1, and channel groups.
- Resolve Home V8 and shared configuration layout changes against the existing runtime contract.
- Test formatting, the full Go suite, vet, and CGO compilation on the Ark. Review the integrated
  diff and explicitly check Home/node protocol and plugin compatibility before deployment.
- Mirror and verify management panel assets before building a deployable Home image.
- Verify a fresh full database backup, compose/config backups, and the current image ID before
  cutover. Rehearse database migration and rollback compatibility without allowing a shadow Home
  to refresh or use copied live OAuth credentials.
- After each live change, prove native Claude and Codex requests, all six active API keys,
  Copilot access and fencing, healthy membership/plugin reports, and usage attribution.
- Canary on the MacBook, then a mini, then the remaining nodes individually. Roll back a failed
  proof before investigating further.

## Source and rollout evidence

Integration, rollback rehearsal, canary and final deployment evidence follows. The corrected final acceptance gate passed.

### Integration and validation before cutover

- Fork PR [#6](https://github.com/chansearrington/CLIProxyAPIHome/pull/6) merged at
  `0fa4486deae0912b88edaf29e38470a03f1bf0d4`. Its tracked tree is identical to reviewed/tested
  `5783d16ab120deac2d2f45057042f06db427202a` (`4c8b1492324ae1c62e42105a9b0e4bed4dbe5641`).
  An ancestry-only merge records the previous deployed fleet after the upstream rebase; no file
  content changed after review. All 81 fleet patches were retained.
- Local adversarial review: **MUST 0**. Nonblocking warning-state and sibling-cooldown findings
  remain tracked in fork issues #7, #8, and #9; optional weighted/session-affinity settings remain
  unenabled. The built-in strict-priority behavior is intentional.
- Full Go suite and formatting pass on Ark Docker. Concurrent configuration regressions pass
  50 repeats; all four read-pool tests pass 20 repeats. Restoring old behavior makes each new
  deterministic regression test fail. CGO compile succeeds. Vet reports only the existing
  `internal/cluster/refresh.go:172` unreachable-code finding.
- Additional fixes found during testing: reserve SQLite's writer before ordinary configuration
  reads (xAI moved outside OAuth scope upstream); isolate nine observability read paths on a
  bounded read-only WAL connection so slow dashboard queries do not occupy scheduler/heartbeat
  connection. PostgreSQL and in-memory SQLite behavior are retained.
- Image: `cpa-home:1.1.0-claude-fleet-5783d16`, ID
  `sha256:579e367a94798ef1b8452ed5517562c0e243c8ac8f2f4a06fb0e1819538516e5`.
  Built using the repository Dockerfile, CGO, and actual complete management panel mirror:
  64 files, 3,919,143 bytes, recipe-v2 digest
  `8c7fa7c22e5bc90c70402b9893e2991fcaa834edacd723a6863b3bd961dac9fd`.
- Fresh verified backup:
  `/mnt/user/appdata/cpa-home/data/backups/home-pre-v110-20261008T040330Z.db`,
  3,717,742,592 bytes, integrity `ok`, SHA256
  `3bc1a8def9cfb66ea62285cbaa0eaf46757729b6e898c6510b93c9066c75112c`.
  Includes the renewed Gmail and HypeSports OAuth chains. Private compose/env/cluster backups
  are under `/mnt/user/appdata/cpa-home-build/ws0011/private`.
- Pinned image-only rollback target:
  `sha256:7a92e9443088eee7a91461ad623791c8c21277c5aebec6a0de5c10897042981e`.
  No database restore is planned: retaining fresh rotating OAuth state is essential.
- Isolated new-image migration/export passes: all 52 tables, row contents/counts, schema and
  pending usage-accounting index unchanged; full integrity `ok`. Old-image readability/export
  succeeds; the final old-image comparison also passed with every table unchanged.
- Offline old/new runtime configuration and old-parser/new-payload reports are equal.
  Runtime config intentionally has zero embedded keys; the separate active API-key table has
  all six original keys and scopes, independently checked.
- Baseline: 18 successful authorized Claude/Codex/Copilot requests correlated to Home across
  all five nodes and six keys; five deliberate Copilot denials preserve Chanse-only isolation.
  Every Claude completion used Gmail. Fresh native Codex session
  `01a119bb-db2a-7a00-860d-a0d35b0088f5` used a shell tool to read an unpredictable marker;
  Home usage 171321/171322 confirms `POST /v1/responses`, MacBook node, no failures, no daemon
  incompatibility warning.
- CPA artifact: official plugin-enabled darwin arm64 8.0.20, commit `0f96f568`, built
  `2026-10-07T19:02:10Z`. Archive SHA256
  `abb68051528506076561298ae3c4f3797c360f1d37127c2e459afdecce454df0` matches release digest and
  checksums.txt; extracted executable 63,816,002 bytes, SHA256
  `f9afdaefd9718c68d1c2290e2ba215964a40523b8cc1ed17460c8a87022bb809`.
  All five current binaries/manifests independently verified at 8.0.7 before changes.
- Existing baseline flaps were traced to dashboard analytics occupying the single writer
  connection for seconds, delaying liveness queries beyond the heartbeat bound. Navigated the
  open Usage page to Upstream during preparation. Provider credentials were healthy in those
  probes; the read-pool fix will also be tested against live dashboard load after cutover.

### First Home canary — rolled back

- Final old-image rehearsal comparison passed, all 52 tables unchanged, integrity `ok`.
- Home switched at **04:27:59Z**, all five old nodes rejoined at **04:28:05Z**. Legacy/v8 config
  routes returned 200; six key scopes, Claude priorities/enabled flags and index preserved.
- All 64 served panel files were byte-identical. Mixed-version acceptance: 18 authorized native
  Claude/Codex/Copilot completions, five expected Copilot denials, all successful requests
  correlated in Home; every Claude completion selected Gmail.
- Concurrent seven-day dashboard overview/aggregate load returned HTTP 500. Following the
  failure gate, Home was restored to the pinned old image at **04:28:58Z**, before investigation.
  No CPA node binary has changed. The separate reader prevented native request failures, but
  its single reader may serialize analytics beyond the existing 10-second management budget.
  This is being verified offline before a bounded reader-pool adjustment and another canary.
- The removed first candidate container's logs were not retained by compose; the failed probe
  did not capture its error body. Future rollback captures private container logs as part of
  the rollback operation. Do not claim this failure's precise cause until the offline probe
  or a captured response establishes it.

### Corrected dashboard gate and reader follow-up

- The original live load harness also omitted the required aggregates `group_by`, a test defect.
  It has been corrected; future probes retain safe HTTP status/error classes and all timings.
- Private actual-database handler rehearsal initially lacked writable container temporary
  storage. Heavy SQLite aggregation spilled to TEMP and returned another error. This harness
  does not match Home's writable temporary storage; rerun with a private tmpfs is required.
- Follow-up `9821c83` bounds the read-only pool at four connections, keeps the writer at one,
  and configures busy timeout per connection through the SQLite URI. Two/four held real reads,
  writer liveness/config updates, read-only enforcement and replacement-connection settings
  pass 20 repetitions. Full suite and CGO compile pass on Ark Docker. Exact historical live
  500 cause remains unverified; the independent actual-copy reproduction is documented separately.

### Final candidate and staged rollout

- PR [#10](https://github.com/chansearrington/CLIProxyAPIHome/pull/10) merged at `64e788e`;
  source `9821c83b2ee26d5016438bc034dacae559932f40`, local review MUST 0 / SHOULD 0 / NICE 0.
  Final image `cpa-home:1.1.0-claude-fleet-9821c83`, immutable ID
  `sha256:dad5e5d84bd9931dfacb1927bb35990f37bbe7487f5252e69f411ddc83b94809`.
- Final-image isolated export and old-image readability/export pass on a disposable backup copy;
  full integrity `ok`, persistent partial index present. Schema/migration code is unchanged from
  the preceding 52-table exact-content rehearsal.
- Actual-copy baseline comparison proves the seven-day overview timeout also occurs on old
  Home (11.0 seconds, eight writer probe failures). Both versions pass a four-panel 24-hour
  workload; old overview 6.43 seconds and writer max 544 ms, new overview 5.70 seconds with
  zero reader waits and writer max 0.36 ms. This long-range analytics capacity limit is not
  claimed fixed; acceptance uses the baseline-supported 24-hour workload with unchanged timeouts.
- Two further safety aborts were probe defects: checking HTTP before the container listener
  starts, and importing Ark's unavailable SSL library for an HTTP-only probe. Both restored the
  old image immediately. Startup now has bounded readiness polling, and the HTTP probe avoids
  the SSL dependency. Native providers passed in the SSL-probe attempt. No node changed in
  either aborted attempt. All subsequent probes retain safe status/error/timing evidence.
- Home's final cutover was **04:41:30Z**. All five old nodes rejoined; config v0/v8, six scopes,
  enabled priorities 3/2/1 and plugin reports pass. The final 64 panel assets match byte-for-byte.
  Four concurrent live panels all return 200: overview 6.51 s, aggregates 1.30 s, realtime 23 ms,
  credential health 19 ms. All 14 concurrent node checks show five healthy nodes/plugins,
  maximum 8.3 ms. The same image's mixed-version native proof passed 18 authorized completions
  and five expected fences, with all Claude completions selecting Gmail.
- MacBook restart **04:42:04Z**, Moxy **04:42:45Z**: both now CPA 8.0.20, hash/size/banner,
  readiness and manifest verified; native Claude/Codex/Copilot and per-key fencing/usage pass.
- Actual default native CLIs after the Mac upgrade: Codex 0.161.0 session
  `01a119d2-9eba-7383-a8ab-f2fb38348005` executed a shell tool and returned an unpredictable
  marker exactly; usage 171608/171609 shows `gpt-6.1-sol`, Mac node, failed=0. Claude Code
  2.1.293 default `claude-opus-5-5` session `a20fecd4-d9d0-4acc-ad04-482f54cee291` completed;
  usage 171607 proves Gmail credential, Mac node and failed=0. No routing/auth overrides used.

### Final fleet acceptance — passed

| Component | Deployed version | Restart / cutover UTC | Result |
|---|---|---|---|
| Home / Ark | 1.1.0 + fleet fixes `9821c83` | 04:41:30 | Config, credentials, keys, panel, routing and dashboard pass |
| MacBook Pro | CPA 8.0.20 `0f96f568` | 04:42:04 | Binary, manifest, readiness, native providers and usage pass |
| Moxy | CPA 8.0.20 `0f96f568` | 04:42:45 | Same checks plus own-key Copilot denial pass |
| Hyper | CPA 8.0.20 `0f96f568` | 04:43:31 | Same checks plus own-key Copilot denial pass |
| Lara | CPA 8.0.20 `0f96f568` | 04:44:12 | Same checks plus own-key Copilot denial pass |
| Chip | CPA 8.0.20 `0f96f568` | 04:44:47 | Same checks plus own-key Copilot denial pass |

- Final independent inventory completed **04:45:45Z**: all five executable SHA256, size,
  version banner, manifest and supervisor readiness match; each old 8.0.7 rollback binary
  independently hashes to the original value. No node rollback was needed.
- Six-key final acceptance combines each node's post-upgrade proof with legacy key #1 on the
  upgraded MacBook: **18 authorized completions and five deliberate Copilot denials**. All 18
  successes are present in Home with failed=0; all six Claude completions selected Gmail.
  Protected scopes, three enabled Claude priorities, membership and plugin reports are preserved.
- Final concurrent dashboard probe: overview **8.18 s**, aggregates **1.12 s**, realtime **20 ms**,
  credential health **15.5 ms**, all HTTP 200. All **18** node-health samples report five healthy
  nodes and successful required plugin reports, maximum **12.1 ms**. No timeout increase.
- Fresh native Codex startup has **no incompatible-feature/daemon restart warning**. Saved CPA
  provider, local URL, Responses wire API and secure auth helper are unchanged by this rollout.
- Nonblocking source findings remain tracked in fork issues #7/#8/#9; optional weighted/session
  affinity features remain unenabled. Longer-range historical analytics needs separate query
  optimization; do not describe this upgrade as fixing every dashboard timeout.
- Final proof files are private under `/tmp/cpa-fleet-upgrade-20261008`; they contain only
  allowlisted status/identity metadata, except native CLI outputs kept in protected directories.
  Verified production backup is retained on Ark. The standalone HTML guide is committed alongside
  this record and clearly distinguishes available optional capabilities from deployed behavior.

- Post-verification cleanup removed disposable database copies and credential-bearing rehearsal
  exports; retained the verified production backup and secret-free fingerprints. All four fleet
  locks were independently confirmed **UNLOCKED** after orderly renewal shutdown.
