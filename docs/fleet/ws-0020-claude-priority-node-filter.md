# Claude priority and node availability correction

Status: deployed and verified on all five nodes; enrollment pins merged, 2026-10-13 16:41 UTC.

The reported symptom was Microsoft Claude appearing in CodexBar despite priority 1 being the
emergency backup. This investigation distinguishes historical incorrect fallback from genuine
quota exhaustion later in the day.

## Findings

- **Higher numbers run first.** Home's `bestPriorityBucket` chooses the maximum integer and its
  scheduler sorts priority buckets descending. Database attribute and metadata priorities agree
  for all eight Claude credentials: Gmail 10, HypeSports 9, 82 8, Chip 7, Developer 6, Moxy 5,
  Lara 4, Microsoft 1. Microsoft remains Chanse-only; the seven regular accounts remain shared.
- CodexBar follows the latest successful Claude request for the MacBook node and client key 6.
  It can show different accounts as requests for different models finish. It does not choose
  CPA credentials. Its selection and status file matched actual attributed usage.
- Chip's Opus requests stopped after a burst of network read timeouts around 04:57 UTC.
  Its recorded Opus state was HTTP 499, `context canceled`, `unavailable=true`, no quota
  exhaustion and zero retry/recovery timestamps. Home explicitly considers that state eligible.
  Sonnet's state was active and Sonnet continued using Chip.
- At 06:28:56 UTC a new native Opus request on shared client key 3 returned 503
  `auth_not_found` without an upstream API request. Key 6 succeeded using Microsoft. Both keys'
  new Sonnet requests succeeded using Chip. Provider usage at 06:20 still showed Chip with
  remaining session and weekly allowance; there was no separate Opus quota window or intervening
  Chip Opus 429 explaining this exclusion.
- **CPA node 8.0.20 filters an already-selected Home credential again.** Its
  `preparedExecutionModelsWithAlias` calls `filterExecutionModels`; the node's
  `availabilityBlock` treats unavailable state without recovery timestamps as indefinitely
  blocked. `executeHomeOnce` ends the selection with `no_execution_models`, excludes that
  credential and dispatches again. This produces Microsoft fallback for Chanse or no available
  credential for a shared user. Streaming has the same defect.
- Upstream [PR6490](https://github.com/router-for-me/CLIProxyAPI/pull/6490), commit
  `4a8ecf5ceb3926acceeab9d4f59f9923a1116d9c`, removes that second filter for Home dispatch.
  Local selections retain local filtering. The fix first shipped in 8.0.23 and is included in
  the tested official 8.0.27 release.

Historical INFO logs did not retain the first/second RPOP payloads. The causal verification is
the captured credential state, the matching execution source, the native failure without an
upstream attempt, and a deterministic old-fails/new-passes regression, rather than a claim of
retrospective packet-level tracing.

## Verification before rollout

All Go checks ran on the Ark in `golang:1.26-bookworm`, with array-backed caches and a 3 GiB
exec-enabled temporary filesystem. The Docker disk had 99,693,318,144 bytes available.

- An isolated Home replay of the projected pre-exhaustion credential states selected Chip at
  priority 7 for both Sonnet and Opus. Expired transient failure recovery also passed.
- The upstream cancellation regression copied into CPA 8.0.20 failed execute, count-tokens,
  streaming and WebSocket paths with `auth_not_found`.
- On CPA 8.0.27 all 24 Home availability cases passed. The auth, execution registry, Home
  transport and plugin-host package suites passed as well.
- Official plugin-enabled darwin arm64 archive SHA256
  `d9bf58db6e07d0f587a248abc4a1099178fbec1d659d874c961d8caf3495a09e` matched both the GitHub
  release digest and `checksums.txt`. Extracted executable: 64,394,914 bytes, SHA256
  `23427a5c7907005377e74161db94f7effda67d857ffc05708365a8e20d37f6f0`, version 8.0.27,
  commit `bf934162`, built 2026-10-12T16:38:18Z.

## Deployment and acceptance

All four canonical fleet locks are acquired and renewed throughout the one-node-at-a-time
rollout. The existing tested `upgrade-node-core.sh` verifies the old and staged hashes, retains
the verified 8.0.20 binary, restarts in place, checks readiness and the new startup banner, and
updates each install manifest. Failed acceptance rolls changed nodes back before investigation.

Home's image, database, priorities, keys, groups, quotas and OAuth refresh ownership are unchanged.
Future enrollment pins were merged through [agent-os PR784](https://github.com/chansearrington/agent-os/pull/784),
commit `5d3d91da440f8a70690f357c32f177f5ee700dce`. Exact-head local review found zero MUST
issues. Its one SHOULD issue required Hyper's source binary to be upgraded before merging;
Hyper's installed checksum was verified and that issue was resolved before merge. Both
enrollment test files passed all 11 tests.

At the daytime rollout all seven shared Claude accounts were genuinely quota-blocked. Native
Claude acceptance therefore requires Chanse's valid Microsoft fallback to succeed and shared
keys to remain quota-blocked; it does not bypass a legitimate limit to manufacture a success.
Native Codex is checked on all nodes and every client key; Copilot's private scope is preserved.

The first MacBook/Moxy attempt was rolled back when the quota assertion expected HTTP503.
The old binary independently returned the same correct HTTP429 `model_cooldown`; the
assertion was corrected, and the rollout repeated successfully. No quota was reset or bypassed.

| Node | Successful restart, UTC | Acceptance |
|---|---|---|
| MacBook | 16:33:00 | Exact binary/manifest, native Codex and Claude/Copilot, membership/plugins |
| Moxy | 16:34:32 | Exact binary/manifest, native Codex, legitimate Claude429 and private Copilot denial |
| Lara | 16:36:01 | Same checks |
| Chip | 16:37:28 | Same checks |
| Hyper | 16:38:57 | Same checks; enrollment executable source verified |

Final acceptance at 16:40:58 UTC additionally checked both Claude Sonnet/Opus and Copilot with
all six keys through the MacBook: key6 returned HTTP200, the other five returned Claude429
`model_cooldown` and Copilot503 `auth_not_found`. Every key also passed a native Codex Responses
completion. All five nodes remained healthy with `reported_ok` plugin state; configuration
hashes, priorities, scopes and bindings were preserved. Home's independently read immutable
image remains `sha256:a0622434ac0107c827f756a16a4bc0824c69673ba13e0057075d5a1589d20fc0`.
CodexBar's fresh selection matched attributed MacBook/key6 Microsoft usage; all eight token
accounts and automatic follow/sync settings remain intact.

The next recorded shared Claude recovery is account 82's session reset at 19:20 UTC / 2:20 PM
Central on October 13. Its latest weekly reading had 59% remaining. HypeSports and Chip weekly
windows reset at 21:00 UTC / 4:00 PM Central; Gmail resets at 00:59:59 UTC on October 14 /
8:00 PM Central on October 13. These limits are shared across Claude Code models.

Private evidence is under `/private/tmp/ws0020-*` on the Mac and
`/mnt/user/appdata/cpa-home-build/ws0020/` on the Ark. The previous verified executable remains
`cli-proxy-api.8.0.20.bak` on each upgraded node.
