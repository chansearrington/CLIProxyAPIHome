# Developer Claude subscription

Status: shared access, CodexBar, and the Home quota correction deployed and verified, 2026-10-10 03:24 UTC. All fleet locks released.

Chanse added `developer@hypesports.live`, requested access for every user and the usual
CodexBar integration, and chose the priority order below. The initial restricted session
could not reach Home or edit the app's settings. Chanse restored unrestricted access, and
live verification and configuration then completed.

## Shared access

The credential is `01e7d73e-7783-41a1-a84c-1fed7a005257`, built-in Claude OAuth. It was enabled
and active but had no channel-group binding. Each of the six active client keys already
included group 1. Without a credential binding, none of those scoped keys could select it.
Anthropic's live OAuth profile independently confirmed its exact email.

Under all four canonical fleet locks, created binding **10** to group **1**,
`Fleet shared — Claude, Codex, Antigravity, xAI`, at **2026-10-10 02:55:48 UTC**.
The binding is the only access configuration change. All existing keys, user ownership,
model scopes, credential priorities and routing overrides, groups, and other bindings were
verified unchanged. Microsoft and Copilot remain in the Chanse-only group 2; only key 6 has
that scope. No token refresh was forced and no node was restarted.

| Order | Claude account | Priority | Scope |
| --- | --- | --- | --- |
| 1 | `chanse@hypesports.live` | 5 | Fleet shared |
| 2 | `chanse.arrington@gmail.com` | 4 | Fleet shared |
| 3 | `the.lara.vance@gmail.com` | 3 | Fleet shared |
| 4 | `developer@hypesports.live` | 2 | Fleet shared |
| 5 | `carringt@microsoft.com` | 1 | Chanse only |

Between 02:55:46 and 02:57:03 UTC, all six keys completed native streaming
`claude-sonnet-5-5` requests through the MacBook CPA node: HTTP 200, `message_stop`, and `ok`.
Successful Home ledger rows in that window show Developer for keys 1–6, including Hyper,
Chip, Moxy, Lara, Chanse, and the unowned legacy key. Examples: 204652, 204655, 204656,
204660, 204662, and 204648. Attribution uses the time window, node, model, and key identity;
client request IDs are not claimed to match generated Home ledger IDs.

Hype and Gmail were weekly-exhausted and Lara was five-hour-exhausted, so selecting Developer
matched the configured order. The completed Home quota refresh at 02:57:49 UTC was healthy,
fresh, successful, and had zero collection failures: 4% session used and 2% weekly used.
Later independent provider usage and CodexBar reads agreed on healthy weekly usage; session
usage increased with concurrent real traffic. Final credential state was enabled, active,
available, priority 2. All four locks were released and independently reported UNLOCKED.

### Backup and rollback

Before the binding, detached `nohup` VACUUM INTO backup, full integrity, size, SHA-256, and
private mode checks passed:

- `/mnt/user/appdata/cpa-home/data/backups/home-pre-developer-shared-20261010T025217Z.db`
- 4,148,621,312 bytes; integrity `ok`; mode 0600.
- SHA-256 `9f048c76199ee60a6b9fafce0d8a9c032f24696115526e1b656079dc2b5c2b67`.

To undo only shared access, reacquire all canonical locks and delete
`/v8/management/channel-group-details/10`. Do not restore a stale database over newer
rotating OAuth authorizations. The fresh backup is retained.

## CodexBar

Added **Developer** as the fourth Claude account, after Hype, Gmail, and Lara. CodexBar UUID
`900A77D6-2ADD-4607-8AD6-5A5FF4DD5CDF` is pinned to the exact Home UUID and email in the existing
helper settings. No source change to the helper was needed. Home remains the refresh-token
owner; only its access token is copied to CodexBar.

A private staged configuration passed independent provider profile verification and all-four-
account OAuth usage checks. Paused the sync LaunchAgent, quiesced the app during installation,
preserved unrelated config and the selected account, installed the account and mapping, and
resumed the helper. Live all-account usage exited 0; all four sources were OAuth without errors.
Developer initially reported 0% session and weekly usage before live traffic began.

The visible Claude settings list contains Hype, Gmail, Lara, and Developer. Selecting Developer
showed OAuth, 7% session usage and 2% weekly usage. The final persisted selected
account is Developer; menu selection does not change Home routing priorities. The scheduled LaunchAgent completed three runs with exit 0; its 03:00:29 UTC status
contains all four exact identities and expiries. No native Claude Keychain credential was read
or changed. Private pre-addition files `config.pre-developer.json` and
`settings.pre-developer.json` are retained in `~/.local/state/cpa-codexbar-sync`, mode 0600.

