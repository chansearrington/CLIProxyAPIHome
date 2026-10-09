# Third shared Claude subscription

Status: configured and verified, 2026-10-09 06:33 UTC. All fleet locks released.

Chanse added a third regular Claude subscription and requested the same setup as Hype and
Gmail: available to every user and ranked before the restricted Microsoft backup. Copilot and
Microsoft Claude retain their Chanse-only access.

## Finding and change

The newly added credential is `the.lara.vance@gmail.com`, UUID
`3e101de1-ae15-4b1d-9bb6-1ff0194936a2`. It was enabled, active, and priority 2, but had no
channel-group binding. All six current client keys are scoped to channel group 1; without a
binding, none could select the new credential.

The existing live priorities already matched the requested order, so no priority write was
needed:

| Order | Account | Priority | Access |
| --- | --- | --- | --- |
| 1 | `chanse@hypesports.live` | 4 | Fleet shared |
| 2 | `chanse.arrington@gmail.com` | 3 | Fleet shared |
| 3 | `the.lara.vance@gmail.com` | 2 | Fleet shared |
| 4 | `carringt@microsoft.com` | 1 | Chanse only |

Under all four canonical fleet locks, created channel-group-detail **9**, binding the new UUID
to group **1**, `Fleet shared — Claude, Codex, Antigravity, xAI`. This is the only configuration
change. Group 2, existing credential bindings, all keys and user ownership, model groups,
priorities, proxy/prefix settings, and cooling/retry overrides were verified unchanged.

All current users have at least one key in group 1: Moxy (user 2), Chanse (3), Chip (4),
Hyper (5), and Lara (6). The unowned key 1 also includes group 1. The regular Claude pool
therefore has the same access scope for all users; Microsoft and Copilot remain in group 2,
which is bound only to Chanse's key 6.

This credential is built-in Claude OAuth, with the same inherited refresh, model, quota, and
cooldown behavior as the two other shared Claude subscriptions. No token was copied, no OAuth
refresh was forced, and no Home or node service restarted. CodexBar was outside this Home
configuration change; its optional addition was asked separately.

## Backup and verification

Before changing the binding, a detached `nohup` backup job completed `VACUUM INTO`, full
`integrity_check`, size, and SHA-256 validation:

- Ark path: `/mnt/user/appdata/cpa-home/data/backups/home-pre-lara-shared-20261009T062919Z.db`.
- Size: **3,942,510,592 bytes**; integrity: **ok**; mode: **0600**.
- SHA-256: `ad5418c79fa5440eed01e592f0fe818e6a48a32d87dbc563122a5360b1229a09`.
- Home image remains `sha256:dad5e5d84bd9931dfacb1927bb35990f37bbe7487f5252e69f411ddc83b94809`.

Anthropic's live OAuth profile independently confirmed the new credential's exact email.
Its provider usage endpoint returned HTTP 200 and 0% usage for five-hour and weekly windows.
Home's registered model list includes `claude-sonnet-5-5`, Opus, Fable, and Haiku alongside
the standard Claude catalog.

After the binding, each of the six active API keys made a native streaming Sonnet request
through the MacBook CPA node: all returned HTTP 200, `message_stop`, and `ok`. The Home usage
records in that verification window contain successful Sonnet requests for all six key IDs,
using Gmail as expected while higher-priority Hype is exhausted. Example key 1–5 usage rows:
187537, 187540, 187541, 187542, and 187543. Successful key-6 Sonnet rows are also present.
The client-supplied request IDs are not the generated Home ledger IDs; the usage check used
the time window, node, model, and key identities rather than claiming an exact ID match.

Home's own quota fetch for the new UUID was accepted and completed at
**2026-10-09 06:33:35 UTC**: enabled, healthy, fresh, collection successful, zero failures,
100% remaining in both windows. Its weekly reset is **2026-10-11 02:00 UTC**.
The new credential was not forced ahead of Gmail for an inference test; its lower priority is
intentional. Eligibility was verified from the exact bindings, enabled state, registered models,
and successful provider/Home quota authentication.

All four locks were released and independently reported **UNLOCKED** after the change.

## Rollback

Under reacquired canonical fleet locks, delete only
`/v8/management/channel-group-details/9`. No priority or key rollback is needed. Retain the
fresh database backup; do not restore an old database over newer rotating OAuth tokens.
