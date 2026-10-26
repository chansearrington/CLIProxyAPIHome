# Third Codex subscription: HypeSports

Status: complete — Gmail 3/HypeSports 2/Hotmail 1; shared access, six keys, five nodes and three-account CodexBar sync verified; maintenance restored and locks released.

The user requested the newly added Codex subscription between Gmail and Hotmail everywhere
the fleet uses CPA. The existing Home entry is `acct-f4ba26` (label of its credential UUID), created October 16. It initially had no priority
or channel binding. Higher numeric priorities run first; the requested order is:

| Account | Priority |
| --- | --- |
| `acct-90c281` | 3 |
| `acct-f4ba26` | 2 |
| `acct-126999` | 1 |

HypeSports is bound to shared group 1 by binding **24**, available to all six existing client keys.
Home distributes dispatchable credentials to the five nodes centrally; their native
logins and configuration do not need account copies. Preserve existing UUIDs/history,
keys, permissions, Claude priorities and private-provider fences.

## Guards and ownership

Home's fleet policy was saved privately and paused with no active task. All four canonical
fleet locks returned `acquired:` and renew independently. Home has automatically completed
its official CPA 8.0.31 rollout on all five nodes, with successful upgrade receipts and no
active rollout. There is no laptop fleet coordinator.

Home changes use only the database-backed Management API, after a detached `VACUUM INTO`
backup passes full integrity and SHA256 verification. Private evidence stays under
`/mnt/user/appdata/cpa-home-build/ws0032/` and `/tmp/cpa-third-codex/`.

Verified backup: `/mnt/user/appdata/cpa-home/data/backups/home-pre-third-codex-20261016T074503Z.db`,
7,171,919,872 bytes, mode 0600, integrity `ok`, SHA256
`60116cc7f965a89c66cd8ab1ac09dcc0f6a57bdff68c741bbfdf0d42c8e22d67`.
The SSH output session disconnected after the gated operation; a fresh API read and private
receipts confirmed the successful changes before continuing, without replaying mutations.

## CodexBar

Added the exact Home UUID/email to the existing Codex sync settings, between the previous
two mappings, with private profile home `~/.local/share/cpa-codexbar-codex/hypesports`.
The installed helper verified token identity/expiry and provider quota before publishing
access and identity tokens. Home retains sole refresh ownership; the refresh-field value
in the monitoring profile is the existing inert marker.

Installed CodexBar returned all three Codex emails with successful OAuth quota reads.
The shared sync lock protected settings publication; config changes use the existing
config lock and atomic compare-and-swap. Claude provider configuration and unrelated
top-level settings compare identically. All ten Claude accounts remain present: seven
quota reads succeeded and three received Anthropic provider rate limits. No Claude
credential repair or reauthorization was performed. All three Codex profiles are mode
0700 with auth files 0600. Native `~/.codex/auth.json` was absent and remains absent.

## Acceptance and rollback

All six original client keys completed native `gpt-6.1-sol` Responses with HTTP 200 and the
requested reply. Home attributed successful Gmail requests to key IDs 1–6 in records
527802, 527807, 527811, 527813, 527818 and 527835 respectively.

To prove the secondary without changing production order or original key scopes, created a
temporary channel group containing only HypeSports and a random temporary client key. Native
Responses completed successfully through every node's loopback listener, with exact Home
attribution to the new credential:

| Node | Home usage record |
| --- | --- |
| MacBook | 527822 |
| Moxy | 527827 |
| Hyper | 527826 |
| Lara | 527825 |
| Chip | 527824 |

The temporary key, binding and group were deleted after proof. All six original key records,
all previous channel groups/bindings, model-group details and full inference-config fingerprint
match the pre-change baseline. All 16 credential identities/settings match apart from the two
requested priority updates. Five node memberships remain healthy with required plugins
`reported_ok`. Home's new-account quota collection succeeded at 07:55:51 UTC, fresh/healthy,
with zero collection failures. No image, binary, native login or node configuration changed.

The existing 120-second CodexBar LaunchAgent independently completed a successful three-account
sync at 07:48:02 UTC with last exit code 0, following actual Mac/key-6 Gmail usage. The helper's
initial publication and installed CodexBar OAuth reads passed for all three emails.
Home's full original maintenance policy was restored. All four canonical locks were released
and independently confirmed unlocked.

If Home acceptance fails, restore Gmail priority 2, remove only the new shared binding,
and return the new account's priority to unset. Preserve the original account UUID and
latest rotating tokens. Do not restore a stale whole database. Local rollback removes
only the HypeSports Codex mapping/profile path and restores the previous selected source
if necessary, under existing locks. Preserve current Claude tokens/settings.
