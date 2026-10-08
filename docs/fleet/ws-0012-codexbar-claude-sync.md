# Claude access-token sync into CodexBar

Status: installed and verified on the MacBook Pro, 2026-10-08 UTC.

Chanse authorized trying Home-supplied tokens for CodexBar's Hype and Gmail Claude accounts.
The Codex account is outside this change. This replaces the two Claude web cookies with
OAuth access tokens; it does not share refresh-token ownership.

## Implementation

The helper is maintained at `tools/codexbar-claude-sync/sync.py`. A user LaunchAgent runs it
at login and every 120 seconds. An online run reads Home over the Mac's existing Ark SSH
access. The remote process uses the Ark's management password and v8 credential-download API,
then projects only access token, expiry, email, and pinned credential ID into the SSH response.
Neither refresh tokens nor the management password leave the Ark.

The exact bindings are:

| CodexBar label | CodexBar account ID | Home credential ID | Verified provider email |
| --- | --- | --- | --- |
| Hype | `C69AAD18-BC24-4086-9BE6-68D19ADBE049` | `24e5779d-cd29-4f9f-89c7-45a577e9d70e` | `chanse@hypesports.live` |
| Gmail | `93B30590-2F05-4BA2-86B5-CD69538EF3E2` | `d95b7988-e30a-446b-8b8b-61dabed87d2e` | `chanse.arrington@gmail.com` |

Every changed token is independently verified against Anthropic's OAuth profile before an
atomic mode-0600 config update. Only those two token fields change. The global Web/Manual
settings remain because CodexBar 0.60.3 chooses OAuth per token account; the running app and
CLI both confirmed this behavior. Gmail remains the selected account.

Installed local paths:

- Helper: `~/.local/lib/cpa-codexbar-sync/sync.py`.
- Settings: `~/.config/cpa-codexbar-sync/settings.json` (IDs/paths only, no tokens).
- LaunchAgent: `~/Library/LaunchAgents/com.chansearrington.cpa-codexbar-claude-sync.plist`.
- Private status and rollback: `~/.local/state/cpa-codexbar-sync/`.

The rollback file contains the two original cookies and stays private on the Mac. Status
contains labels, emails, expiry times, and success/error information; no credentials.
Home, its database/config, all five nodes, and native Claude/Codex credentials are unchanged.
No Home or node deployment, restart, refresh request, or fleet lock was needed.

## Verification

- Anthropic's live profile endpoint independently matched both exact emails.
- A private staged config succeeded with `codexbar usage --provider claude --all-accounts
  --format json` before touching the live config. Both sources reported `oauth`.
- Live CLI verification after installation exited 0: Hype weekly 100% used, Gmail weekly 9%
  used (Gmail session 32% at 05:18 UTC); neither account returned an error.
- The running CodexBar settings UI showed `Source: oauth` and the expected weekly usage for
  each account. Its file watcher reloaded the update without an app restart. Gmail was
  reselected after checking Hype.
- The sync succeeded with `SSH_AUTH_SOCK` removed, confirming it does not depend on an
  interactive shell's SSH agent. LaunchAgent's initial run exited 0 with no token changes.
- Nine focused Python tests passed on the Ark with fake credentials. They cover replacement on
  rotation, original rollback retention, unmanaged setting preservation, rejected expiry and
  identity, refresh-token field rejection, remote secret filtering, profile failure, concurrent
  config edits, no-op behavior, and the authentication-failure latch.

No forced Home refresh was performed. The first future automatic provider rotation has not
yet been observed; controlled rotation tests cover the replacement path. The helper cannot
repair an invalid Home refresh token. Any unavailable/expired account or verification failure
leaves the current CodexBar config unchanged, and status records the failure.

## Disable or roll back

Unload the user LaunchAgent:

```sh
launchctl bootout "gui/$(id -u)" "$HOME/Library/LaunchAgents/com.chansearrington.cpa-codexbar-claude-sync.plist"
```

Then restore only the original managed token fields:

```sh
/usr/bin/python3 -B "$HOME/.local/lib/cpa-codexbar-sync/sync.py" --settings "$HOME/.config/cpa-codexbar-sync/settings.json" --restore
```

Other CodexBar settings are retained. The old cookies may expire, so rollback does not promise
that their website sessions will still be valid. Remove the LaunchAgent plist to keep it disabled
across logins. Full details and a non-secret settings template are in the helper's README.
