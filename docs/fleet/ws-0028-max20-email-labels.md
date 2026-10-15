# Max 20x upgrades and CodexBar email labels

Status: complete — both Home badges show fresh Max 20x plans; all ten CodexBar email labels and usage/sync pass; all six client keys complete native Claude requests. All locks released and independently confirmed unlocked.

The user upgraded The Vanderblooms and Hyper Rally to Claude Max 20x and requested their
Home plan badges reflect that change. The user also requested email addresses as CodexBar
account names.

## Plan verification

Independent Anthropic OAuth profile reads through Home returned HTTP 200 for both original
credential IDs and the expected email addresses. Both report organization type `claude_max`
and rate-limit tier `default_claude_max_20x`:

- `c714d247-5142-477d-9036-3e8b3fc88dbd` — `the.vanderblooms@gmail.com`.
- `19dbc651-b64b-428c-9e74-e0517753e584` — `hyper.rally@outlook.com`.

Home's existing automatic plan collector already recognizes this tier; no code change or
manual badge override is needed. At the first check, both stored snapshots still read
Max 5x. Vanderblooms' snapshot was collected at 15:14 UTC; Hyper Rally's was stale from
01:21 UTC. On-demand collection refreshed the provider-backed snapshots after the
required fresh backup was verified. No execution cooldowns, priorities, permissions or
account identities were changed.

## CodexBar names — completed

Renamed all ten mapped Claude accounts in `~/.config/codexbar/config.json`, matching the
verified email mapping in `~/.config/cpa-codexbar-sync/settings.json`. Updated the sync
mapping's display labels to those same emails.

Only `label` values changed. Account UUIDs, account ordering, current selection, OAuth
tokens, other provider settings and sync ownership were preserved with atomic writes and
concurrent-change checks. Private pre-change backups are retained under
`/private/tmp/cpa-plan-email-20261015/` with restrictive permissions.

The installed CodexBar CLI returns usage successfully for all ten accounts, each labeled
with its email. The sync helper passes with all ten mappings and follows the latest
successful Mac/key-6 request as `the.vanderblooms@gmail.com`. A subsequent scheduled sync
also preserves the new names. CodexBar 0.75 has a config file watcher, so an app restart
was not needed.

No Home image, node binary, application version or refresh-token ownership changed.

## Guards

All four canonical fleet locks acquired and heartbeating. A detached `VACUUM INTO` backup
finished on Ark before the Home write:

- `/mnt/user/appdata/cpa-home/data/backups/home-pre-max20-labels-20261015T152606Z.db`.
- 6,584,360,960 bytes, mode 0600, integrity `ok`.
- SHA256 `273bf6fe7b4859e0138f4866d8841969d7b819f38e0d22e32d23c2b1c99fdf3a`.

`POST /quota/collect` accepted exactly the two requested credential IDs. Final snapshots:

| Account | Plan | Successful collection (UTC) |
| --- | --- | --- |
| `the.vanderblooms@gmail.com` | Max 20x, premium | 2026-10-15 15:30:55.893 |
| `hyper.rally@outlook.com` | Max 20x, premium | 2026-10-15 15:30:56.590 |

Both snapshots report fresh/success. Credential settings and stored priorities were
independently compared with their pre-change values and matched. All six current client
keys then completed native Sonnet requests with HTTP 200 and the requested marker.
There is no manually maintained plan label to update in future: Home derives it during
quota/profile collection.

Private evidence is retained under `/mnt/user/appdata/cpa-home-build/ws0028/` on Ark and
`/private/tmp/cpa-plan-email-20261015/` on the Mac. The settings/config backups contain
secrets and must never be committed. Only display labels changed in the local files.

Close-out: all five nodes remain healthy. All four locks released at 15:33:59–15:34:02 UTC
and independently checked unlocked. No Home code/image or node service changes were needed.
