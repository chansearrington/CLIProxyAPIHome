# Fleet maintenance belongs to Home

Status: complete — Home owns maintenance; all four minis completed Home-issued tasks with the laptop coordinators absent; inference/scopes/panel preserved; workers and policy verified.

The user rejected the MacBook-owned coordinator and requested this become part of CPA Home.
The previous deployment reused an older laptop release watcher. Home already refreshes its model
registry on startup/every three hours and distributes CPA configuration/plugins; the missing
pieces were consumer model-menu reconciliation and guarded core binary upgrades.

## Architecture

Home now owns fleet policy, release checks, candidate review, serial task leases, rollout order,
rollback and history. State is stored in Home's existing snapshot-backed database KV table under
the protected `__home:fleet:` namespace. General RESP KV clients cannot read/write that namespace.
There are no new file-backed runtime settings, database tables or snapshot-format changes.

`GET /v8/management/fleet`, `PUT /fleet/policy` and
`POST /fleet/releases/:version/{approve,hold}` expose the control plane through existing
Management authentication. Model catalogs come from Home's existing registry, not a second
scheduled downloader. Major/minor releases and retrying failed candidates require explicit review.

Node-local workers reuse their existing verified RESP/mTLS certificates for `GET fleet-plan`
and `RPUSH fleet-status`. Home binds identity to the certificate, verifies revocation and task
ownership, and sends only typed catalog/upgrade/rollback operations. Workers receive no Management
secret. Their polling launchd jobs execute their own host's work; no host coordinates the fleet.
The canary is an always-on mini. An optional sleeping laptop does not block required nodes.

One live task is leased across the fleet. Expired tasks wait for owner reconciliation; they are
never silently treated as successful. Workers retain idempotent receipts and renew leases.
Each mini also respects its existing local maintenance lock. Catalog sync preserves explicit
model defaults, fallbacks, aliases and active conversation pins. Binary artifacts are verified
by Home and again locally; node services restart in place, with native probes and reverse rollback.
Genuine quota is not reset or bypassed. A required-node acceptance receipt supports future
agent-os enrollment without a laptop release coordinator.

The initial adapter is darwin/arm64, matching this fleet. Existing CPA 8.0.30 command payloads
are unchanged. The new executor protocol is additive and does not require a CPA core fork.
Certificate reuse was independently tested against the existing live listener with verified TLS
hostname/CA validation and PING before deployment.

## Guarded deployment

All four canonical fleet locks acquired and independently heartbeating. All Go formatting,
focused/full tests and compilation run on Ark in `golang:1.26-bookworm`, with the existing array
caches and 3 GiB executable tmpfs. The private `ws0031/run-go.sh` is the canonical wrapper with
only its source bind changed to the isolated source directory; module mode is readonly.

Before Home changes, detached `VACUUM INTO` backup completed and passed full integrity:

- `/mnt/user/appdata/cpa-home/data/backups/home-pre-home-native-fleet-20261015T222408Z.db`.
- 6,786,383,872 bytes, mode 0600, integrity `ok`.
- SHA256 `49f586e835c7468b7497089c6e043b5975fb71d54d9ba2a99681c6cce2e469fe`.

Image rollback target, independently read from the running container:
`sha256:76499632b410686ec14a68abbd26360369bd82cbace1ed77bc4617a2c207ba72`
(`cpa-home:1.1.1-claude-fleet-f6ebd63`). Preserve the current DB and rotating tokens on rollback.
All 64 current panel assets match the retained official 1.1.1 bundle and will be preserved.

The three MacBook fleet schedulers were unloaded before the migration. Their plists/settings
are retained privately pending permanent retirement after Home-driven acceptance. CodexBar's
local token-sync jobs, the MacBook CPA node and native application logins are outside this change.

Private evidence: `/tmp/cpa-home-native-fleet/` on the Mac and
`/mnt/user/appdata/cpa-home-build/ws0031/` on Ark. No secrets enter the repository or reports.

## Completed cutover and acceptance

- Deployed Home image `cpa-home:1.1.1-claude-fleet-2800d59`, immutable ID
  `sha256:3f421772d8ddf859a9e29c415540c2bb97cd7de2d639688598309b6715c57122`.
  Container started `2026-10-15T22:39:41.074068532Z`, zero restarts. An immediate first HTTP
  check raced normal startup; the ready instance and all subsequent checks passed.
- Before and after cutover, all six original client keys completed real Opus 5.5 and Sol 6.1
  native requests. All five node memberships and required plugin reports are healthy.
- All 15 credential settings, full client-key/configuration fingerprints, channel bindings,
  model-group details and all 64 served panel assets match the pre-change baseline.
- Home policy is enabled with a 3600-second periodic interval, catalog sync and guarded stable
  patch updates. Moxy, Hyper, Lara and Chip are required, in that order; MacBook is optional.
- Home itself fetched and verified CPA 8.0.30's official archive/binary hashes. No new CPA
  release was available, so no artificial binary upgrade or downgrade was performed.
- With all three laptop coordinator jobs unloaded and their plists removed from login startup,
  Home issued and recorded successful catalog tasks: Moxy 22:43:42 UTC, Hyper 22:44:43,
  Chip 22:45:46 and Lara 22:49:53. The optional MacBook executor was not installed until
  after all four required nodes succeeded, proving the control plane does not require it.
- Lara's first tasks reported a local adapter failure. Its OpenClaw CLI/runtime are managed by
  fnm rather than the hardcoded Homebrew path. The installer now resolves real executable
  directories and the catalog adapter uses the existing bounded login-shell CLI pattern.
  Subsequent scheduled Home execution passed. Failures remain honestly recorded in history.
- Node worker runtime bundle: `d7b02b0aa45205aa`. It contains no Management secret or cross-node
  SSH coordinator. The MacBook worker reports `current` and only executes its own optional node.
- Home's required-node acceptance attestation reached the MacBook's private worker receipt:
  schema `home-fleet-accepted-core/v1`, CPA 8.0.30, all four required node IDs verified.
  Agent-os enrollment now prefers this Home-attested hash, instead of a laptop coordinator pin.
- Moxy/Chip Sol 6.1 and Lara/Hyper Opus 5.5 selections, matching fallbacks, librarian primary
  tiers and active conversation pins remain preserved. Catalog synchronization does not promote
  defaults automatically.

The older laptop coordinator implementation and its picker-review wrapper are retired from
agent-os; its installer exits with a Home handoff notice. Legacy plists are archived at
`~/.local/state/cpa-home-fleet-migration/retired-launchagents/` for history, not in the login folder.
Old one-tap/manual recovery tools remain historical; the Easy pilot remains disabled.

Verification: full Go suite and compile on Ark; deterministic fleet state-machine, lease,
identity, rollback, review, namespace and Management API tests; seven offline worker tests;
25 focused agent-os enrollment/pin tests plus the Home-receipt case; workflow lint passed.
No existing CPA payload shape changed and the live official CPA 8.0.30 clients passed after cutover.
The node executor protocol is versioned/additive. Major/minor approval is exposed in Home's
Management API; the existing panel bundle is preserved rather than claiming new frontend controls.

For manual maintenance, pause Home policy and let active tasks finish/reconcile before taking
normal fleet locks. A permanently unavailable task owner requires operator reconciliation;
expired tasks never permit silent fleet advancement. Worker/config/binary receipts remain private.
