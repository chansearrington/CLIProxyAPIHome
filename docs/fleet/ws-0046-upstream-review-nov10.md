# Upstream CPA and Home review — November 10

Status: assessment complete — recommend stable CPA 8.0.64 and a fleet-preserving Home 1.1.9 integration; no upgrade or runtime configuration change performed. Independent recovery access and repair of the failed deployment path are prerequisites.

The user asked whether upstream changes should be brought in, then explicitly reminded us that
the last Home upgrade severed agent access and required restoring direct OpenAI access to repair
it. This assessment must not be treated as authorization to deploy new releases.

## Verified deployed and upstream versions

| Component | Deployed | Latest stable | Recommendation |
| --- | --- | --- | --- |
| Home fleet image | 1.1.8, commit `87210da`, image `sha256:75ede4deb4e4d4cbe2f0a33b456dfbf57c55792ba9018d4ea04bf1df40fd6283` | 1.1.9, November 7 | Worth integrating after migration/rollback rehearsal; retain fleet patches and custom Reconnect panel |
| All five CPA nodes | 8.0.58, binary `7c0b6f98bef85319d371a3f110c9e904de7896e6259cf1b492d8b62ef7263fa2` | 8.0.64, November 10 | Worth updating after correcting the canary/rollback failures |

Home has zero restarts since its accepted November 6 recovery. Home upstream main exactly
matches v1.1.9; CPA main has only one additional documentation/sponsorship-removal commit beyond
v8.0.64. There is no reviewed runtime benefit to choosing main instead of these stable tags.
The Home checkout already contains the three v1.1.9 release-workflow/test commits merged on
November 7, but the deployed image predates them and the new runtime fixes are still missing.

## Changes relevant to this fleet

### CPA 8.0.59–8.0.64

- **8.0.63:** official Claude software baseline advances from **2.1.280 to 2.1.321**, SDK
  0.112.1 to 0.128.0 and runtime v26.3.0 to v26.12.1. Fable 5.2 joins the embedded registry;
  preserved-thinking/per-turn timing gates also cover it. This supersedes the old built-in
  baseline behind our Fable incident, beyond merely adding a model-list entry.
- **8.0.62/8.0.64:** preserve native Claude user-agent extensions and background `cli-bg`
  requests, avoiding misclassification and unnecessary cloaking of real client traffic.
- **8.0.64:** only an actual persisted heartbeat PONG renews Home subscription liveness;
  other traffic cannot mask a dead membership. Relevant to earlier fleet reconnect incidents.
- **8.0.59–8.0.61:** protect Codex refreshes from replay after uncertain outcomes, preserve
  rotating grants/renewal windows, and defer renewals during macOS maintenance wakes.
  Home remains the refresh owner for our shared credentials, so the matching Home fix matters.
- Preserve prompt-cache prefixes around deferred ToolSearch/mid-session Responses system
  items, improve tool-output translation and Responses websocket/compaction behavior.
- **8.0.62** reverts the 8.0.60 post-stream draining/connection-reuse change. Prefer the final
  8.0.64 target rather than stopping at that intermediate release.

The 8.0.58→8.0.64 comparison contains 78 commits/286 changed files, including tests. Treat it
as a meaningful compatibility update despite all versions sharing the 8.0 patch-release line.

### Home 1.1.9

- Persistent **Codex refresh journal** claims rotating grants before submitting them and
  preserves uncertain outcomes/receipts, reducing the chance that a timeout or restart replays
  a consumed grant and loses account authorization.
- Deduplicate retried CPA usage ingestion, including billing/quota side effects, rather than
  appending and charging the same request again.
- Request-scoped errors no longer incorrectly make a healthy credential unavailable.
- Unpriced models can still be dispatched; retain this installation's explicit billing policy
  instead of accidentally changing allow-unpriced behavior during an update.

This release changes the database/snapshot format **16→17**, adding `codex_refresh_journal`
and migrating refresh-outcome evidence. It does not change go.mod/the official Home SDK pin
from the currently integrated CPA 8.0.53. Do not bump the SDK independently just to make its
version match the node binary.

## Deployment blocker discovered