Rollback only this app addition by pausing the helper, removing Developer from CodexBar and
its mapping from the helper settings, then resuming the helper. Whole-file restore can overwrite
unrelated newer settings and replace current access tokens with stale values.

## How to share the next account in the UI

Verified the actual Home interface read-only, including the Add binding dialog:

1. Add the subscription under **Upstream → Account Credentials**, enable it, and set its priority.
2. Open **Users & Access → Credential scopes**.
3. Under **Fleet shared — Claude, Codex, Antigravity, xAI**, click **Add binding**.
4. Search for the account's email, check its checkbox, and click **Add binding** in the dialog.

The dialog explicitly explains that client keys bound to the scope can use its credentials.
Already bound accounts appear as disabled checkboxes with **Bound**. Do not select Microsoft
or Copilot for the shared scope.

The Users tab shows every current user already has the shared credential scope. For a new
user, ensure their client key includes the same shared scope; leaving a key's scope empty is
unrestricted, not a safe replacement for the shared scope. Home access and CodexBar's local
account list are separate settings. Future app additions also need an exact account mapping
in the installed access-token sync.

## Quota percentage bug discovered during verification

The first active probe displayed weekly 100% used, despite the account's low live usage. A
subsequent completed probe showed 2% correctly. Source inspection found a deterministic
conversion defect: `normalizedProviderRatio` divided by 100 only when the value exceeded 1.
Claude's `utilization` is already a percentage, so 1% became a ratio of 1 (100%), and 0.5%
became a ratio of 0.5 (50%). No raw provider body was captured for the first probe; the exact
first-probe input is therefore inferred, not independently proven.

The new regression test reproduces failures at 0.5% and 1% in session, weekly, and extra-usage
windows. The corrected helper always divides Claude percentages by 100 and clamps to the
existing range. Tests cover 0%, 0.5%, 1%, 2%, 50%, and 100%. The focused quota package, full
Home suite, and Home compile passed in Ark's `golang:1.26-bookworm` container. No payload,
Management API schema, or CPA node contract changed. PR:
https://github.com/chansearrington/CLIProxyAPIHome/pull/13.

### Quota correction deployed

PR #13 was reviewed at `2007f0f5bcd27c0bf2a64e66fd6566c06748e228` with zero MUST, SHOULD, or
NICE findings and synchronous independent validation. It merged to fleet as
`de715bd7252ceae5bf274887ce0ef9db6fd5f927`. The only source changes relative to the tested fleet
base are the percentage conversion and its regression test. Before building, all 64 panel
assets were independently verified against the live server and the retained mirror.

Under all four canonical locks with 45-second renewal, made another fresh detached backup
including the newly added credential binding, checked its full integrity and SHA-256, built
the CGO Docker image, and changed only the Home service's image reference. No node executable
or client settings changed.

- Backup: `/mnt/user/appdata/cpa-home/data/backups/home-pre-developer-shared-20261010T031544Z.db`.
- Size 4,160,606,208 bytes; integrity `ok`; mode 0600.
- SHA-256 `07d8bd6439c514dda362d53d32e4834e9e8de3ade4232367d6d7ed47ea3c6d53`.
- Compose backup: `/mnt/user/appdata/cpa-home/backups/compose-pre-claude-percent-20261010T031544Z.yml`, private.
- New image: `cpa-home:1.1.0-claude-fleet-de715bd`.
- Immutable ID: `sha256:19143382b131c59add5e65ba2e83831a66c4bc127b0262b88a21c6024c833874`.
- Cutover/start: **2026-10-10 03:22:35 UTC**; native proof complete **03:23:26 UTC**.
- Image-only rollback: `sha256:dad5e5d84bd9931dfacb1927bb35990f37bbe7487f5252e69f411ddc83b94809`;
  restore the saved compose image reference and recreate only service `home`. No database restore.

After cutover, every active key again passed native streaming Sonnet: six HTTP 200 replies
with `ok` and `message_stop`. Home ledger rows 206008, 206010, 206012, 206013, 206014, and 206070
confirm Developer attribution for keys 1–6. Keys, scopes, bindings, and priorities were verified
unchanged. A new completed quota probe at 03:24:41 UTC was healthy/fresh/successful, zero
failures: 19% session used, 5% weekly used, with increasing usage from real concurrent traffic.
All five Home node memberships reconnected and were checked healthy at 03:25:57 UTC, each
with plugin report state `reported_ok`. All four locks were
released and independently reported UNLOCKED. CodexBar's scheduled four-account sync remains
successful after the Home restart.


The plain-language HTML guide was updated with the four-account pool, automatic CodexBar
authorization, the percentage fix, and verified self-service sharing steps.
