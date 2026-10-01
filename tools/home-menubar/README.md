# Home Usage — menu bar item for CLIProxyAPIHome

A tiny macOS menu bar app (one Swift file, no dependencies) that shows the quota and usage of
every account this Home manages. Built for the fleet in ws-0008
(`docs/fleet/ws-0008-codexbar.md`), because CodexBar can only see the logins on the Mac it runs on.

## What it shows

- **Bar:** `CPA 92%` = the fullest Claude or Codex limit right now (the one most likely to slow
  work down). Plain under 80 %, orange from 80 %, red from 95 %. `CPA ⚠︎` = polling stopped
  (key rejected or missing); `?` after the number = Home was unreachable on the last try and the
  numbers are the last ones it got.
- **Dropdown:** each account Home holds (masked email, plan), every quota window with used %,
  bar and local reset time, the account's last-24-hour requests and tokens, and a note when the
  numbers are old ("not recently used" — Home only re-measures accounts that are in use), quota is
  low/exhausted, or Home has the account marked unavailable. The last line is the whole fleet's
  last-24-hour totals.
- **Refresh now** asks Home to re-measure the shown accounts with their providers (real provider
  calls), then re-reads after 15 s. The automatic refresh every 2 minutes only reads Home's
  database.

## Install / update

1. Put the Home management key in the login Keychain (service `cpa-home-management`, account
   `home-usage`). Never pass it as a command argument; feed `security -i` over stdin, e.g. from
   the Ark's `.env` (see the brief for the exact command used).
2. `tools/home-menubar/install.sh` — compiles with `swiftc`, ad-hoc signs, installs
   `~/Applications/Home Usage.app`, and starts it at login with the LaunchAgent
   `~/Library/LaunchAgents/com.chansearrington.home-usage.plist`. Re-run it to update.
3. If Bartender hides the new icon, move it to the shown section (Bartender settings), or quit
   Bartender, move `com.chansearrington.home-usage-home-usage` to `Show` in its active profile, and
   reopen it.

Settings (optional): `defaults write com.chansearrington.home-usage homeURL http://host:8327` and
`pollSeconds` (minimum 30, default 120). The default URL is the Ark's Tailscale name, which only
resolves on the tailnet; the app's transport-security exception covers `*.taile4a41.ts.net` only,
so a different host needs HTTPS (or a matching `NSExceptionDomains` entry in `install.sh`).
Apple Silicon only (`-target arm64-apple-macos14`).

`"Home Usage.app/Contents/MacOS/HomeUsage" --print` fetches once and prints the bar text and menu
as plain text (used for proof and troubleshooting); `--show-menu` asks the running app to open
its menu for four seconds so it can be screenshotted without moving the mouse.

## Turn it off

- For now: click it → **Quit Home Usage** (it comes back at next login).
- For good: `tools/home-menubar/uninstall.sh` (add `--forget-key` to also delete the Keychain
  item).

## Safety notes

- Home has no read-only key; the management key can change everything. It lives only in the
  login Keychain and in memory; it is never logged or written to disk by this app.
- On a 401/403 the app stops polling at once (Home bans a client IP for 30 minutes after five bad
  keys), remembers that across relaunches (`authPaused`), and waits for **Retry once**. Tested
  against a fake server that always answers 401: one request, then silence, also after a relaunch.
- "Refresh now" only appears once accounts are loaded and only names those accounts (Home reads an
  empty list as "all").
- It never calls `/usage-queue` (destructive) or `/api-key-usage` (embeds raw keys).
