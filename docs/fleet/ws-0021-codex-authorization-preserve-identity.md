# Replace Codex authorization without replacing the credential identity

Status: done — original UUID retained, fresh authorization verified, duplicate removed; 2026-10-13 23:06 UTC.

The user authorized moving a newly created Codex login onto the original Home entry, keeping
its settings, history and permissions, and removing the duplicate.

## Identity and scope

- Original credential: `acct-126999`, created September 8.
- New duplicate: a second Home credential for the same account (since deleted), created October 13.
- Both are Codex credentials for the same email, ChatGPT account ID and OAuth subject. These
  values were compared inside the Ark; token contents are never printed or saved in this repo.
- The original is attached to shared channel group 1 by binding 3. The duplicate is unassigned.
- Home's Add credential login creates a new UUID. It does not renew an existing credential
  merely because its email matches.

## Identity-preserving operation

All four canonical fleet locks are acquired and renewed. Before any credential write, a
detached SQLite `VACUUM INTO` backup is verified by integrity check, size and SHA256.

The duplicate is disabled to prevent it from refreshing the same authorization after transfer.
Only `access_token`, `refresh_token`, `id_token`, `expired` and `last_refresh` are transferred
through `PATCH /v8/management/credentials/fields`, with the original UUID as the selector.
The original UUID, account ID, filename, routing attributes, priority, proxy, WebSocket setting,
disabled setting, concurrency policy and channel/user permissions are retained.

This uses Home's database-backed Management API. No auth-directory runtime files are edited,
no token fields leave the Ark, and no Home or node service needs restarting.

The metadata patch preserves provider/type identity and therefore does not reset Home's quota
identity generation. Original usage history is checked with a stable pre-change maximum usage
ID. Counts, ID bounds and token totals for those historical rows must remain identical.

The duplicate is deleted only after native requests succeed with every client key, Home
attributes those requests to the original UUID, and settings/access fingerprints still match.
Home soft-deletes the duplicate; the original record and its history are never deleted.

## Future renewal

For a valid refresh token, use Home's `/credentials/refresh` operation against the existing
credential. When a browser login is needed, the current panel creates another entry. An
identity-preserving repair transfers the new authorization fields onto the original UUID
through `/credentials/fields` and then removes the unassigned duplicate. Uploading or editing
an entry under a different UUID does not preserve the original identity.

The original refresh-token family and the replacement family must never remain active in two
separate credential entries. Home remains the refresh owner.

## Retained model errors

The first transfer produced HTTP 200 completed native `gpt-6.1-sol` responses for all six
client keys, attributed by Home to the original UUID. Settings/access fingerprints and
pre-existing usage rows matched. Final acceptance still found the original's global
`status=error`, so the operation was rolled back before investigation; fresh authorization
was retained in the duplicate.

A read-only inspection found three older token-expired 401 errors in `gpt-5.6-sol`,
`gpt-5.6-terra` and `gpt-6-astra`; `gpt-6.1-sol` was already active. The field patch correctly
preserves runtime state. In `internal/cliproxy/auth/result.go:214`, success resets only the
requested model. The global error badge clears only when `hasModelError` finds no remaining
model error (`result.go:1039`). Therefore a successful request on one model cannot clear an
older authentication failure stored on a different model.

The successful retry included actual native requests for all three models. This let Home clear each
error through its normal success transition, without fabricating results, resetting genuine
quota, replacing the original auth record, or deleting its historical usage.

## Verification and rollback

Acceptance includes native Codex Responses completions with all six client keys, usage
attribution to the original UUID, unchanged settings and access fingerprints, unchanged
original historical rows, exactly one active Codex entry, and all five nodes healthy with
successful required plugin reports.

On failed acceptance, restore the original authorization metadata and retain the freshest
replacement token pair in the duplicate before reenabling it. Do not restore a whole stale
database over newer rotating authorizations. Private rollback snapshots stay on the Ark with
mode 0600.

Private operation evidence: `/private/tmp/cpa-codex-reauth-preserve.*` on the Mac and
`/mnt/user/appdata/cpa-home-build/codex-identity-preserving-reauth/` on the Ark.

## Completed acceptance

- Completed at `2026-10-13T23:06:12.594877Z`: exactly one enabled, available, **active**
  Codex entry remains, under the original UUID and September 8 creation date. Its existing
  filename, WebSocket setting, metadata settings, routing attributes, channel bindings and
  client-key permissions matched the pre-change fingerprint.
- Original bounded history is identical: **98,290** usage rows, IDs 5–346990,
  **6,692,695,013** total tokens. The duplicate had zero usage before removal. New verification
  requests append to the original entry; none of its historical rows are recreated or deleted.
- All six client keys completed native `gpt-6.1-sol` requests with HTTP 200. Home records
  347183–347188 attribute these requests to the original credential UUID and the six keys.
- `gpt-5.6-sol`, `gpt-5.6-terra` and `gpt-6-astra` also completed with HTTP 200, attributed to
  the original UUID in records 347189, 347190 and 347192. Their previous authentication errors
  cleared normally; the entry's global error message is now empty.
- A quota collection completed at `2026-10-13T23:06:43.371591584Z`. Final readback shows
  `success`, `fresh`, `healthy`, plan `promax`, and no collection error. The transferred token
  expiration is October 23 at 5:46 PM CDT; expiration of an access token alone is handled by
  Home's refresh owner while its refresh authorization remains valid.
- All five nodes remain healthy with required plugin reports verified. No Home image,
  node binary, service, API key, access policy, or application source changed. All four locks
  were released.

### Backup evidence

The successful retry used:

- `/mnt/user/appdata/cpa-home/data/backups/home-pre-codex-reauth-20261013T230051Z.db`
- Size: `5,567,258,624` bytes; integrity: `ok`; permissions: `0600`.
- SHA256: `78357e4b64931336e4ecceeec65174552d2041f80541533be2d9a916ab42bef3`.

The first, rolled-back attempt retained its own verified backup:

- `/mnt/user/appdata/cpa-home/data/backups/home-pre-codex-reauth-20261013T225211Z.db`
- Size: `5,565,222,912` bytes; integrity: `ok`; permissions: `0600`.
- SHA256: `df97687a7c8a190dea9756a447a6802c4703d54ff17ce656a956b00ec4f15387`.

Private final evidence on the Mac: `/private/tmp/cpa-codex-reauth-preserved-result.json` and
`/private/tmp/cpa-codex-reauth-final-readback.json`. Neither file contains token values.
Private authorization rollback snapshots remain only on the Ark under the operation
folder with mode 0600; do not upload them or restore them over newer rotating tokens.
