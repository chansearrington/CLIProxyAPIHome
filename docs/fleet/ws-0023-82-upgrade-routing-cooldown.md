# Restore account 82 routing after its Max 20x upgrade

Status: resolved and verified, 2026-10-14 11:16 UTC — 82 dispatchable and preferred over Microsoft; existing settings/access preserved. All four fleet locks released.

The user observed Microsoft serving requests after upgrading account 82 and requested
verification and repair. Home's ledger confirms successful Microsoft Sonnet/Opus requests
from the Mac node at 10:59 UTC. CodexBar's automatic selection independently followed
Microsoft's latest successful request.

## Confirmed cause

Account 82 remains enabled at priority **8**, with shared binding **15** to group **1**.
Microsoft remains priority **1**, private to key 6. Higher numeric priority goes first:
`internal/cliproxy/auth/selector.go` selects the greatest eligible priority bucket, after
availability filtering. The priorities are not reversed.

82's persisted Sonnet, Opus and other model states retained a previous Anthropic HTTP-429
cooldown through **15:20 UTC / 10:20 AM CDT**. The Sonnet model state was last updated at
10:34:45 UTC. Meanwhile, the successful account probe at 10:51:27 UTC reported an active
Max 20x plan and 0% session/weekly usage. Plan/quota snapshots and execution cooldowns are
separate: `internal/quota/collector.go` persists the snapshot without clearing execution
cooldowns. The badge update therefore did not make the existing blocked model states
dispatchable.

An authenticated Messages request through Home's Management API, explicitly using 82's
credential, returned HTTP 200, model `claude-sonnet-5-5`, `end_turn`, and `ok`. This establishes
current provider inference availability independently of the quota display.

## Repair and verification plan

Acquire and renew all four canonical fleet locks; take a fresh detached SQLite backup with
integrity, size, permissions and SHA256 verified. Use the existing credential cooldown-reset
API, which atomically removes only Home-owned HTTP-429 quota cooldowns. Preserve the original
UUID, tokens, priority, account settings, existing access bindings and usage history.

Then verify real requests through the Mac node for all six existing keys. Gmail priority 10
reset at 11:00 UTC and has already resumed normal requests; those requests should prefer
Gmail while it is available. If necessary, use a short-lived private verification scope/key
with only 82 and Microsoft to prove 82 is selected over Microsoft without changing any
existing user's priorities or scopes. Revoke the verification key and remove its scope
afterward. Any new provider 429 must establish a genuine new cooldown rather than be ignored.

No Home image, node executable or global cooling policy change was made.

## Observed recovery and completed verification

During the investigation, 82's persisted model states received a quota-reset marker at
`2026-10-14T11:00:54.128100132Z`, before the verification scope was created and before this
driver called any cooldown-reset endpoint. The operator/process responsible was not
independently identified. Do not attribute this reset to the badge collector, the subscription
upgrade, or this repair driver. The driver did **not** need to call the reset endpoint.

The first scope test expected the old blocked state but observed successful 82 routing.
Its temporary key/scope were immediately removed before inspecting the newer state and
repeating verification against the actual recovered state. All eight previously cooled
82 model states are now active, with quota-exceeded false and zero retry deadlines.

- Independent provider inference checks: both Sonnet 5.5 and Opus 5.5 returned HTTP 200.
- A temporary scope containing **only 82 and Microsoft**, with a private key owned/restricted
  like Chanse's key, selected **82** for both native models through the Mac node. First ledger
  examples: **404318** (Sonnet), **404321** (Opus). A second scope check and repeat check did
  the same; repeat examples: **404687** (Sonnet), **404692** (Opus). This proves the priority
  comparison without modifying any existing key, shared scope, or account priority.
- All six original client keys completed native Sonnet requests with HTTP 200 and
  `message_stop`. Ledger records **404693**, **404697**, **404699**, **404695**, **404696** and
  **404698** respectively select **Gmail**. Gmail's priority 10 is ahead of 82's 8, and Gmail
  had resumed after its 11:00 UTC session reset. None of these requests used Microsoft.
- The original credential/settings/access fingerprint matched after cleanup. The bounded
  pre-verification usage history also matched exactly in count, min/max IDs and total tokens.
  Both temporary keys/scopes were revoked/deleted; only original key IDs 1–6 and group IDs
  1–2 remain active. The original shared 82 binding 15 remains intact.
- All five CPA nodes are healthy with required plugins `reported_ok`.
- CodexBar's sync at 11:14:41 UTC reports healthy automatic following of **Gmail**, matching
  the latest normal Mac/key-6 Claude request. It should follow 82 when 82 next serves that
  client's request; it should not be forced to display an account unused by the latest request.

Fresh detached backup before verification writes:
`/mnt/user/appdata/cpa-home/data/backups/home-pre-82-upgrade-cooldown-20261014T110041Z.db`;
**6,049,013,760 bytes**, integrity `ok`, mode `0600`, SHA256
`913ea93fb3a5c0cdb65637e856193e0a3d98fd62dccf7796be4b99be1464ea88`.
The backup was not restored, and no old token state was written back. Private sanitized
evidence is in `/private/tmp/cpa-82-*`; the verified backup manifest is under
`/mnt/user/appdata/cpa-home-build/ws0023/deploy/20261014T110041Z/`.

For future upgrades, confirm active subscription, fresh upstream quota **and actual successful
inference** before clearing an older Home quota cooldown. The supported targeted operation is
`DELETE /v8/management/credentials/<existing-uuid>/cooldown`; it preserves identity and access,
and does not reset Anthropic's quota. Do not turn off cooling globally or recreate the account.
