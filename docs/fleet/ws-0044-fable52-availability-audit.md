# Fable 5.2 catalog, native execution and Claude Code picker audit

Status: audit complete — Home distributes Fable 5.2 to all five nodes and all four OpenClaw runtime menus; native completions fail on all five nodes with the proxy’s Claude Code 2.1.280 compatibility identity. Claude Code’s short picker is a separate unsynchronized surface. No live configuration or binaries changed.

Resolved November 10 by [the reviewed compatibility and picker fix](ws-0045-fable52-fix.md).
The findings below describe the pre-fix audit.

Checked November 9, 2026 Central (November 10 03:05–03:09 UTC) after the user showed the
MacBook Claude Code picker advertising Fable 5.1 instead of Fable 5.2.

## Distribution is working

- Home `/models?scope=available` and `/routing/model-definitions/claude` contain
  `claude-fable-5-2`, provider `claude`, 1,000,000 context tokens and 128,000 maximum output.
- Authenticated `/v1/models` on MacBook, Moxy, Hyper, Lara and Chip all list that exact ID.
  All five nodes run CPA 8.0.58, with matching Home-reported executable hashes.
- All four OpenClaw source catalogs, allowed-model maps and running gateway `models.list`
  views (`provider-config` and `configured`) contain Fable 5.2. Runtime rows report available.
- Home’s enabled hourly catalog tasks succeeded on all four minis around 02:27–02:36 UTC.
  The MacBook has no configured OpenClaw catalog target, so its worker correctly reports
  catalog false; its CPA node still receives the model from Home.

## Native execution is blocked

Sent one minimal native `/v1/messages` request for `claude-fable-5-2` through each node,
using each machine’s original client key. Every request returned HTTP **400**:

> Claude Code 2.1.280 does not support this model; version 2.1.290 or newer is required.
> Run 'claude update', or update the Claude desktop app, then try again.

Repeated the probes with the actual installed clients’ version headers: MacBook **2.1.322**,
minis **2.1.300**. All five again returned the same 2.1.280 upstream rejection. No completion
succeeded. A model-list entry or OpenClaw available flag is insufficient proof of native execution.

The known fleet runbook documents that CPA’s Claude cloaking/signing path can present its own
fixed Claude Code version independently of the installed client. Official v8.0.58 source still
passes the `2.1.280` fallback in `claude_executor_cloaking.go`, lines 699 and 704. Official
v8.0.61 contains the same fallback. Merely updating the client is insufficient; do not claim a
routine node patch update resolves this without a successful native canary.

Home has no explicit Claude header-default override in its current configuration. Determining
and validating the appropriate CPA compatibility fix remains separate implementation work.

## Claude Code picker is separate

The MacBook’s `~/.claude.json` has an `additionalModelOptionsCache` entry selecting
`claude-fable-5-1[1m]`, labeled Fable 5.1. None of the five machines’ explicit
`modelPicker.options` includes Fable 5.2. MacBook’s picker extensions are static GPT/Grok/Gemini
entries; the minis have similar static extensions. No picker entries or default selections
were changed during this audit.

The Home fleet catalog adapter (`tools/fleet-worker/catalog_sync.py`) reconciles OpenClaw
provider/allowed-model catalogs only. It does not reconcile Claude Code’s model picker or its
first-party cached options. Automatic Claude Code picker reconciliation is therefore an
unimplemented part of the broader model-availability workflow, rather than a failed Home
catalog push.

## Scope and evidence

No Home/node upgrades, restarts, maintenance-policy edits, routing/priorities, account
credentials, defaults, configuration files or client caches changed. Tests issued small native
requests and read-only model/runtime queries. Home separately reports a failed 8.0.61 binary
rollout; catalog tasks continue succeeding. That binary-rollout issue was not modified here.

Private sanitized evidence: `/private/tmp/cpa-fable52-20261109/`, including `home-audit.json`,
`node-audit.json`, `home-definitions.json`, `live-proof.json` and `versioned-proof.json`.
Client keys stayed in process memory and were excluded from saved reports.
