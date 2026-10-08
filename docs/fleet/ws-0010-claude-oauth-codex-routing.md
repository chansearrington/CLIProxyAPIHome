# Claude OAuth recovery and Codex CPA routing

Status: complete and verified, 2026-10-08 UTC. Gmail and HypeSports authentication restored,
Codex routes through CPA by default, and all fleet locks are released.

Home rejected Gmail and HypeSports because their stored Claude OAuth refresh tokens were
invalid, while the Gmail quota displayed in the panel was stale. The MacBook's Codex CLI
also had no CPA provider configured and was making inference requests directly to ChatGPT.
Its startup warning came from incompatible CLI and daemon feature settings.

## Claude credential findings

- The intended order, reconfirmed by Chanse, is HypeSports → Gmail → Microsoft as an emergency
  backup. The live priorities are respectively 3, 2, and 1. Home selects the highest available
  priority; the order itself is correct (`internal/cliproxy/auth/scheduler.go:983`).
- Both Home credentials had `disabled=true`, `status=disabled`, and a terminal refresh
  diagnostic: Anthropic HTTP 400, `invalid_grant`, `reason=token_expired`, “Refresh token
  expired.” Their metadata still had `disabled=false`, consistent with automatic authentication
  failure rather than a manual disable.
- Terminal refresh failures disable credentials (`internal/cliproxy/auth/result.go:1146`,
  `internal/cliproxy/auth/manager.go:1818`). Enabling an existing credential changes its status;
  it does not replace its expired OAuth tokens (`internal/cluster/management/oauth.go:525`).
  Retained events show enable → refresh claim → refresh failure, explaining the reverting toggle.
- Gmail's saved quota observation was from 2026-10-06 09:30:58 UTC, with a weekly reset of
  2026-10-07 01:00 UTC. Disabled credentials are excluded from collection
  (`internal/quota/collector.go:578`), so that observation could not establish current quota.
- Independent live web data from CodexBar showed Gmail at 0% five-hour and weekly usage,
  and HypeSports at 100% weekly usage. A fresh Gmail OAuth authorization independently verified
  its exact email through both token exchange and the provider profile endpoint, and its usage
  endpoint confirmed 0% for both windows, with a weekly reset of 2026-10-14 01:00 UTC.
- The native Claude Code Keychain login is HypeSports, has a different valid token pair, and
  successfully reads that account's provider profile and exhausted weekly usage. Copying that
  refresh token into Home would give two processes ownership of a rotating token; recovery uses
  a separate Home authorization instead.
- The repository's proposed quota sentinel is not deployed: no container, process, or sentinel
  appdata directory exists on the Ark. It did not cause these toggles.

Retained logs do not establish why the old refresh tokens became invalid. The retained Hype
failure shows attempt 1 only; older Gmail failure logs are unavailable. Neither shared token
rotation nor ambiguous refresh retries can be claimed as the historical cause.

## Claude recovery

The fresh authorization uses Home's existing Claude OAuth client, scopes, PKCE flow, and
`http://localhost:54545/callback`. Tokens and authorization codes are kept out of repository
files. The replacement targets the existing Gmail UUID and copies its existing routing metadata,
including priority and filename. Its channel-group associations remain attached to that UUID.
The native Keychain login is not copied or changed.

Home's normal Add credential login creates a new UUID; it does not match an existing email
(`internal/cluster/management/oauth_login.go`, `internal/cluster/management/oauth.go:328`).
An identity-preserving upload avoids creating a duplicate without the existing channel access.

The original Gmail UUID `d95b7988-e30a-446b-8b8b-61dabed87d2e` was restored on 2026-10-08
at approximately 03:49:04 UTC. Its priority 2, filename, all routing attributes, channel details,
and API-key channel groups are unchanged. There are still exactly three Claude credentials,
including exactly one Gmail credential. No native login or Home/node service was restarted.

Before upload, all four canonical fleet locks were held and a `VACUUM INTO` backup was verified:

- Ark path: `/mnt/user/appdata/cpa-home/data/backups/home-pre-gmail-reauth-20261008T034522Z.db`
- Size: 3,715,952,640 bytes; full `integrity_check`: `ok`.
- SHA-256: `b305576de87f66c887bf83828d8317ae2abe6a6fcd74fac4fe8d44b319e506b4`.

