# Home → CodexBar Claude access-token sync

This optional Mac helper keeps existing Claude token accounts in CodexBar supplied with
Home's current access tokens. Home remains the only refresh-token owner. The helper never
calls an OAuth refresh endpoint and never changes Home, node routing, or native CLI logins.

CodexBar 0.60.3 recognizes `sk-ant-oat…` token-account values as OAuth credentials even when
the global Claude source is Web/Manual. It watches its config for external changes. Both
behaviors are implemented in the pinned upstream
[provider descriptor](https://github.com/steipete/CodexBar/blob/v0.60.3/Sources/CodexBarCore/Providers/Claude/ClaudeProviderDescriptor.swift)
and [config persistence](https://github.com/steipete/CodexBar/blob/v0.60.3/Sources/CodexBar/SettingsStore%2BConfigPersistence.swift).

## How it works

1. Over existing SSH access, run a read-only Python program on Home's host. It reads the
   management password there and downloads only the pinned credential IDs from the v8 API.
2. On that host, project each credential to ID, email, expiry, and **access token only**.
   Refresh tokens and the management password never enter the SSH response.
3. Reject missing accounts, unexpected fields, identity mismatches, and tokens with less than
   60 seconds remaining. Verify every changed token's email against Anthropic's OAuth profile.
4. Replace only the mapped CodexBar accounts' `token` fields using a mode-0600 atomic write.
   Preserve other providers, account IDs, labels, selected accounts, and settings.
5. Keep a private copy of the original managed tokens for rollback. Unchanged tokens cause
   no profile request or config write. Home/SSH/provider failures leave the config alone.

An authentication failure from Home latches `auth-blocked`, preventing repeated bad-key
requests and the management API's failed-auth ban. Fix the Ark-side key, then remove that
marker to resume. Other failures retry on the next scheduled run.

## Setup

Requires Python 3.9+, SSH with a noninteractive working key and trusted host entry, and existing
Claude token accounts in CodexBar. Create a private settings file using `settings.example.json`.
Pin the exact Home credential UUID, CodexBar account UUID, and provider email for each account;
do not infer identity from the label. `management_url` must be the host's local Home address.

Install `sync.py` outside the checkout, for example at
`~/.local/lib/cpa-codexbar-sync/sync.py`. Use mode 0700 for the installation and settings
directories and 0600 for settings. Do not store credentials in the settings file.

First run `sync.py --settings /absolute/path/settings.json --check`. To test CodexBar without
changing its live file, run `--stage /private/path/staged-config.json`, then:

```sh
CODEXBAR_CONFIG=/private/path/staged-config.json codexbar usage --provider claude --all-accounts --format json
```

Run without `--check` or `--stage` to sync. A user LaunchAgent can invoke `/usr/bin/python3 -B`
with the installed script and settings path, using `RunAtLoad=true`, `StartInterval=120`, and
`ProcessType=Background`. This catches up after login and after waking/reconnecting, normally
within two minutes of an online scheduled run. No service on Home needs installing or restarting.

`state_dir/status.json` contains non-secret status and expiry information. Keep the state
directory private: `original-tokens.json` is the rollback copy of the old cookies/tokens.
Avoid logging SSH responses or dumping CodexBar's config.

## Rollback

Unload the LaunchAgent first, then run `sync.py --settings /absolute/path/settings.json --restore`.
Restore changes only the managed token fields, retaining unrelated settings edited since setup.
The restored web cookies may have expired in the meantime. Remove the installed helper,
LaunchAgent, settings, and private state directory when no longer needed.

## Verification

`test_sync.py` uses fake credentials to cover rotation, secret filtering, identity and expiry
validation, concurrent edits, unchanged-token behavior, profile failure, preservation, and rollback.
Run `python3 -m unittest -v` from this directory in the permitted test environment. Live validation
must separately confirm both provider identities and successful CodexBar OAuth quota reads.

This helper polls Home; it cannot renew an expired/revoked Home refresh token. If Home needs
reauthorization, complete that there. It also cannot prove a future provider token rotation until
one occurs; the controlled rotation test exercises replacement with a new access-token value.
