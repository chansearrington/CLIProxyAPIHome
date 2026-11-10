# Character8201 Claude shared setup and main synchronization

Status: complete — priority 12/shared binding 30; all six keys and five nodes pass; twelve-account CodexBar sync follows the new account; backup retention applied; maintenance restored and locks released.

The user identified the new account as `character8201@gmail.com`. Home's enabled Claude
credential `9e99dc31-29a0-404d-98a0-ef93bdb188a7` retains its UUID/history and Home's sole
ownership of OAuth refresh tokens.

## Repository synchronization

Preserved all existing tracked edits and untracked recovery records before fast-forwarding
`fleet` from `897d9c6` to deployed `origin/fleet` (`35594d7`). Merged the three remaining
upstream main commits into fleet at `8229bf3`. Fast-forwarded main to upstream `7c451f1`.
After explicit user approval, pushed both branches to the fork. Existing local work was
restored without conflicts. No Home image or node binary was changed.

Ark Docker verification using `golang:1.26-bookworm` passed focused `internal/proxyutil`
tests and a Home compile. The temporary compile output was removed.

## Routing and CodexBar

The user's clarified rule assigns each new account the next highest priority number,
without renumbering any existing account. Existing Claude priorities remain 1–11; the new
account is **12**, with shared channel-group-1 binding **30**. Original API keys, model scopes,
other providers, existing account settings and inference configuration match the saved baseline.

Saved the full Home maintenance policy, paused it with no active task, and acquired all four
canonical fleet locks with independent renewal and supplemental 60-second heartbeats.

Added the exact email as a CodexBar Claude token account and pinned its Home UUID/email and
CodexBar UUID in the existing 120-second sync settings. Anthropic confirmed `claude_pro`, Stripe
subscription, default Claude rate-limit tier and `trialing` status. Staged CodexBar 0.83.0 OAuth
usage passed with 0% session and weekly utilization. The fresh Home quota collection succeeded, with a provider-backed **Pro** badge and healthy
session/weekly windows. The installed helper succeeds with all
**twelve** mappings and follows the new account from successful MacBook/key-6 traffic. Private
Microsoft remains skipped because its existing Home authorization has expired. No refresh token
or management password leaves Ark. Atomic config/settings publication held the shared sync and
config locks and preserved existing account identities, rotating tokens and unrelated providers.

## Live proof

Every request returned HTTP 200 with the expected response and exact Home attribution to the
new credential. Unique request user-agent markers correlate each completion with its usage row.

| Node/key/model | Home usage ID |
| --- | --- |
| MacBook key 1, Sonnet 5.5 | 2744936 |
| MacBook key 2, Sonnet 5.5 | 2744937 |
| MacBook key 3, Sonnet 5.5 | 2744939 |
| MacBook key 4, Sonnet 5.5 | 2744938 |
| MacBook key 5, Sonnet 5.5 | 2744941 |
| MacBook key 6, Sonnet 5.5 | 2744942 |
| MacBook key 6, Opus 5.5 | 2744944 |
| Moxy key 4, Sonnet 5.5 | 2744945 |
| Hyper key 2, Sonnet 5.5 | 2744946 |
| Lara key 5, Sonnet 5.5 | 2744947 |
| Chip key 3, Sonnet 5.5 | 2744949 |

## Backup scope and retention

The initial blanket full-database backup requirement was too broad for this configuration-only
change. The user corrected it: save affected configuration and a scoped rollback, without copying
usage/history. This policy is now recorded in FLEET.md and the orchestration guards.

The targeted pre-change settings/bindings backup is
`/mnt/user/appdata/cpa-home/data/backups/home-config-20261107T151918Z.json`, **13,532 bytes**,
mode **0600**. It contains settings and bindings, not OAuth secrets or client API keys.

Stopped the waiting coordinator before switching to targeted config backup. Both full-snapshot
validators were stopped; the unnecessary unverified November 7 snapshot was discarded. No
unattended account-mutation coordinator remains.

Applied the user's retention rule to the discussed backup directory: keep the two latest verified
database versions across all operation names, and at most two versions of every other backup type.
Deleted **38** superseded/unverified database snapshots and **11** obsolete sidecar/log files,
reclaiming **206,318,526,701 bytes**. The directory now has **10 files**, **49,179,650,956 bytes**,
including the two verified database backups:

- `home-pre-118-20261106T1430Z.db`.
- `home-pre-reconnect-20261105T2300Z.db`.

The other eight files are the targeted configuration backup, distinct historical archive types,
and at most two versions of each payload type. Retention supersedes older briefs' instructions
to keep individual historical database snapshots. Private operation evidence remains in
`/mnt/user/appdata/cpa-home-build/ws0042-character8201/` on Ark and
`/private/tmp/cpa-account-20261107/` locally.

## Scoped rollback

Remove shared binding 30 and clear only the new credential's priority. Keep the credential and its
authorization/history. Local rollback removes only its CodexBar UUID and exact helper mapping
under the existing locks; preserve all current rotating tokens. Never restore an old whole
database or config over newer authorizations.

The exact saved maintenance policy was restored after acceptance. All four operation-owned
fleet locks were released. All original account settings/priorities, client keys/scopes, model
bindings and inference configuration were verified against the targeted pre-change baseline.
