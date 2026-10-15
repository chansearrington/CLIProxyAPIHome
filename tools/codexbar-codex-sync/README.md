# Home Codex access tokens in CodexBar

`sync.py` publishes access tokens and identity tokens from explicitly pinned Home Codex
credentials into isolated CodexBar profile homes. Home keeps every refresh token.
It uses the shared SSH, atomic-write and usage-projection utilities in
`../codexbar-claude-sync/sync.py`.

CodexBar 0.75 requires a nonempty `tokens.refresh_token` field to recognize native auth.
The helper writes the literal **`cpa-home-owned-no-local-refresh`**, which is an inert marker,
never a provider credential. No Codex CLI should use these monitoring homes. Expired access
must be renewed by Home; local reauthentication is not part of this workflow.

Settings extend the Claude helper's nonsecret schema:

```json
{
  "ssh_host": "home-server",
  "env_file": "/private/home/.env",
  "management_url": "http://127.0.0.1:8327/v8/management",
  "codexbar_config": "/Users/example/.config/codexbar/config.json",
  "state_dir": "/Users/example/.local/state/cpa-codexbar-sync",
  "codex_state_dir": "/Users/example/.local/state/cpa-codexbar-codex-sync",
  "codex_system_home_id": "original-home-credential-uuid",
  "follow_usage": {
    "client_key_id": 6,
    "cpa_node_id": "macbook-node-uuid",
    "max_age_hours": 24
  },
  "codex_accounts": [
    {
      "home_id": "new-home-credential-uuid",
      "email": "example@example.com",
      "profile_home": "/Users/example/.local/share/cpa-codexbar-codex/example"
    }
  ]
}
```

An existing saved CodexBar account can supply `codexbar_id` and its private
`managedHomePath` as `profile_home`. Its identity and metadata stay intact, while its
monitoring auth receives Home's access-only projection. Back up that auth file privately
before first use. Unmapped system/native Codex auth is never overwritten.

Every changed credential is checked for pinned owner/workspace identity, future JWT expiry,
and successful provider usage before publishing. Failed reads leave existing tokens intact;
management 401/403 latches further polling in `auth-blocked`. Resolve management authentication
before removing that file. Private status contains identities, expiry and usage IDs only.
Files are mode 0600, profile homes 0700. Config updates share the Claude sync lock and
CodexBar's config lock, preserve other settings, and check for concurrent edits.

A user LaunchAgent can run the helper every 120 seconds. It also follows the latest successful
Mac/key-scoped Codex usage, selecting its mapped profile without changing CPA priorities or
native Codex login. CodexBar presents the followed account first; CPA Home remains responsible
for routing and numeric priority.

To disable, unload the dedicated LaunchAgent. Remove only the added profile paths and restore
its previous active source in CodexBar config. Restore any changed existing managed auth from
the private pre-install backup only after stopping this sync. Never restore a stale whole
config over normally rotating Claude tokens.

Validation: `python3 -B tools/codexbar-codex-sync/test_sync.py`. Live acceptance additionally
uses CodexBar's installed `usage --provider codex --all-accounts --source oauth --format json`.
