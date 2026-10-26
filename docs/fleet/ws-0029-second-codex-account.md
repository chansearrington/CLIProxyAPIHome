# Second Codex subscription and CodexBar

Status: complete — Gmail priority 2/shared, Hotmail priority 1; six-key and five-node native Codex proofs pass; both CodexBar accounts and scheduled sync pass.

The user added a second Codex account and requested priority 2 above the original account,
with both CPA Home and CodexBar updated.

- New: `acct-90c281` (label of its Home credential UUID).
- Original: `acct-126999` (label of its Home credential UUID).

Home's higher numeric priority runs first. Initially Gmail was 1 and Hotmail 2;
Gmail also had no channel binding. Requested state is Gmail 2, Hotmail 1,
with Gmail in the same shared channel group 1 as Hotmail. Existing identities,
keys, model scopes, WebSocket flags, cooldowns and other provider settings are preserved.

## Backup and live guards

All four canonical fleet locks acquired and heartbeating before Home writes.
Detached SQLite `VACUUM INTO` backup completed and independently passed full integrity:

- `/mnt/user/appdata/cpa-home/data/backups/home-pre-second-codex-20261015T192652Z.db`.
- Size 6,698,442,752 bytes; mode 0600; integrity `ok`.
- SHA256 `c72f05044c694a042391d6a30bbcc7d66e330308b1285acffa9591a1437df558`.

Priority changes use `PATCH /v8/management/credentials/fields`; shared access uses
`POST /v8/management/channel-group-details`. Failed Home acceptance restores only the
original priorities and deletes the newly created binding. No whole database restore,
Home/node restart, build, or token refresh is needed.

## CodexBar

CodexBar 0.75 had the original saved Hotmail account and followed the native system login.
Its saved monitoring auth had expired. New access-only helper:
`tools/codexbar-codex-sync/sync.py`, installed under `~/.local/lib/codexbar-codex-sync/`.
It projects access and identity tokens into a new Gmail profile home and the existing
saved Hotmail monitoring home. The native `~/.codex/auth.json` is untouched.
Home remains the sole refresh-token owner. The literal inert refresh-field marker exists
only because CodexBar rejects an empty field during parsing; no actual refresh token leaves Home.
Both changed tokens independently passed the provider usage API before publication.

Settings: `~/.config/cpa-codexbar-sync/codex-settings.json`.
LaunchAgent: `com.chansearrington.cpa-codexbar-codex-sync`, every 120 seconds.
Private status: `~/.local/state/cpa-codexbar-codex-sync/status.json`.
The existing ten Claude accounts and their sync helper/settings are preserved.
Codex selection follows successful MacBook/key-6 Home usage; the followed account sorts first.
Existing saved Hotmail identity stays intact. Gmail is a supported advanced profile-home entry.
The helper holds the shared sync lock and CodexBar config lock during config publication.

Staged and live installed CodexBar CLI returned both accounts with `source=oauth`, correct
emails, and successful quota reads. Native UI inspection is unavailable: the computer-use
server timed out before returning any app state, so no visual menu proof is claimed.

Private pre-install config and original managed Hotmail auth backup:
`/tmp/cpa-second-codex/` (directory 0700, credential files 0600).
Home operational evidence: `/mnt/user/appdata/cpa-home-build/ws0029/`.

## Completed acceptance

- Gmail priority **2**; Hotmail priority **1**; new shared channel binding **23**.
- All six original client keys completed native `gpt-6.1-sol` Responses with HTTP 200
  and the exact requested reply. Home attributes usage IDs 471057, 471063, 471068,
  471073, 471076 and 471113 to Gmail for keys 1–6 respectively.
- All four minis separately completed native Responses through their own loopback nodes.
  The MacBook completed the six-key proof. All five memberships remain healthy with
  required Copilot plugin reports `reported_ok`.
- Existing API-key entries compare identically. All credential identity, routing prefix,
  proxy, disabled, WebSocket and concurrency settings compare identically except the
  requested priorities; existing bindings are retained.
- Installed CodexBar CLI returns exactly both Codex accounts with OAuth usage and no
  errors. The dedicated LaunchAgent completed three runs with last exit code 0 by
  19:35 UTC and followed Mac/key-6 Gmail usage. Both access-token projections are
  unchanged on subsequent runs; no actual refresh token is stored in the monitoring homes.
- All **ten** Claude accounts still pass installed CodexBar usage checks; their identities,
  labels and ordering match the pre-change snapshot, and their original scheduled sync passes.
- Seven focused helper tests pass: valid identity, wrong owner/workspace, expiry,
  refresh-token rejection, access-only publication with unrelated setting preservation,
  and provider rejection leaving existing auth intact.

The first Home acceptance attempt used the MacBook's Tailscale address from Ark, but the
node intentionally binds only `127.0.0.1`. It timed out before connecting; the guard restored
original priorities and removed temporary binding 22. The operation was reapplied, and
actual inference acceptance ran locally through the node's existing loopback listener.
No node exposure or listener configuration changed.

No Home image, node binary, native Codex login, or Claude sync deployment changed.

All four canonical fleet locks released and independently confirmed **UNLOCKED**.
