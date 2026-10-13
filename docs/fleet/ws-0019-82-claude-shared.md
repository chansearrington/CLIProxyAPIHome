# Shared Claude account 82 and CodexBar menu visibility

Status: configured and verified, 2026-10-13 UTC. All fleet locks released.

Chanse requested the established setup for **82**, `character82@gmail.com`: shared CPA access,
CodexBar authorization, automatic token rotation and account following. A later screenshot
showed that the account was absent from the visible stacked menu.

## CPA Home

Home UUID `1f648cbe-96b5-4401-8062-1d7a462e483e` is enabled, active, and available at final verification.
Final scope binding **15** connects it to shared group **1**. All six current client
keys include this scope. Microsoft Claude and Copilot retain their private group 2 binding,
limited to Chanse's key 6. No key ownership or existing scope was changed.

Priorities changed during the user's live configuration work. We did not patch any priority;
the latest observed order is Gmail (10), HypeSports (9), **82 (8)**, Chip (7), Developer (6),
Moxy (5), Lara (4), then private Microsoft (1). The initial read had no 82 priority, followed
by 2; subsequent reads observed the new order. Preserved those changes instead of overwriting
them with the earlier snapshot.

Configuration writes were serialized under all four canonical fleet locks with renewal.
Private pre-change snapshots and an inverse binding API rollback were retained, following the
prior immediate account-restoration workflow. The only lasting Home write is scope binding
15. Early binding 13 was rolled back after concurrent priority changes; binding 14
was rolled back after native requests reached an actual provider rate limit. The final binding
was validated against current enabled state, key ownership/scopes, existing bindings, and
healthy memberships. This account-only restoration used targeted snapshots rather than a new
full multi-gigabyte database backup. No Home image or node restart occurred.

Anthropic independently confirmed the exact email and initially reported 0% usage. All six
keys then completed HTTP-200 Sonnet requests through the MacBook. Home's successful 82 ledger
examples for keys 1–6: 1 → 305849, 2 → 305845, 3 → 305843, 4 → 305846, 5 → 305844, 6 → 305822.

During live traffic, Anthropic returned rate-limit errors; Home's successful active collection
and CodexBar independently reported 100% session usage. A later provider read reported quota
available again (30% session used, 4% weekly used), and real 82 inference resumed. No cause for
that provider-side quota change was independently established; the earlier exhausted snapshot
is historical, not the final state. No cooldown reset or OAuth refresh was forced.

Final streaming Sonnet requests directly through Moxy, Lara, Chip, and Hyper returned HTTP 200,
`message_stop`, and `ok`. Home ledger rows **306681, 306683, 306680, and 306682**, respectively,
confirm 82 attribution. The MacBook's latest example is 306757. All five memberships were
healthy with plugins `reported_ok`, and each remote node's authenticated model catalog also
returned HTTP 200. All four locks were independently checked unlocked.

Rollback only shared access by reacquiring the locks and deleting
`/v8/management/channel-group-details/15`. Do not restore stale OAuth tokens or overwrite
the user's newer priority choices.

## CodexBar and the visible menu

Local account label **82**, UUID `13F066B6-B2BA-405C-A12E-933B13B4D689`, is pinned to the exact Home UUID
and email. There are now seven shared Claude accounts plus private Microsoft: eight local
accounts total. Staged all-eight-account OAuth usage succeeded before installation. Paused
sync and quit the app during installation, then resumed both. Only access tokens are copied;
Home retains refresh-token ownership. 82's local access token matched Home's current token.
The existing MacBook/key-6 automatic account-follow setting is preserved.

The missing row had a separate display cause. CodexBar 0.60.3 has a **six-account snapshot
limit** in stacked mode. It takes the first six stored accounts and replaces the last with
the selected account if necessary. With Microsoft selected outside the original first six,
the main Microsoft card plus Hype/Gmail/Lara/Developer/Chip exactly matched the screenshot.
82 and Moxy were correctly registered, but outside that visible set.

Verified against the pinned primary source:
[UsageStore token-account limit and selection](https://github.com/steipete/CodexBar/blob/v0.60.3/Sources/CodexBar/UsageStore%2BTokenAccounts.swift#L162).

Reordered the local list to **82, Moxy, Chip, Hype, Gmail, Lara, Developer, Microsoft**, retaining
all eight accounts, tokens, IDs, unrelated settings, and the selected identity. This changes
menu presentation, not CPA scheduling. Reordering is safe because automatic selection resolves
UUIDs to the current list index. The app was relaunched. Final saved selection and the automatic
sync follow 82, so it appears as the main account; it also stays within the first visible rows
when CPA later switches elsewhere. Other accounts remain registered for account switching.
No post-change screen capture was taken; verification used pinned renderer source, persistent
list/selection, live OAuth usage, and the running app.

## Automatic-follow runtime compatibility

An incidental status check found an intermittent `invalid_timestamp` result. Apple's installed
Python 3.9.6 rejects valid RFC3339 timestamps with five fractional digits, while Home emits
variable precision. A real usage row **306419**, timestamp
`2026-10-13T04:21:04.02318Z`, reproduces the rejection; six-digit rows parse successfully.

Changed only the sync LaunchAgent's interpreter to the already-installed
`/opt/homebrew/bin/python3` (**3.14.7**), preserving its 120-second interval. No helper source
change or new Python installation was needed. Fresh automatic sync at **2026-10-13T04:28:38.168367+00:00**
succeeded with `following` mode and selected **82**. The newer
runtime accepts Home's timestamp precision. Usage-following setup now documents Python 3.11+
while legacy token-only sync can still use Python 3.9.

## Local rollback

Private mode-0600 backups remain under `~/.local/state/cpa-codexbar-sync/ws0019/`:
`config.pre-82.json`, `settings.pre-82.json`, `config.pre-menu-order.json`, and
`sync-agent.pre-python.plist`. To remove only 82, pause sync, remove its local account and exact
mapping, then resume. To undo menu order, reorder current account objects by their old IDs,
preserving current tokens and selected identity. Avoid whole-file restoration over newer
settings or token rotations. Returning to Python 3.9 also returns the timestamp limitation.
