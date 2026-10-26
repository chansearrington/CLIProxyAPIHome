# The Vanderblooms shared Claude subscription

Status: configured and verified, 2026-10-14 19:17 UTC — all six keys and all five nodes selected The Vanderblooms; CodexBar is following it. All four fleet locks released.

The user added **The Vanderblooms** and requested the established CPA/CodexBar setup.
Independently verified that the Anthropic OAuth profile email matches Home credential
**acct-03a662**. The profile returned HTTP 200 and an active
**Pro** subscription (`claude_pro`, tier `default_claude_ai`); do not label it Max 20x.

The credential is already enabled and active but has neither priority nor shared scope
binding. The user explicitly chose **after Lara, before Microsoft**. Set only this new
credential's priority to **3**; bind it to existing shared group **1**, which all six current
keys include. Preserve all existing priorities, ownership, keys, model restrictions and
private Microsoft/Copilot bindings to group 2.

Add local CodexBar account **The Vanderblooms**, pinned by exact Home UUID/email, to the
existing 120-second access-token-only sync. Home retains refresh-token ownership. Put the
new local account first so it is within CodexBar's limited stacked menu; preserve the
selected identity and automatic following of successful Mac/key-6 requests.

All four canonical fleet locks are acquired with renewal. A fresh detached SQLite backup
is in progress. Verify its integrity/size/mode/SHA256 before Home changes. Use targeted
inverse operations for rollback, preserving newer token rotations and user changes.

Verify the new provider identity and plan, fresh quota collection, native requests for
all six client keys, and all five node health/plugin reports. Stage and test the nine-account
CodexBar configuration before installation; preserve unrelated settings/accounts. Pause
local sync and quit CodexBar during installation, then resume both. No Home image or CPA
node restart is required.

## CodexBar installation

Staged all nine token accounts and verified OAuth usage successfully for each with installed
CodexBar 0.60.3. The new account reported 0% session/weekly usage. Installed local UUID
**467E0397-2AAD-40EE-B81C-30FD30B0BC9D**, label **The Vanderblooms**, first in the stored list.
The previous selected identity and all unrelated settings/accounts matched after installation.
Held the sync helper's exclusive lock while CodexBar was gracefully quit, wrote both files
atomically, released the lock and relaunched the app. The existing LaunchAgent and its
120-second schedule remain intact; no refresh token was copied.

Private mode-0600 pre-addition backups, staged configuration and OAuth verification results
are under `~/.local/state/cpa-codexbar-sync/ws0024/`. The exact Home UUID/email mapping was
added to `~/.config/cpa-codexbar-sync/settings.json`. Home owns token renewal; the helper
copies access tokens and follows actual successful Mac/key-6 usage automatically.

## Home changes and live acceptance

Applied priority **3** and shared scope binding **20** to group **1**. No other credential,
key, owner, scope or priority changed; the normalized before/after settings fingerprint and
all pre-existing bindings matched. The credential remains enabled and active under its
original UUID. Microsoft and Copilot remain restricted to private scope 2.

Fresh Home collection at **19:15:39 UTC** succeeded with no collection errors and persisted
the **Pro** plan badge. The profile and CodexBar OAuth usage checks independently report
the same subscription identity. Do not infer Max status from the other shared accounts.

All six current keys completed native streaming Sonnet 5.5 through the Mac node with
HTTP 200, `message_stop`, and `ok`. Home records for keys 1–6:
**436145, 436146, 436147, 436149, 436144, 436148**. Every record selects this new UUID.
Key 6's native **Opus 5.5** completion also passed; ledger **436150** selects the same account.

Direct native Sonnet requests through each mini returned HTTP 200, `end_turn`, and `ok`,
without modifying their OpenClaw settings:

| Node | Home usage record |
| --- | --- |
| Moxy | 436151 |
| Chip | 436152 |
| Hyper | 436153 |
| Lara | 436154 |

All five node memberships are healthy with required plugins `reported_ok`. Other Claude
accounts carried genuine earlier provider-429 cooldowns at the pre-change read, so their
higher priorities did not displace this available account. No quota cooldown was cleared.

Installed CodexBar's new-account OAuth usage check passed. Its token equals Home's current
access token, the account is within the first six stored rows, the app is running, and the
existing LaunchAgent remains loaded with last exit code 0. Sync at **19:16:13 UTC** reports
healthy automatic following of **The Vanderblooms**, matching the latest successful
Mac/key-6 request. Both config files remain mode `0600`. No helper source change was needed.

Fresh detached SQLite backup, verified before either Home mutation:
`/mnt/user/appdata/cpa-home/data/backups/home-pre-vanderblooms-20261014T190955Z.db`;
**6,319,976,448 bytes**, full integrity check `ok`, mode `0600`, SHA256
`d77e4586ac19aa6c18533891efa74a4f4a8661e246790ed1a4ad89b32c4f0326`.
Private sanitized proof files are `/private/tmp/cpa-vanderblooms-*`; no tokens or keys are
in this repository. No Home image, CPA binary, node service, or global cooling setting changed.

## Targeted rollback

Under renewed canonical locks, remove only scope binding **20** and restore this credential's
previous unset priority with a targeted metadata patch. Do not delete the original credential
or restore stale database/token state. To undo the local addition, hold the sync lock and
quit CodexBar; remove only local UUID **467E0397-2AAD-40EE-B81C-30FD30B0BC9D** and its exact
helper mapping, preserving the latest remaining accounts, tokens and unrelated settings;
then resume the app and helper. No rollback was needed.
