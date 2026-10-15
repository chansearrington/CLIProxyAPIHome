# Fleet models and automatic CPA node currency

Status: model/default/node update complete; laptop maintenance ownership is superseded by
[Home-native maintenance](ws-0031-home-native-fleet.md). The three laptop schedulers are retired.
The following records the initial deployment and its verification.

The user reported missing Opus 5.5, Sol 6.1 and other recent models and requested updating the
fleet/nodes with more automation. The user confirmed automatic model-menu sync and canary-gated patch releases, with review for
major upgrades. The user then confirmed promoting Moxy/Chip to Sol 6.1 and Lara/Hyper to Opus 5.5,
including matching fallback references.

## Finding

Home and the nodes already exposed `claude-opus-5-5` and `gpt-6.1-sol`. Each OpenClaw 2026.9.4
agent retained an older static catalog and defaults. Opus 5.5 was in the provider catalog but
missing reasoning metadata; Sol 6.1 and four other IDs were absent. The daily release check was
running but merged model-list changes into a pending Ship card, leaving availability behind.

The initial attempt with the older picker helper was restored automatically: its health check
went green before the catalog reloaded, and the subsequent immediate model check failed. The
new helper validates the model views themselves with bounded retries. The successful update
preserves defaults, fallbacks, aliases, session choices, plugins, channels and credentials.

## Live result

All four OpenClaw gateways expose all **55** current shared CPA IDs in `models.list` views
`provider-config` and `configured`. Added IDs:

- `gpt-6.1-sol`.
- `claude-haiku-5-5`.
- `claude-opus-5-5-high`.
- `claude-sonnet-5-5-high`.
- `gemini-4-argon-high`.

Missing reasoning/input/context/output metadata is filled from Home's operational definitions;
explicit existing metadata wins. Four unlisted older Gemini rows remain to protect existing
references. Private `copilot/` IDs and the readiness canary are excluded from shared menus.
Home's public catalog includes private plugin names even with shared-only keys; access policies
are still enforced by dispatch and were not expanded.

## CPA 8.0.30

Latest official release checked October 15: CPA 8.0.30, Home 1.1.1. Home was already current and
was not rebuilt, restarted or reconfigured. There is no Home database write, so no new database
backup is needed. The 8.0.29→8.0.30 comparison contains no Home wire or plugin ABI change.

- Verified archive SHA256: `39492d8dbb6d8cb4e6775629f190b4860ee35a559b2095df0068ecc92de4b680`.
- Executable SHA256: `87becc2b0acc395d45830a4d9846cdfebf46ddd77fbae916a2cd57417b930c37`.
- Size: 64,543,794 bytes; banner commit `5be2cdab`.
- All four canonical locks held and independently renewed during live changes.
- MacBook canary, then Moxy, Hyper, Lara, Chip; restart in place only.
- Actual native **Opus 5.5 and Sol 6.1** completions with requested markers passed on every node.
  Both models also passed with **all six original client keys** after rollout.
- All five Home memberships remain healthy with required Copilot plugin reports `reported_ok`.
- Retained verified rollback on each node: `cli-proxy-api.8.0.29.bak`.
- OpenClaw model before/candidate backups are private under each agent's
  `~/.openclaw/backups/cpa-catalog-*/`.

Existing Claude/Codex priorities and scopes, private Copilot/Microsoft fences, native logins and
CodexBar access-token ownership are preserved. OpenClaw itself remains 2026.9.4; moving its core
and plugins to the separate 2026.10.3 release uses its migration/compatibility procedure.

## Installed automation

User LaunchAgent `ai.openclaw.fleet-currency` runs at login and every **hour** on Chanse's MacBook.
It needs the Mac awake/logged in, existing Ark/agent SSH access, and healthy nodes/gateways.

- Model additions refresh independently of release approval cards.
- Stable CPA patch releases within the current major/minor line install automatically, using
  checksum verification, all four locks, a MacBook canary, serial rollout, native completion
  acceptance and rollback. Larger release-line changes remain in the review lane.
- Failed releases latch to prevent repeated unattended installation. Explicit Hold decisions
  are honored. Upgrade operations have detached target workers and durable receipts, so a
  dropped SSH response can be reconciled before rollback or retry.
- A successful full rollout writes a verified accepted-core receipt, allowing enrollment/repair
  to follow newer accepted patches. The reviewed source baseline is now 8.0.30, after Hyper's
  installed binary was verified.
- The daily release check retains its version-review lane through a wrapper that disables its
  older picker-card producer. The completed 8.0.30 card was resolved against this user request;
  no Ship tap was invented.

Source and detailed runbook are in agent-os:
`control-plane/cpa-home/{fleet-currency.py,catalog-sync.py,install-fleet-currency.py,core-pin.mjs,release-review.mjs}`
and `docs/runbooks/cpa-fleet-currency.md`.

Content-hashed private installation: `~/.openclaw/services/fleet-currency/current/`.
Status/history/progress: `~/.openclaw/state/fleet-currency/`.
The installed job completed with exit 0 and reported all four catalogs and the node release
**current**, without rewriting configs on the subsequent run. Independent gateway checks and
install manifests provide acceptance; a menu screenshot is not claimed.

Pause only the updater with:

```sh
launchctl bootout gui/$(id -u)/ai.openclaw.fleet-currency
```

Reinstall without `--patches` for catalog-only automation. The older Easy/8317 pilot stays paused.

Verification includes 21 focused Python tests, 5 core-pin/review-wrapper tests, enrollment suites,
148 rollout/closure/pin checks, and the repository's matched CPA Home offline workflow. Workflow
lint also passed with checksum-verified actionlint 1.7.12. `ci-local` reports its actionlint network
installer as skipped; its CPA suites pass, and the skipped linter was executed independently.
Private operational evidence: `/tmp/cpa-fleet-models/` and the installed status/history above.

Source commit `7eba3d21` is retained on the local agent-os `main` and feature branch; no remote push was performed. All four locks are independently confirmed UNLOCKED.

## Confirmed default-model promotion

Following the user's explicit reply, Moxy and Chip now default to `cpa-gui/gpt-6.1-sol`;
Lara and Hyper default to `cpa-gui/claude-opus-5-5`. Corresponding Sol and Opus fallback
references were promoted, including Chip's separate main-agent override and matching
librarian fallbacks. Librarian primary models, unrelated fallback families, and existing
conversation pins were preserved.

Each agent has a private before/candidate backup under `model-default-promotion-*`.
OpenClaw schema validation passed, followed by the gateway's actual `config.get` snapshot:
valid source/runtime configs, the desired default and main override, and identical source/applied
revision hashes. Gateway health passed. No node or gateway restart was required.

All six client keys subsequently completed a real request using the newly selected primary family. Each mini passed through its own loopback node: Moxy/Chip Sol 6.1 and Lara/Hyper Opus 5.5; system/MacBook keys also passed Sol 6.1. Evidence: `/tmp/cpa-fleet-models/default-primary-proofs.json`. All four locks released after these checks.

A post-promotion background check stopped during Lara verification with a generic error; independent Lara health/default checks passed. The catalog worker now bounds command startup and its retry window, and its structured failure reason reaches controller status instead of being discarded by SSH exit handling. No model selections were lost.

Final scheduled retry at 21:14 UTC completed with exit 0: all four catalogs and the CPA release current, and all four promoted defaults independently preserved. Installed source hashes match committed agent-os source through `b9f81d40`.
