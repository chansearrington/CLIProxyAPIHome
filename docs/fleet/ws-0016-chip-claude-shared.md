# Chip Claude subscription

Status: Home and CodexBar configured and verified, 2026-10-12 13:13 UTC. All fleet locks released.

Chanse added `acct-f0ee78` and requested the established shared Claude setup in CPA and
CodexBar. Home credential UUID: `acct-f0ee78`.

## Findings and intended change

The account was already enabled, active, available, and priority 2, but had no credential-scope
binding. Every current client key includes shared scope 1; without a binding those keys could
not select the account. Only the missing shared binding needs changing.

The existing priorities already put Chip ahead of the restricted Microsoft backup:

| Order | Account | Priority | Scope |
| --- | --- | --- | --- |
| 1 | HypeSports | 6 | Fleet shared |
| 2 | Gmail | 5 | Fleet shared |
| 3 | Lara | 4 | Fleet shared |
| 4 | Developer | 3 | Fleet shared |
| 5 | Chip (`acct-f0ee78`) | 2 | Fleet shared |
| 6 | Microsoft | 1 | Chanse only |

No priority changes are needed. Microsoft and Copilot retain scope 2, accessible only to
Chanse's key. No Home build, node restart, key reassignment, or native Claude login change is
part of this setup. Home alone retains ownership of rotating refresh tokens.

Anthropic's live OAuth profile independently confirmed `acct-f0ee78`. Its usage endpoint
returned HTTP 200 and 0% used for the session and weekly windows. The weekly reset was
2026-10-13 21:00 UTC. The current access token expires 2026-10-12 20:55:30 UTC.

## CodexBar

Added **Chip** as the fifth Claude account. CodexBar UUID
`5BF3653B-DF6F-4986-910B-9960356587E9` is pinned to the exact Home UUID and email in
`~/.config/cpa-codexbar-sync/settings.json`. Only access tokens are copied; refresh tokens and
the management password remain on the Ark.

The private staged config passed all-five-account OAuth usage checks before installation.
Paused the sync LaunchAgent, quit CodexBar to avoid overlapping settings saves, and preserved
the latest settings and selected account. Installed the appended account and helper mapping,
relaunched the app, and resumed the 120-second scheduled sync. No helper source change was
required. Developer remains selected (index 3); menu selection does not change CPA priorities.

Live CodexBar usage exited 0: all five accounts reported OAuth without errors, and Chip
reported 0% session and weekly usage. Scheduled sync at 2026-10-12 13:05:56 UTC succeeded with
all five exact identities. A final scheduled sync at **13:14:32 UTC** also succeeded, and Chip’s
CodexBar token exactly matched Home’s current access token. Unrelated configuration fields
were independently compared with the private pre-addition snapshots and preserved. Config
and settings remain mode 0600. Private backups are retained
under `~/.local/state/cpa-codexbar-sync/ws0016/` as `config.pre-chip.json` and
`settings.pre-chip.json`.

## Verification and rollback

Under all four canonical fleet locks, with automatic renewal, created shared scope binding
**11** at **13:09:53 UTC**, linking the Chip credential to group **1**. This was the only
Home configuration write. Key identities and ownership, key scope/model-group assignments,
all existing bindings, credential enabled states, and priorities were independently compared
before and after and remained unchanged. Every current user has a key in scope 1.

The fresh detached `nohup` backup completed before the binding:

- `/mnt/user/appdata/cpa-home/data/backups/home-pre-chip-shared-20261012T130459Z.db`.
- **4,866,740,224 bytes**, full integrity check **ok**, mode **0600**.
- SHA-256: `73a42e04e8324fd62d8963760c5dc89fe3c5b5bde5533dfce0d4d9f2ed4b92cf`.

Home's requested active quota probe completed at **13:10:06 UTC**: enabled, healthy, fresh,
collection successful, zero failures, 100% remaining for both five-hour and weekly windows.

All six client keys passed streaming `claude-sonnet-5-5` requests through the MacBook node
between **13:10:05 and 13:11:27 UTC**, each HTTP 200, `message_stop`, and reply `ok`. Home ledger
examples for keys 1–6 are **271100, 271110, 271119, 271130, 271143, and 271208**. These successful
records selected **Developer**, consistent with its higher priority while Hype, Gmail, and Lara
were weekly-exhausted. Records were correlated by time, model, node, and key; no claim is made
that a client request ID equals Home's ledger ID.

Post-change requests directly through Moxy, Lara, Chip, and Hyper also returned HTTP 200,
`message_stop`, and `ok` at **13:12:58–13:13:00 UTC**; their Home records are **271212, 271209,
271211, and 271210**, respectively. All five memberships were healthy with plugins
`reported_ok`. No services were restarted. All four locks were released and independently
checked unlocked.

Chip was not forced ahead of Developer for an inference test. Its lower priority is intentional;
Chip's eligibility is established by its verified provider identity, successful provider and Home
quota authentication, enabled/available state, and exact scope binding shared by all six keys.

To undo only CPA sharing, reacquire all canonical fleet locks and delete
`/v8/management/channel-group-details/11`. Do not restore a stale database over newer rotating
OAuth authorizations.

To undo only the CodexBar addition, pause its sync LaunchAgent, remove the Chip account and
its exact helper mapping, then resume the sync. Avoid restoring whole old configurations over
newer settings or current tokens.

## Adding another shared account yourself

1. Add and enable the account under **Upstream → Account Credentials**; set its priority above
   Microsoft as appropriate. Higher priority numbers are preferred.
2. Open **Users & Access → Credential scopes**.
3. Under **Fleet shared — Claude, Codex, Antigravity, xAI**, choose **Add binding**, search for
   the email, select the account, and confirm **Add binding**.

Current users' keys already include this scope. For a future user, include the shared scope
on their key too. Keep Microsoft and Copilot in the private scope. Adding an account to Home
does not itself create a local CodexBar account: the local token account and pinned sync
mapping must also be added.