Each active API key, IDs 1–6, completed a native Sonnet streaming request with HTTP 200,
`message_stop`, and reply `ok`. The MacBook's actual `claude -p --model sonnet` also exited 0
with reply `ok` on `claude-sonnet-5-5`. Home's 03:49:17 UTC quota snapshot is fresh and healthy,
with 0% used / 100% remaining for the Gmail windows. At 03:51:36 UTC the credential remained
active, enabled, and available across multiple collector cycles; all 16 Claude usage records
since recovery were Gmail successes, with zero Microsoft use. All four locks were then released
and independently confirmed unlocked.

HypeSports was independently reauthorized after Chanse signed into that account. Its token
response and provider profile both verified the exact HypeSports email; live usage confirmed
100% weekly use. The interrupted browser navigation initially had `ttp://` in the redirect URI.
Correcting it to the supported `http://localhost:54545/callback` completed authorization normally.

Under reacquired locks, the existing HypeSports UUID and priority 3 were restored. The first
native Sonnet attempt hit the genuine HypeSports quota limit at 03:54:41.127 UTC, and the same
client request succeeded on Gmail at 03:54:41.587 UTC. Home then retained HypeSports as enabled,
with a quota cooldown until 2026-10-13 21:00 UTC (4 PM CDT), rather than authentication-disabled.
Its fresh 03:54:41 UTC quota snapshot correctly reports exhausted with successful collection.
All six API keys and the actual MacBook Claude Code command passed again through Gmail.
The first Opus request also encountered an upstream HypeSports 429 at 03:55:22 UTC before
succeeding on Gmail. The existing scheduler only propagates a credential-wide rejection to
already-existing model states (`internal/cliproxy/auth/result.go:334`), so a previously unseen
model can encounter its own initial rejection. Both Sonnet and Opus now have the real weekly
cooldown. These two upstream attempts were handled by fallback; client proofs still returned 200.
HypeSports routing attributes, filename, channel associations, and priority remain unchanged;
Gmail and Microsoft full auth records were unchanged by this second recovery. No native Keychain
credential was copied or changed.

A separate parent-agent read-only database check confirmed all three credentials enabled,
HypeSports cooling down until the real reset, Gmail active and available, and priorities 3 → 2 → 1.
At that check, all 45 successful Claude usage records since the Gmail recovery belonged to Gmail;
none belonged to Microsoft.
The final HypeSports observation at 03:56:46 UTC confirmed the cooldown persisted and Gmail
remained available. All consumed temporary PKCE state, callbacks, and staged new token payloads
were removed; the loopback listener was closed. All four fleet locks were released and their
unlocked status checked independently. Home and CPA node services were never restarted.

## Codex findings and recovery

- The original session `01a11992-4f82-75e0-a79e-2f085a56cc4e` used provider `openai`.
  Its SQLite diagnostic log records successful inference WebSockets to
  `wss://chatgpt.com/backend-api/codex/responses`; Home had zero usage records for its session,
  root-session, or parent-session ID. The local CPA node was nevertheless healthy and connected
  to Home. This establishes missing client routing independently of node health.
- The screenshot's older CLI was 0.160.1 talking to a 0.161.0 daemon. It required
  `api_key_model_discovery=false`. Homebrew switched the installed CLI to 0.161.0 at
  2026-10-07 22:29:30 CDT; that version requires the feature enabled. The daemon retained
  the opposite setting. A fresh current CLI reproduced the warning requiring `true`.
- Neither choosing standalone mode nor restarting the daemon creates a CPA provider.
  The original user configuration, shell startup files, and process had no CPA inference route.
- A real isolated Codex request through CPA succeeded with model `gpt-6.1-sol`. Home usage
  row 171058 records session `01a1199a-7da5-7453-9291-b529345fac21`, request
  `01a1199a-7dcc-73b9-bf16-945699b54d3b`, the MacBook node, 10,085 total tokens, and no failure.
- A subsequent probe using the existing user configuration and provider `auth.command`
  executed a real shell command and accessed the deferred tool registry. Diagnostic logs show
  completed tool calls; Home also recorded the requests. The existing secure key helper can
  provide the CPA key without putting it into Codex configuration or changing ChatGPT login.

