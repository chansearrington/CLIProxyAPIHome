# CodexBar follows CPA's Claude account

Status: installed and verified on the MacBook, 2026-10-12 16:21 UTC.

Chanse wanted the single Claude percentage in the menu bar to follow the account CPA actually
uses, instead of requiring manual account selection.

## Behavior

The existing two-minute access-token sync now optionally selects the account from the latest
successful Claude request for the MacBook node and Chanse's key 6. It reads Home's Management
API with provider, success, key, node, age, and one-record filters, then independently validates
the exact returned scope and timestamp before resolving the CodexBar account's current index
by its pinned UUID. Selection can change even when no access token rotates.

Installed follow settings:

```json
{
  "client_key_id": 6,
  "cpa_node_id": "1e3ea42b-169e-474c-9c4e-4527c72654fe",
  "max_age_hours": 24
}
```

Another user's or node's requests cannot change the selection. Simultaneous sessions may use
different credentials; the latest successful completed request wins. The helper follows
observed inference rather than guessing the next scheduler choice. An idle, stale, unavailable,
or unmapped observation preserves the current account. Usage lookup failure does not prevent
otherwise valid token rotation. Manual selection is replaced on the next successful poll.

This remains a local Mac helper. No Home image, CPA executable, priorities, credential scopes,
keys, OAuth refresh ownership, or native CLI authorization changed. No fleet locks were
required for read-only Home access and local app configuration.

## Private Microsoft fallback

To follow all credentials this Mac can use, added **Microsoft** as the seventh local Claude
account after the six shared subscriptions. Home UUID `acct-63b680`,
CodexBar UUID `9D306A4E-A8B9-4093-A80C-9665E334A976`, verified email
`acct-63b680`. The existing private Home scope remains Chanse-only. Anthropic's
profile independently verified the exact identity, and CodexBar's staged all-seven-account
OAuth usage succeeded. Only the access token is copied; the refresh token stays on the Ark.

## Review and proof

- PR [#15](https://github.com/chansearrington/CLIProxyAPIHome/pull/15), reviewed at exact head
  `12a7c89ca7d1fd76c93a228c33e15cc17c3a9908`, with zero MUST/SHOULD/NICE findings and independent
  synchronous validation. Merged as `ab9e95f`.
- All **17 focused Python tests passed on the Ark**. Coverage includes no-token-change
  selection updates, reordered accounts, wrong key/node/provider, failed/stale/future usage,
  unavailable lookup with continued token rotation, staged/check-only behavior, concurrent
  settings edits, authentication failure latching, and remote secret filtering.
- Private staged live config selected **Moxy** from actual Home usage. CodexBar's default
  usage command returned Moxy OAuth usage; all seven accounts returned usage without errors.
- A second staged run started with Chip selected and **zero token changes**; it automatically
  selected Moxy, reproducing the user's missing behavior without altering production routing.
- Paused the LaunchAgent, quit CodexBar for the installation, preserved private backups,
  installed the reviewed helper/settings/account, and resumed the app and two-minute job.
  Installed helper SHA-256:
  `17ce7a6c87ed61a80e07114fc7abb198673b088a93b5919d98e3ce3375b6ca30`.
- The running app's saved selected account is Moxy, and default live CodexBar usage returned
  Moxy, OAuth, 31% session used and 8% weekly used at verification. Those figures change with
  traffic and the app retains its existing percentage-display preferences.
- The automatic LaunchAgent run at **16:21:27 UTC** succeeded, following the Mac/key-6 Moxy
  request timestamp **16:21:22 UTC**, with no unnecessary selection rewrite. The installed
  helper bytes match the reviewed source. No production account order was changed for tests.

The app's tagged config persistence implements external file watching and revision updates:
[CodexBar 0.60.3 persistence source](https://github.com/steipete/CodexBar/blob/v0.60.3/Sources/CodexBar/SettingsStore%2BConfigPersistence.swift).
Verification used saved selection and the default CLI usage result; no screen capture of the
menu bar was taken.

## Disable or undo

For manual account selection, remove `follow_usage` from
`~/.config/cpa-codexbar-sync/settings.json` or set it to `null`. Token sync continues normally.

Private mode-0600 pre-install files are retained in
`~/.local/state/cpa-codexbar-sync/ws0018/`: `config.pre-follow.json`,
`settings.pre-follow.json`, and `sync.pre-follow.py`. To remove only Microsoft, pause the
helper and remove its local account and exact mapping, then resume. Avoid restoring whole
old token configs over later settings or token rotations. Returning to the old helper also
requires pausing sync before replacing it and removing the follow setting.