Home's rollout is **failed** with `binary_operation_failed`, targeting 8.0.64. Moxy's retained
manifest records repeated failed attempts at **8.0.61, 8.0.63 and 8.0.64**, each with
`outcome: rollback_not_ready`, `attempt_reason: not_ready_timeout`. Its current and saved
8.0.58 binaries both match the accepted hash. A matching binary alone is not proof that the
rollback completed cleanly at the time of failure.

The precise readiness failure is not established by this read-only review. Do not clear the
latch or retry another version without diagnosing it and proving both forward and reverse
readiness checks. The updater currently latches per failed version, so a different newly
published patch can otherwise cause another automatic attempt. Repairing this path, or
explicitly holding automatic patch deployment during repair, is the first operational action
to plan before a new rollout. No policy was changed in this assessment.

## Requirements from the prior outage

1. **Independent recovery access first.** Before any manually initiated disruptive cutover,
   verify a working direct OpenAI/operator route that does not depend on CPA Home, any node
   being updated, or Home-issued management auth. Keep the control session and recovery
   commands on that route until acceptance. Do not borrow Home-owned refresh grants for
   local fallback authentication. This route has not been exercised by this read-only review;
   it must be positively tested before deployment, not restored after an outage.
2. Use one independently running SSH/Docker/launchd recovery controller with exact PID/status
   markers, explicit rollback rules and no model-call dependency. A dropped assistant session
   must not strand a swap. Stop the existing controller before manual intervention; never run
   competing cutover/rollback loops.
3. Rehearse using production's actual read-only root filesystem, mount layout, tmpfs limits
   and SQLite temporary-directory environment. The running Home still has `/tmp` limited to
   64 MB, but **both TMPDIR and SQLITE_TMPDIR correctly point to the writable data volume**:
   `/CLIProxyAPIHome/data/sqlite-tmp`. Preserve those values in candidate AND rollback Compose
   configurations. They were independently verified live during this review.
4. Measure full startup/API/native readiness. The previous successful startup took nearly
   four minutes; container running state and a short fixed deadline are insufficient.
5. For Home's real schema migration, take a verified full snapshot and rehearse old/new
   images offline on a copy. Test refresh-journal/fence preservation as well as table and
   API compatibility: an older image that can open the file may ignore newer refresh safety
   evidence. Never restore stale rotating credentials over newer live authorizations.
   Routine configuration-only work still uses targeted backups. Retain only two versions.
6. Freeze/review current working source before integrating upstream. Preserve the custom
   panel/Reconnect, quota recovery, writer/read-pool protections, channel/key fences, current
   account priorities and the deployed Claude picker worker `80bc72a639456aed`.
7. Retain our working 2.1.300 profile override while nodes are mixed-version. After all nodes
   are updated, test the official 2.1.321 baseline with both actual client versions (2.1.300
   minis and 2.1.322 MacBook) before retiring the temporary override. Blindly keeping it would
   override the new defaults; removing it before old nodes are upgraded would reintroduce
   the Fable failure.
8. Use an always-on node canary with independently verified rollback, then serial rollout.
   Keep the workstation/recovery route usable. Treat Home's all-fleet cutover as a separate
   operation. Accept exact Fable 5.2 plus native Claude/Codex/tool/compaction flows, all six key
   scopes, five memberships/plugins, original settings/history/billing and scheduled picker sync.

## Sources and evidence

- https://github.com/router-for-me/CLIProxyAPI/releases/tag/v8.0.64
- https://github.com/router-for-me/CLIProxyAPI/releases/tag/v8.0.63
- https://github.com/router-for-me/CLIProxyAPI/compare/v8.0.58...v8.0.64
- https://github.com/router-for-me/CLIProxyAPIHome/releases/tag/v1.1.9
- https://github.com/router-for-me/CLIProxyAPIHome/compare/v1.1.8...v1.1.9
- Previous outage: [Home 1.1.8 recovery](ws-0041-home-118-recovery.md).
- Current Fable/worker acceptance: [reviewed implementation](ws-0045-fable52-fix.md).

Private sanitized GitHub comparison/runtime evidence:
`/private/tmp/cpa-upstream-review-20261110/`. No builds, migrations, upgrade attempts, credential
writes, restarts or live policy changes were performed for this assessment.
