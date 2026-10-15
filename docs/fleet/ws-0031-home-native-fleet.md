# Fleet maintenance belongs to Home

Status: staged — full Ark Go suite/compile pass, worker tests pass, backup verified; cutover pending.

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
