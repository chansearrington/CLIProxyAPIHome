# Moxy Claude subscription

Status: Home and CodexBar configured and verified, 2026-10-12T15:41:00.281100+00:00. Fleet locks released after proof.

Chanse added `moxy.kit@outlook.com` and requested the established shared setup, then explicitly
prioritized immediate activation in CPA Home. Home credential UUID:
`3ed52fd4-9f8c-4be4-a766-b97b45a2e1d5`.

## Immediate Home activation

The credential was already enabled, active, available, and priority 2, but had no scope binding.
Created binding **12** to shared scope **1** at **2026-10-12T15:38:21.414342+00:00**, under all four canonical
fleet locks with automatic renewal. Every current user's client key includes scope 1.
Only that binding changed; key ownership, all key scopes/model groups, existing bindings,
credential enabled states, and priorities were independently compared and preserved.

The configured order remains HypeSports (6), Gmail (5), Lara (4), Developer (3), **Chip and
Moxy (both 2)**, then Microsoft (1). Chip and Moxy have equal priority; no new strict order
between them was imposed. Microsoft and Copilot remain private to Chanse in scope 2.

To prioritize the requested immediate restoration while the existing shared accounts were
unavailable, captured a private mode-0600 snapshot of the exact pre-change bindings and
non-secret key/credential configuration at `/private/tmp/ws0017-before.json`. This narrow,
reversible API change used its inverse operation for rollback instead of waiting for another
full multi-gigabyte VACUUM backup. The earlier verified full backup from ws-0016 is retained;
it predates this authorization and must not be restored over current rotating tokens.

Rollback: reacquire all four canonical locks and delete only
`/v8/management/channel-group-details/12`. No Home or node deployment/restart was needed.

## End-to-end verification

Anthropic's OAuth profile independently matched the exact email. The initial provider usage
and staged CodexBar read reported 0% session and weekly utilization. Home's own active quota
probe completed successfully: **healthy**, **fresh**, collection
**success**, zero failures, at **2026-10-12T15:38:43.090107638Z**.

All six client keys completed streaming `claude-sonnet-5-5` requests through the MacBook
between **2026-10-12T15:38:42.627647+00:00** and **2026-10-12T15:41:00.281100+00:00**, each HTTP 200, `message_stop`,
and reply `ok`. Requests directly through Moxy, Lara, Chip, and Hyper likewise returned 200
and `ok`. All five memberships remained healthy with plugin reports `reported_ok`.

Home's ledger independently contains successful inference through this exact **Moxy credential
for every key ID 1–6 and all five node identities**. Example key-to-ledger rows:
1 → 276592, 2 → 276600, 3 → 276609, 4 → 276605, 5 → 276607, 6 → 276572.
Correlation uses time, model, node, key, and credential identity; client request IDs are not
claimed to equal Home-generated ledger IDs. Existing higher shared accounts were unavailable,
so this provides actual Moxy selection proof without temporarily changing anyone's priority.

## CodexBar

Added **Moxy** as the sixth shared Claude account, UUID `5C98DA3F-11BB-46FE-A0CB-5340C213CEB1`, pinned to the
exact Home credential UUID and email in the existing helper settings. Staged all-six-account
OAuth usage passed before installation. Paused the helper, quit the app to avoid overlapping
settings writes, installed the account and mapping, and resumed the app and 120-second sync.
Preserved the current selected account (index 4, Chip) and all unrelated settings, independently
compared with pre-addition snapshots.

Live all-six-account usage exited 0 with OAuth and no errors. Moxy's later live usage read
showed 4% session and 1% weekly usage after real fleet traffic began. Scheduled six-account
sync succeeded at **2026-10-12T15:41:04.356600+00:00**. Moxy's local token exactly matched Home's current
access token at final verification. Refresh tokens and the management password remain on the
Ark; only access tokens are copied. No native Claude Keychain credential was changed.

Private mode-0600 backups are retained under
`~/.local/state/cpa-codexbar-sync/ws0017/` as `config.pre-moxy.json` and
`settings.pre-moxy.json`. Staged token configuration was removed after verification.
To undo only this app addition, pause sync, remove Moxy's CodexBar account and its exact helper
mapping, then resume. Avoid restoring entire old configs over later edits or refreshed tokens.

For the next account, use **Users & Access → Credential scopes → Fleet shared → Add binding**
after adding/enabling it under Upstream. Existing users already have the shared scope.
CodexBar still needs its own token account and pinned helper mapping.
