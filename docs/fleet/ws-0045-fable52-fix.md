# Fable 5.2 execution and automatic Claude Code picker sync

Status: complete — Fable 5.2 serves on all five nodes/six keys; five Claude pickers and native 1M request verified; 20 Ark tests pass; Home-scheduled MacBook catalog task succeeded; maintenance restored and locks released.

The user requested a plan, review and complete fix for Fable 5.2 native execution and the
missing automatic Claude Code picker entries across the five-node fleet.

## Plan

1. Confirm the installed CPA executor’s supported software-profile override controls both
   request headers and the signed billing version. Capture the actual installed Claude Code
   software tuple against a loopback fake-key server, isolated from user settings/credentials.
   Prefer a DB-backed Home configuration change over node rebuilds or fleet-wide upgrades.
2. Extend the existing Home-owned node-local catalog adapter to reconcile Claude Code’s
   `modelPicker.options` as well as OpenClaw. Query each node’s authenticated CPA model listing;
   select current native Claude model families only, excluding encoded aliases/private models.
   Preserve defaults, existing custom options, permissions, logins and active sessions. Persist
   ownership of generated options, retain user edits, and never edit first-party caches.
3. Make Claude-only nodes eligible for Home catalog tasks. Keep policy/scheduling and mTLS
   identity ownership in Home; do not add a laptop coordinator or another scheduler.
4. Add focused deterministic tests for discovery, Claude-only operation, version promotion,
   custom-option/default preservation, concurrent edits, endpoint validation and rollback.
   Run tests on Ark in the existing Docker environment. Review the implementation and fix findings.
5. Pause Home policy with no active job, acquire/check/renew all four canonical locks, and
   save targeted settings/configuration backups. Apply the measured Claude baseline through
   Home’s API and test a real Fable 5.2 canary; roll back that leaf on failure.
6. Verify Fable 5.2 through every node and all six keys, preserve native Sonnet/Opus/Codex
   access, and install the reviewed catalog-worker bundle. Reconcile picker entries on all
   five machines and verify the actual Claude Code picker on the MacBook.
7. Restore the exact maintenance policy, verify a subsequent Home-issued catalog cycle,
   retain only the latest two backup versions, release locks and record evidence/limitations.

## Plan review

- The earlier audit’s hardcoded-version finding was incomplete: production cloaking calls
  `helps.DefaultClaudeVersion(cfg)`, which extracts the configured user agent. The supported
  header-default path updates the signed version too. A binary rollout is unnecessary if the
  canary proves the configuration path, avoiding the unrelated failed patch-rollout lane.
- Listing a model is insufficient acceptance: native completions and Home usage attribution
  are required. Confirm the model stays Fable 5.2 rather than silently falling back.
- Home’s model metadata is global; generated picker entries must also exist in the local
  key-scoped listing. Exclude encoded Claude aliases for other providers. Only loopback CPA
  endpoints are trusted for local token use.
- Generated picker entries must not overwrite user-owned labels/settings. Track exact prior
  generated rows; preserve modified rows and unrelated GPT/Grok/Gemini entries. Never promote
  session/default models or patch `.claude.json` cached first-party choices.
- Support a MacBook with Claude Code and no OpenClaw provider. A successful Claude-only
  receipt must not attempt OpenClaw CLI/runtime validation.
- Configuration-only backups are sufficient. No database snapshot, token export or account
  reprioritization is part of this change. Restoring old OAuth state is prohibited.
- Failed native canary: restore the changed Home configuration leaf before investigating.
  Failed picker publication: scoped rollback with optimistic concurrency; retain unrelated edits.

## Implementation review and compatibility canary

- Captured actual native clients against isolated fake-key loopback servers: mini Claude Code
  2.1.300 uses SDK 0.128.0/runtime v26.3.0; MacBook 2.1.322 uses SDK 0.128.0/runtime v26.12.1.
  No native credential or user settings were used in the measurements.
- Home's supported header-default override controls `DefaultClaudeVersion` used in production
  cloaking/signing. Applied the measured 2.1.300 software baseline through a narrow config PATCH.
  The API accepted the historical OAuth-provider input path and canonicalized it to
  `/config/upstream/claude/header-defaults`. Readback and a full comparison excluding only that
  leaf verify that all other configuration, accounts/priorities, keys, bindings and model scopes
  are unchanged. The initial alias-path readback assertion was corrected; no native check failed.
- Real Fable 5.2 canary now returns HTTP 200, exact model `claude-fable-5-2`, reply `FABLE52_OK`.
- Implementation review tightened ownership validation and added two-version OpenClaw backup
  retention alongside the new Claude-picker backups. Focused tests cover future version promotion,
  user-owned rows, edited generated rows, concurrent writes, scoped rollback preserving a newer
  default, Claude-only operation without OpenClaw, wrong endpoints/symlinks and idempotency.
  All 18 existing/new worker tests pass on Ark inside `golang:1.26-bookworm` after these changes.


## Deployed implementation and acceptance

All five machines now run content-addressed catalog-worker bundle `80bc72a639456aed`.
The installed SHA256 values match the reviewed workspace source:

- `worker.py`: `70c746cbd33683cc1022f1e623b5f5647378d6db8797bb71629d43d4ece41a29`.
- `catalog_sync.py`: `ff4324717507a5e0d6722872c710d5b9c40e49b6dff7ee642b44240088a14597`.