The supported custom-provider settings are documented in the
[official configuration reference](https://learn.chatgpt.com/docs/config-file/config-reference).
CPA is now the default provider in `~/.codex/config.toml`, using
`http://127.0.0.1:18317/v1`, Responses transport, and the existing secure
`~/.claude/bin/cpa-home-key.sh` through `auth.command`. Model and reasoning settings, plugins,
and ChatGPT login were preserved. Chanse explicitly confirmed CPA should be the default for
clients sharing this configuration.

Before restarting the shared daemon, its documented `thread/loaded/list` RPC returned an empty
list with no next cursor: there was no loaded work to interrupt. The current CLI's supported
restart dialog persisted `api_key_model_discovery=true`, preserving the other shared settings.
A CLI `-c` override on the restart command alone did not override persisted daemon settings;
the dialog applied the actual persistent correction. No alias or daemon bypass remains.
A fresh bare `codex` launch attached to the aligned daemon with no warnings.

The final daemon proof session `01a119a1-f262-74b0-ab6a-7c03b49cbc52` reported provider `cpa`,
executed a shell read, and returned an unpredictable file marker exactly. Home rows 171131 and
171133, at 03:50:18 and 03:50:22 UTC, recorded successful `gpt-6.1-sol` requests through the
MacBook node, with request IDs `01a119a2-220c-75b4-a7d7-88223b6fdbd8` and
`01a119a2-31cd-7534-add8-93e7f2d3e054`. The proof thread was idle afterward.

Local rollback backups are mode 0600:

- `~/.codex/backups/config.toml.pre-cpa-20261008T034709Z`
- `~/.codex/app-server-daemon/settings.json.pre-cpa-20261008T034938Z`

The already-running original investigation session retains its direct provider. Start a new
`codex` session to use the corrected default; resuming a provider-pinned old session has not
been verified.

## Upstream audit and preventive patch

The latest verified releases are [Home 1.1.0](https://github.com/router-for-me/CLIProxyAPIHome/releases/tag/v1.1.0)
and [CPA 8.0.20](https://github.com/router-for-me/CLIProxyAPI/releases/tag/v8.0.20).
The live Home remains 1.0.73 with fleet changes, and nodes are 8.0.7.

Home's newer changes include [Codex configuration-update capability](https://github.com/router-for-me/CLIProxyAPIHome/commit/b72ff33d60c63c77aaa37f3f4871c3c242a310b1),
[V8 migration](https://github.com/router-for-me/CLIProxyAPIHome/pull/122), and
[upstream/client configuration layout](https://github.com/router-for-me/CLIProxyAPIHome/pull/125).
There are 12 upstream commits absent from fleet and 81 fleet commits absent from upstream.
A future rebase must preserve the fleet's routing, quota, and plugin changes; replacing the image
with the upstream release would discard them. No upstream update was established as the fix
for Codex's daemon warning.

CPA fixed unsafe replay of a single-use Claude refresh token in
[c163bae4](https://github.com/router-for-me/CLIProxyAPI/commit/c163bae4bcc12df5aa76a1a58ba3f26860be9393)
and stale concurrent refresh overwrites in
[935aa6e3](https://github.com/router-for-me/CLIProxyAPI/commit/935aa6e3eac58f6d422242a44ac0ecb7de211809).
Related issues: [6288](https://github.com/router-for-me/CLIProxyAPI/issues/6288),
[6417](https://github.com/router-for-me/CLIProxyAPI/issues/6417), and
[5611](https://github.com/router-for-me/CLIProxyAPI/issues/5611).
CPA 8.0.17's [Claude OAuth regression 6432](https://github.com/router-for-me/CLIProxyAPI/issues/6432)
was fixed in 8.0.18; a future node upgrade should include that fix.

Home's local Claude implementation still lacks the replay guard, including upstream Home 1.1.0.
The preventive port is committed locally as `643f6c48d04ba6956d3b82b84c2f8cefd92b9dd0`
on `fix/claude-ambiguous-refresh-retry`, in the sibling worktree
`CLIProxyAPIHome-claude-refresh-fix`. It retries only explicit retryable HTTP responses.
All six Claude package tests pass; restoring the original implementation makes all three new
transport/read/JSON regressions fail with three HTTP calls. CGO Home compilation passed.
Validation ran on the Ark in `golang:1.26-bookworm` with isolated source/output and all four
scoped fleet locks; the compile artifact was removed and locks released.

This preventive patch is not merged, pushed, or deployed. It prevents a verified defect, but
cannot recover a refresh token already rejected by Anthropic and is not a proven cause of this
incident. The existing error text “after 3 attempts” uses the configured maximum even when a
non-retryable error stops after one attempt; it is not evidence of three HTTP requests.
