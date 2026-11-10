# Credential reconnect: cutover handoff

Status: deployed and verified, 2026-11-06 UTC. Maintenance restored; deployment locks released. Upstream upgrades remain on hold.

## Live acceptance

Chanse approved deployment in chat; out-of-band ship decision is recorded as Inbox card
`JNxqfwqHob`. Home now runs `cpa-home:1.1.1-claude-fleet-4cdbc4b` with immutable ID
`sha256:05509e8110d9229e3ed96897e0e913b90b5e32a3cd62313c094b6d779efe7884`, started
`2026-11-05T23:52:51.005291191Z`, zero restarts. All six client keys returned native Sonnet
200 through the MacBook node; four mini-local requests also returned 200. All five memberships
are healthy. Both affected account UUIDs passed live reconnect-start/cancel checks, and all
64 panel files load. All 18 original accounts, routing metadata, bindings, keys and sampled
historical rows remain intact. Hype's operational sign-in state changed from disabled to active
after the initially successful preservation check; its original UUID and routing remain intact.
Microsoft still reports sign-in required. No real-provider sign-in was performed by the agent.

The full maintenance policy was restored exactly, and independent lock guards were stopped.
Deployment evidence and compose rollback snapshot location are recorded under
`/mnt/user/appdata/cpa-home-build/ws0040-reconnect/deploy/`. Rollback is image-only to the
previous `897d9c6` image; never restore stale rotating tokens from the full DB snapshot.

The preparation record below describes the state before the approved cutover.

The user requested an in-place Reconnect action so a revoked Claude login does not require
deleting an account or splitting its history across a replacement UUID. The feature also
supports Codex, including verifying workspace identity and updating a changed subscription tier.

## Accepted source and artifact

- Backend PR: <https://github.com/chansearrington/CLIProxyAPIHome/pull/26>, merged into
  `fleet` at `5e3326fb05571fddcd7af7fb1f70134f5921e0e6`; reviewed source
  `4cdbc4bb18e1c1b13e70e4ebd95563e9d6f908a1` is an ancestor of that branch.
- Panel PR: <https://github.com/chansearrington/Home-Management-Center/pull/1>, merged into
  its new `fleet` branch at `acf766267359cc48ba230e9ec2dd023882be7b4e`; reviewed source
  `517bef3924af70cefe66dd18540017c907171beb` is an ancestor. Its baseline is the Home 1.1.1
  panel generation, upstream `3813264`, without unrelated upstream UI upgrades.
- Both final local reviews reported **MUST 0 / SHOULD 0 / NICE 0**. The initial findings
  about stale reconnect responses and Codex subscription-tier retention are fixed and tested.
- Image: `cpa-home:1.1.1-claude-fleet-4cdbc4b`.
- Immutable image ID:
  `sha256:05509e8110d9229e3ed96897e0e913b90b5e32a3cd62313c094b6d779efe7884`.
- Full backend suite (`go test -vet=off ./...`) and production Docker build pass. Vet exclusion
  is the documented existing unreachable-code finding. Eleven focused frontend tests,
  typecheck, lint, formatting and embedded build pass. Knip findings exactly match the baseline.
- Final-image testing used one fake disabled account in a separate DB. Reconnect start,
  cancellation, sign-in-required flags, unchanged credential IDs, and **64/64** served panel
  files pass. The browser host was unavailable; rendered component and handler tests passed,
  but no interactive visual browser check or real-provider login is claimed.

## Backup and current live service

Live Home is still `cpa-home:1.1.1-claude-fleet-897d9c6`, image
`sha256:c0bd56efb239136db6f2fad35235b59470e147dda48480d1a24fdae5fe925766`.
No Home image swap, upstream upgrade, node change, real-account token update, credential
deletion, priority change, or scope change was performed for this feature.

Detached `VACUUM INTO` snapshot:

- `/mnt/user/appdata/cpa-home/data/backups/home-pre-reconnect-20261105T2300Z.db`
- **24,232,837,120 bytes**, full `PRAGMA integrity_check`: **ok**.
- SHA-256: `18c2b4043a86a1466385b99d64381163457ca28e7bf0ef23125784700373cb3d`.

The original verification through Unraid's FUSE share was slow. After VACUUM completed,
verification used the read-only physical SSD alias under `/mnt/cache/`, first checking size
and first/last blocks against the share path. This avoided the FUSE overhead without reading
the live production DB through mixed locking paths. Full integrity and SHA verification
took 326.6 seconds on the completed snapshot. The superseded verifier was stopped only after
VACUUM had completed. Never restore this whole DB over subsequently rotated credentials.

The original maintenance policy was restored exactly. All four canonical locks were released;
their 60-second independent heartbeats ended. Temporary Home test containers and the SSH
forward were removed. A native Claude request through the MacBook's local node returned 200
during acceptance. The two real Claude accounts still require user sign-in after deployment.

## Next action

The deployment runbook requires a decided ship card before replacing live Home. The code and
artifact are ready for that cutover decision; the broader Home 1.1.7 upgrade remains on hold.

Private Ark evidence is under `/mnt/user/appdata/cpa-home-build/ws0040-reconnect/`, including
`final-candidate.json`, `backup-direct.json`, `maintenance-restored.json`, `cutover-plan.json`,
and a prepared `docker-compose.reconnect-candidate.yml`. The prepared compose changes only
the image reference; its production baseline SHA-256 is
`593052b08ab38c07ad088343c8188d05b9c5e42890d72e7e002c2dbf94078751`.

Before a later cutover, recheck the live image, compose baseline, backup age and fleet state;
pause maintenance and reacquire all four locks with independent heartbeats. Verify the
candidate and rollback immutable IDs, preserve every credential UUID and binding, and run
native inference/account-scope acceptance after swapping. Use image rollback for a failed
cutover; do not restore stale rotating credentials from this snapshot.