All five Claude Code pickers include `claude-fable-5-2[1m]`, labeled **Fable 5.2**, ahead of
preexisting custom provider entries. Existing defaults, other settings, custom options and
worker settings match the targeted backups. A second reconcile on every node returns current
without another settings write. The four OpenClaw catalogs remain current; no defaults were
promoted. The worker now advertises the MacBook as a catalog target without requiring OpenClaw.

The actual MacBook Claude Code 2.1.322 `/model` screen was observed showing **Fable 5.2**.
An isolated native Claude Code request with tools/hooks disabled returned `NATIVE_FABLE52_OK`,
canonical model `claude-fable-5-2`, context window **1,000,000** and maximum output **128,000**.
It used the real CPA route and did not persist a session or alter the user's default model.

All ten Fable fleet/key checks and the Sonnet/Opus/Codex controls returned HTTP 200 and the exact
expected replies. Home usage independently confirms model, provider, credential and source node:

| Node/key/model | Usage ID |
| --- | --- |
| macbook / 1 / claude-fable-5-2 | 3049665 |
| macbook / 2 / claude-fable-5-2 | 3049667 |
| macbook / 3 / claude-fable-5-2 | 3049666 |
| macbook / 4 / claude-fable-5-2 | 3049668 |
| macbook / 5 / claude-fable-5-2 | 3049669 |
| macbook / 6 / claude-fable-5-2 | 3049670 |
| moxy / 4 / claude-fable-5-2 | 3049671 |
| hyper / 2 / claude-fable-5-2 | 3049672 |
| lara / 5 / claude-fable-5-2 | 3049673 |
| chip / 3 / claude-fable-5-2 | 3049675 |
| macbook / 6 / claude-sonnet-5-5 | 3049676 |
| macbook / 6 / claude-opus-5-5 | 3049674 |
| macbook / 6 / gpt-6.1-sol | 3049677 |

All five Home memberships are healthy and Copilot reports `reported_ok`. No Home image, CPA
node binary, account priority, credential authorization, key scope or model default was changed.
The compatibility repair is the DB-backed header-default setting; no node rebuild was needed.

## Review findings resolved during rollout

- A Hyper worker bootstrap attempt failed. Its original bundle and LaunchAgent were restored
  and verified loaded before retrying successfully. Final bundle revisions were published by
  atomic pointer replacement, preserving the healthy polling LaunchAgents and settings.
- The manual metadata projection initially supplied zero limits for unspecified model fields.
  OpenClaw rejected the staged candidate before publication; the working config was unchanged.
  Corrected the projection to omit nonpositive limits, matching Home's production task metadata.
- Independent target ordering was tightened: Claude picker reconciliation runs before OpenClaw,
  so an OpenClaw validation failure cannot block it, while the overall task still reports failure
  until every configured target verifies. Regression coverage proves both properties.
- A final review adds a second endpoint check against the exact settings bytes being changed,
  preventing a user endpoint change during discovery from being overwritten. The regression
  retains the user's changed settings. All **20** existing/new Python tests pass on Ark inside
  `golang:1.26-bookworm` after all production code changes. `git diff --check` passes.

## Backups, rollback and follow-up

Targeted Home backup:
`/mnt/user/appdata/cpa-home/data/backups/home-config-20261110T161433Z.json`, **12,175 bytes**,
mode 0600. Only this and the November 9 targeted config version remain; database snapshots
were neither created nor restored. Each machine retains its original worker bundle
`08da935389e41a3f` and final accepted bundle; intermediate candidates were removed. Per-target
catalog backups retain at most two versions. Original worker settings/pointer/LaunchAgent and
Claude settings are private under `~/.openclaw/state/cpa-home-fleet-worker/ws0045-before/`.

Scoped compatibility rollback restores/removes only `/config/upstream/claude/header-defaults`.
Picker rollback removes only exactly owned generated options and retains unrelated newer edits;
switching the worker pointer back to its saved original bundle reverts the adapter. Preserve
current tokens, account history and client/default selections; never restore a whole database.

Private sanitized evidence: `/private/tmp/cpa-fable52-fix-20261110/` locally and
`/mnt/user/appdata/cpa-home-build/ws0045-fable52/` on Ark. The existing failed CPA patch-rollout
lane remains a separate issue; this fix does not approve, retry or relabel that release.


## Scheduled acceptance and close-out

The exact original Home maintenance policy was restored and all four operation-owned locks
were released. At **2026-11-10 16:32:56 UTC**, Home issued catalog task
`cc3787e9-10ce-4668-a3a4-992f6f499103` to the MacBook. Its existing LaunchAgent executed the
new adapter through the normal mTLS Home protocol and Home recorded **success**, including
catalog hash `142c63e0fc07ecf9d21c85e170bbc1ac51cde91f10034f5bcb1a1ae0831b3f3d`.
This verifies the newly supported Claude-only laptop target through Home's scheduler,
without a new laptop coordinator or manual policy-interval change. All five node memberships
remain healthy. Final source is retained in this workspace and the matching private installed
bundles; no PR/merge or new Home/node image deployment is claimed.

Reopen `/model` and choose **Fable 5.2**, or use `/model claude-fable-5-2[1m]` explicitly.
A long-running Claude session that keeps its old picker cache may need reopening; current
settings and a fresh native UI are verified. Defaults remain the user's prior selections.
