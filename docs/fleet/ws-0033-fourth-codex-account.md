# Fourth Codex subscription: Developer

Status: complete — Gmail 4/HypeSports 3/Developer 2/Hotmail 1; shared access, six keys, five nodes, fresh quota and four-account CodexBar sync verified; maintenance restored and locks released.

The user added another Codex account and requested shared setup, correct priority and CodexBar.
The new Home entry is `acct-fdfbfe` (label of its credential UUID), created October 18. It is enabled/active but initially
has no priority or channel binding. Preserve Gmail/HypeSports relative order and Hotmail as
last fallback; insert Developer immediately above Hotmail. Higher numeric priorities run first:

| Account | Priority |
| --- | --- |
| `acct-90c281` | 4 |
| `acct-f4ba26` | 3 |
| `acct-fdfbfe` | 2 |
| `acct-126999` | 1 |

Developer received shared channel group 1 through binding **26**, available to all six existing client keys and
five nodes through Home dispatch. Preserve native logins, original credential UUIDs/history,
client keys/scopes, private provider fences and all Claude settings/priorities.

## Guards and ownership

Saved Home's full maintenance policy, paused it and confirmed no active task. All four
canonical locks returned `acquired:` and renew independently. Home has automatically completed
the CPA 8.0.32 rollout on all five nodes; memberships and required plugin reports are healthy.
No laptop fleet coordinator is installed or needed.

A detached SQLite `VACUUM INTO` backup must pass full integrity and SHA256 verification
before any credential/access write. Private evidence is stored under
`/mnt/user/appdata/cpa-home-build/ws0033/` and `/tmp/cpa-fourth-codex/`.
After VACUUM committed and closed the source database, full integrity verification resumed
against the completed copy with a bounded 256 MiB read cache (`verify-backup.py`). Only the
offline verifier's cache changed; Home's live SQLite connection/settings are unchanged.

Verified backup: `/mnt/user/appdata/cpa-home/data/backups/home-pre-fourth-codex-20261018T055301Z.db`;
8,452,292,608 bytes, mode 0600, full integrity `ok`, SHA256
`2e0033a1e6be992e9652c63d38532587898e3327b3a0c923fb3bc9da95dca04f`.

## CodexBar

Added the exact Home UUID/email to the existing pinned Codex mappings with private profile
home `~/.local/share/cpa-codexbar-codex/developer`. The installed helper validated token
owner/workspace, future expiry and real provider usage before publishing access and identity
tokens. Home keeps sole refresh ownership; only the established inert refresh-field marker
is present locally. All four profiles are mode 0700, with auth files 0600.

Installed CodexBar returns all four emails with successful OAuth quota reads. Other providers,
top-level settings and the ten Claude account mappings/settings compare identically.
Native Codex auth existence/content matches the private baseline. The existing 120-second
sync/follow LaunchAgent is retained; account following remains scoped to successful Mac/key-6
Codex usage. CodexBar monitoring order does not control Home dispatch priority.

## Acceptance and rollback

All six original client keys completed native `gpt-6.1-sol` Responses with HTTP 200 and
the exact requested reply. Home recorded successful Gmail usage for keys 1–6 in records
674244, 674245, 674248, 674251, 674253 and 674254 respectively.

An isolated temporary group containing only Developer and a random temporary key proved
the new account on every loopback node without changing production preference or existing
client scopes. Exact new-credential attribution:

| Node | Home usage record |
| --- | --- |
| MacBook | 674258 |
| Moxy | 674261 |
| Hyper | 674260 |
| Lara | 674262 |
| Chip | 674263 |

The temporary key/group/binding were deleted after proof. The six original key records,
all previous groups/bindings, model-group details and full inference-config fingerprint
match their baseline. All 17 credential identities/settings match apart from the three
requested priority updates. Five memberships and required plugin reports remain healthy.
The new credential is active; its Home quota probe at 06:01:41 UTC is fresh/healthy with
zero collection failures. No image, node binary, native login or node configuration changed.

The retained 120-second LaunchAgent independently completed four-account sync at 05:57:45 UTC
with last exit code 0. Installed CodexBar's OAuth reads succeeded for all four emails.
Home's complete original maintenance policy was restored; all four canonical locks were
released and independently confirmed unlocked.

Rollback reverses only this operation: Gmail priority 3, HypeSports 2, Developer unset;
delete only the newly created shared binding and temporary verification key/group/binding.
Retain all credential UUIDs/history and latest rotating tokens. Never restore a stale whole
database over live authorizations. Local rollback removes only Developer's mapping/profile path,
preserving current tokens, Claude accounts and unrelated settings under existing locks.
