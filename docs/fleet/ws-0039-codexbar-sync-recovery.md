# Restore CodexBar's Home account source

Status: repaired and verified locally, 2026-11-05 UTC — nine authorized Claude accounts restored; four Codex accounts working; launchd reruns pass. Two Home-disabled Claude accounts require reauthorization in Home.

The user asked to restore CodexBar's CPA Home source before any fleet upgrade. No Home or
node upgrade, restart, credential mutation, or routing change was performed.

## Cause

CodexBar is 0.83.0. Both two-minute LaunchAgents were loaded. The Codex job was succeeding,
including scoped usage following. The Claude job repeatedly exited 1 with `Home access token
expired or near expiry`.

The helper validated all eleven Claude credentials before publishing any token. HypeSports
Claude (`chanse@hypesports.live`) and Microsoft Claude (`carringt@microsoft.com`) had expired
access tokens. Home's current `/credentials` response marks both disabled and unauthorized.
Their expired tokens stopped sync for all nine other accounts, leaving all eleven local
Claude usage checks unauthorized even though Home had renewed nine credentials.

This was a local synchronization failure, not evidence that all Home credentials were invalid.
The disabled identities need to reconnect through CPA Home; this repair preserves their
mapping, disabled state, and refresh ownership.

## Repair

Updated `tools/codexbar-claude-sync/sync.py` to distinguish expired tokens from malformed or
untrusted responses. Skip expired/near-expiry accounts, retain their existing local tokens,
publish valid Home tokens, and prevent a skipped account from becoming the followed selection.
Every polling run retries the full original mapping, so a reauthorized account resumes on
its next successful poll. Status reports `health: degraded` and names `skipped_accounts`.

Identity mismatches, unexpected fields, missing or duplicate mappings, malformed timestamps,
and provider-profile verification failures still fail closed. Home management 401/403 retains
the existing authentication latch. No refresh tokens leave Home. Claude writes now use the
same CodexBar config lock as the Codex helper, retaining optimistic concurrent-edit checks.

Installed identical reviewed bytes into both existing local helper copies:

- `~/.local/lib/cpa-codexbar-sync/sync.py` (Claude LaunchAgent).
- `~/.local/lib/codexbar-claude-sync/sync.py` (shared utilities imported by the Codex helper).

The existing 120-second jobs, eleven Claude and four Codex mappings, account identities,
labels/order, provider settings, and native CLI logins remain intact. Compared every config
field except the intended Claude token fields and active selection after installation;
all unrelated configuration matched.

## Evidence

- All **21 focused Python tests passed on Ark inside `golang:1.26-bookworm`**. New cases
  cover mixed expired/valid rotation, automatic recovery after renewal, all-expired no-op,
  retained identity/schema checks, and refusing to follow an expired account.
- A private staged config independently verified the nine new token owners through Anthropic
  profiles. Installed CodexBar read usage successfully for all nine; the two disabled Home
  accounts retained authorization errors as expected.
- Before repair, all four Codex accounts passed live CodexBar OAuth usage reads. The current
  Codex sync status was successful and followed Developer from MacBook/key-6 usage.
- Live Claude sync exited 0 and updated all nine eligible tokens, following Hotmail from
  successful MacBook/key-6 Home traffic at `2026-11-05T22:46:08.893653Z`.
- Launchd reruns exited 0: Codex at `22:47:15 UTC` and Claude at `22:47:38 UTC`. The Claude
  rerun required no token rewrites and continued following Hotmail. Default live CodexBar
  usage checks for Hotmail Claude and Developer Codex both exited 0 with OAuth usage.
- Upgrades remain on hold. No Home, node, account priority, or account scope was changed.

Private staged config, pre-repair local config, and both old helper copies are retained in
`/private/tmp/cpa-codexbar-repair-20261105-a9t5isqb/` (directory 0700, files 0600). Do not
restore that whole config over later rotating tokens. To revert helper behavior, hold the
existing sync lock and replace only the two helper scripts; leave current credentials alone.

Remaining action: reconnect HypeSports Claude and Microsoft Claude in CPA Home. Reenabling
them without valid authorization does not repair the provider rejection.
