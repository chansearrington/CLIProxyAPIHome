# ws-0008 — CodexBar and CPA: usage in the menu bar

Status: removed 2026-10-01 — Chanse did not want the Home Usage app; uninstalled, Keychain key deleted, Bartender entries removed, source deleted from the repo

- Branch `ws-0008/codexbar` off `fleet` (for this brief); worktree `.claude/worktrees/ws-0008-codexbar`
- herdr: `CLIProxyAPIHome | ws-0008 | CodexBar`, alias `cpahome-ws-0008`

## Chanse's words (2026-10-01)

"On this machine, I run Codex Bar. And I'd like it to somehow integrate with CPA. But I don't even
know if that's possible or even worth doing. Right now, most of my token counts in Codex Bar aren't
right because they're not connected to CPA and/or the multiple accounts connected to them. Maybe
there's a better way to do this overall. I don't know, but I enjoy seeing the things in my menu bar,
and right now I can't. That's the bottom line."

**Bottom line to deliver:** accurate usage/allowance for the accounts Home manages, visible in
Chanse's macOS menu bar.

## Facts at start

- CodexBar **0.60.3** (`/Applications/CodexBar.app`, bundle `com.steipete.codexbar`, Sparkle
  auto-update on) is running; status items for claude, codex, grok are visible, merged view off.
  Config at `~/.codexbar/config.json` and `~/Library/Application Support/CodexBar/` (+ `providers`).
  It reads each provider's own local login/CLI/cookies — so it sees only the accounts logged in on
  this Mac, not the accounts Home holds.
- Home already has what the menu bar needs: Management API `GET /v0/management/quota/credentials`
  returns every credential's quota windows (Claude ×2, Codex, Antigravity, xAI, Copilot — see
  ws-0005), and Home's usage ledger has per-key/per-user token counts. Management needs the
  management secret; check whether a less-privileged read path exists (per-user / per-API-key usage).
- The MacBook node listens on `127.0.0.1:18317`; the MacBook key is at `~/.config/cpa-home/client-key`.

## Work

1. Research (use subagents; read CodexBar's source — github.com/steipete/CodexBar — not just docs):
   does CodexBar support custom/OpenAI-compatible providers, external data sources, a plugin or
   script hook, or a CLI/widget feed? Is there an upstream issue/PR for proxies like CPA? What do
   its current token counts actually measure and why are they wrong here?
2. Options table, each with pros/cons, effort, maintenance cost, and security (where the Home
   credential lives — never a plain file in a repo; Keychain or 1Password preferred), e.g.:
   a) configure CodexBar to point at CPA/Home if it supports it;
   b) contribute a CPA/Home provider to CodexBar upstream (and run a local build meanwhile);
   c) a small separate menu bar item (e.g. SwiftBar/xbar script or a tiny native app) reading Home's
      quota + usage;
   d) Home's own web panel / a widget instead.
3. **Recommend one, then build it** if it is laptop-only and reversible (no stop point — Chanse's
   standing rule). Anything touching the live Home or nodes follows ORCHESTRATION.md guards and
   waits for ws-0007's fleet-locks.
4. Proof: a screenshot of the menu bar (`screencapture`) showing the managed accounts with numbers
   that match Home's quota API at the same moment; record both in this brief. Note what is still
   not covered.
5. Plain-English write-up for Chanse at the top of the close-out: what he will see, how to read it,
   how to turn it off.

## How to work

- Read `docs/fleet/ORCHESTRATION.md` first; its guards apply to you in full.
- Also read `FLEET.md`, `AGENTS.md`, and the memory notes loaded into your session.
- **No stop points.** Chanse gave full autonomy for this round, including deploys. Decide, act
  under the guards and report results with evidence. Stop for Chanse only if something is
  irreversible AND destroys data or money.
- Use subagents for research and review, and check their work yourself.
- Track your work with the task list (TaskCreate/TaskUpdate) from the start.
- Keep the `Status:` line below current, and commit brief updates to your branch as you go.
- Close-out: outcome, evidence per acceptance criterion, what is live, follow-ups. Then remove your
  worktree/branch once merged, and set Status to `done`.

## Findings (written as we go)

### Local CodexBar install (checked 2026-10-01)

- Correction to "Facts at start": the config is `~/.config/codexbar/config.json` (there is no
  `~/.codexbar/`). App preferences are in the `com.steipete.codexbar` defaults domain; data
  (usage history, Codex account snapshots) in `~/Library/Application Support/CodexBar/`.
- Enabled providers: `claude` (source `cli`), `codex` (`liveSystem`), `grok`. 60+ others listed
  but disabled, including generic proxy ones: `litellm`, `llmproxy`, `sub2api`, `clawrouter`,
  `openrouter`. There is no `cliproxyapi`/CPA provider in 0.60.3.
- Menu bar mode `percent`, refresh every two minutes, launch at login on, a WidgetKit extension
  running (`CodexBarWidget.appex`).
- Xcode + Swift 6.4 are installed on the laptop; SwiftBar/xbar are not.

### Home's read paths for quota and usage (code read, spot-checked 2026-10-01)

- Quota: `GET /v0/management/quota/credentials` (`internal/managementhttp/server.go:221`), detail
  `GET .../quota/credentials/:credential_id` (`:222`), force-probe `POST .../quota/collect`
  (`:223`). Item shape = `QuotaCredentialSnapshot` / `QuotaWindow`
  (`internal/cluster/quota_snapshots.go:129-219`): `provider`, masked `label`/`account`,
  `quota_status`, `freshness`, `observed_at`, `primary_windows[]` (≤2 per credential; full set only
  in the detail) with `used`/`limit`/`remaining`/`used_ratio`/`reset_at`/`unit`.
  Units differ: Claude/Codex/Antigravity `percentage` (0-100), Copilot `requests`, xAI `currency`.
  Codex's primary pair is the weekly window + Spark, not the 5-hour one.
- GET is a pure database read; Home's collector (`internal/quota/collector.go:24-28`) probes every
  minute but only credentials used in the last 30 min, and a snapshot is "fresh" for 30 min. Idle
  accounts show `stale` with their last-known numbers. `POST /quota/collect` makes real provider
  calls — never on a timer, only on a manual refresh.
- Usage/tokens: `GET /usage/overview`, `/usage/aggregates?group_by=…`, `/billing/*` — all
  Management-only. Avoid `/usage-queue` (it pops records) and `/api-key-usage` (embeds raw keys).
- **No less-privileged path.** One management secret, all-or-nothing (it can also read client keys
  and `config.yaml`) — treat it as root. Nodes expose no usage/quota route to API-key clients, and
  their own management returns 404 under Home. The `/user/*` portal (username+password → 24 h JWT)
  only shows credits/billing, not provider quota.
- Lockout: 5 bad/missing keys from one client IP → 30-min ban (CPA SDK v7.2.83
  `handler.go:300-397`). Home runs on a Docker bridge with the port published on the Ark's Tailscale
  IP, and its logs record real client addresses (e.g. ws-0005's `100.88.81.10`), so a bad key from
  the laptop bans only the laptop. A poller must stop on the first 401.
- The secret lives on the Ark as `MANAGEMENT_PASSWORD` in `/mnt/user/appdata/cpa-home/.env`
  (value never printed). The laptop already holds the Ark's root SSH key, which is strictly more
  powerful, so a copy in the laptop's login Keychain does not widen who can control the fleet.

### CodexBar's source (v0.60.3 = tag commit `2b78164`; main and v0.70.0 checked too)

- **Providers are a fixed built-in list** (`Sources/CodexBarCore/Providers/Providers.swift:21`,
  69 at v0.60.3). No CLIProxyAPI provider. Generic gateways exist (LLM Proxy, LiteLLM, Sub2API,
  ClawRouter, Bifrost), each reading its own gateway's API shape — none speaks Home's.
- **Local plugins exist** (`docs/plugins.md`): one `.js`/`.ts` file in
  `~/.config/codexbar/providers/` (already created, empty), sandboxed GET/POST to approved origins
  only, returns rate windows + detail rows. Limits that matter here:
  - **No menu bar icon of its own.** Status items are only vended for built-in `UsageProvider`
    cases (`StatusItemController+StatusItemVending.swift:6-18`); a plugin is a card appended inside
    a built-in provider's dropdown, or a dropdown tab with Merge Icons on. Same on main
    (`docs/plugins.md:404-407` there).
  - **Secrets are stored as plain text** in `~/.config/codexbar/config.json` (`pluginSecrets`,
    `CodexBarConfig.swift:194-196`, `SettingsStore+Config.swift:75-81`), not the Keychain.
  - Plain HTTP only to loopback/RFC 1918/`.local`; Home's Tailscale `100.x` address would need HTTPS
    (`ProviderEndpointOverrideValidator.swift:150-175`).
  - Plugins are excluded from widgets.
- No URL scheme, no push/ingest path; `codexbar serve` is read-only; `hooks` only fire outward.
  `ANTHROPIC_BASE_URL`/`OPENAI_BASE_URL`/proxy env vars are ignored.
- Upstream: every CPA-specific PR was closed unmerged (#235, #335, #1614, #2413, #2415, #2442,
  #2457 — the last on 2026-09-22 as "niche integration … use the plugin-host architecture").
  A built-in provider is ~27 files / ~525 lines (xKiro, `9090006`).

### Why CodexBar's numbers are wrong on this Mac

- **Claude**: usage % comes from Anthropic's OAuth usage API for whatever Claude login sits on this
  Mac (`ClaudeOAuthUsageFetcher.swift:61`; source `cli` here). But Claude Code on this laptop does
  not use that login — `~/.claude/settings.json` sends everything to the local CPA node
  (`ANTHROPIC_BASE_URL=http://127.0.0.1:18317`, `apiKeyHelper` = the MacBook fleet key), which
  spends Home's two Claude accounts. So the % describes an account the fleet doesn't use.
- **Token/cost counts** are summed from this Mac's own session logs (`~/.claude/projects/**/*.jsonl`,
  `~/.codex/sessions`) and priced at list price — they never see the minis' traffic, and the
  "cost" is not what Home's accounts actually spend.
- **Codex**: `~/.codex/auth.json` is a direct ChatGPT login (`auth_mode: chatgpt`), so the % is that
  one login, not Home's Codex credential(s). **Grok**: the local Grok CLI login, likewise.
- There is no setting in CodexBar that can re-point these at Home.

## Options

| | Option | In the menu bar? | Effort | Upkeep | Where the Home key lives | Verdict |
|---|---|---|---|---|---|---|
| a | CodexBar plugin reading Home | No — only a card inside CodexBar's dropdown | Small (1 file) + a localhost relay for the key and the `100.x` HTTP rule | Plugin API is young (Aug 2026), may shift | Plain text in CodexBar's config — or a relay holding it in Keychain | Second best |
| b | Built-in CPA/Home provider upstream + local build meanwhile | Yes | Large (~27 files) | High: unsigned local build loses auto-update, Keychain prompts; upstream has refused 7 CPA PRs | Keychain | No |
| c | **Small native "Home" menu bar item** (single Swift file, built on this Mac) | **Yes, its own number** | Small-medium (~300 lines) | Low: no third-party code; the built app keeps working through Xcode/CodexBar updates | **Login Keychain**, read via `/usr/bin/security` | **Recommended** |
| c′ | Same via SwiftBar/xbar script | Yes | Small | Adds a third-party app to install and update | Keychain | Fine, but needs another app |
| d | Home's web panel / widget | No (browser tab) | None | None | Browser session | Already exists; not what Chanse asked for |

**Decision (2026-10-01): build (c).** It is the only option that puts correct numbers in the menu
bar itself, keeps the key in the Keychain, and leaves CodexBar untouched (Chanse keeps it for
everything it does get right). It is laptop-only and removable in one command. Design:

- Reads `GET /v0/management/quota/credentials` + each credential's detail (all windows, incl.
  Codex's 5-hour) and `GET /usage/overview` (fleet tokens, last 24 h) every 2 minutes — database
  reads only, no provider calls. "Refresh now" forces `POST /quota/collect` for the shown
  credentials, only on click.
- Bar text: the highest "used %" across the Claude and Codex windows that limit work right now;
  dropdown: every account with each window's used %, reset time and freshness.
- Key: the Home management secret copied from the Ark's `.env` into the login Keychain over
  stdin (never argv, never a file). Stops polling on the first 401/403 so it can never trip Home's
  5-strikes ban.
- Source in this repo at `tools/home-menubar/`; app at `~/Applications/Home Usage.app`; started at
  login by `~/Library/LaunchAgents/com.chansearrington.home-usage.plist`.

## Build (2026-10-01, laptop only)

- Source `tools/home-menubar/` (`HomeUsage.swift` ~560 lines, `install.sh`, `uninstall.sh`,
  `README.md`). Built with the laptop's `swiftc` (Swift 6.4), ad-hoc signed, installed to
  `~/Applications/Home Usage.app`, started by LaunchAgent `com.chansearrington.home-usage`.
- Key: copied from the Ark's `.env` straight into the login Keychain (service
  `cpa-home-management`, account `home-usage`) by piping `add-generic-password … -w '<value>'`
  into `security -i` — the value never touched argv, a file or the screen (length check only).
  First call with it: `GET /quota/credentials` → 200 (remote management is on).
- **Bartender 6** (installed on this Mac) put the new icon in its *Always Hide* list, so it sat
  off-screen. Fixed by backing up Bartender's prefs
  (`~/Library/Preferences/bartender-backup-pre-ws0008-20260930T205348.plist`, 189,380 bytes),
  quitting Bartender, moving `com.chansearrington.home-usage-home-usage` to its `Show` list and
  reopening it, then pinning the icon's position next to CodexBar
  (`defaults write com.chansearrington.home-usage "NSStatusItem Preferred Position home-usage" 470`).
  It now sits immediately left of CodexBar's three icons on the active display.
- Lesson: on this Mac, whole-display `screencapture` shows only wallpaper while the screen is locked
  (loginwindow shields at layer 2001-2004) — check for that before debugging "missing" icons.
  Individual windows can still be captured with `screencapture -l <window id>`.

## Proof

- **2026-10-01 02:00:45Z — numbers (screen locked, so text + window captures):**
  `HomeUsage --print` and `GET /v0/management/quota/credentials` taken within seconds of each other;
  `compare.py` checked every window: **10 windows match, 0 mismatches**
  (`docs/fleet/ws-0008-proof/compare-20261001T020045Z.txt`, menu text in
  `menu-text-20261001T020045Z.txt`). Bar = `CPA 92%` = the Claude gmail account's 5-hour window
  (92/100), the fullest Claude/Codex window — correct by the headline rule.
- Menu bar at the same moment (`docs/fleet/ws-0008-proof/menubar-20261001T0200Z.png`, stitched
  from the four status-item windows, left to right): Home Usage `CPA 92%` (orange, ≥80 %),
  CodexBar Grok `26%`, CodexBar Codex `30%`, CodexBar Claude `–`. CodexBar shows **no Claude
  number** (the Mac's own Claude login is not what the fleet uses). Its Codex 30 % equals Home's
  Codex weekly 30 %, so the laptop's ChatGPT login is the same account Home holds.
- **2026-10-01 02:08:30Z — real menu bar screenshot (screen unlocked):**
  `docs/fleet/ws-0008-proof/menubar-dropdown-20261001T020830Z.png` shows `CPA 92%` next to
  CodexBar's `26%` / `30%` / `–`, with the Home Usage dropdown open. API snapshot taken in the same
  second (`api-20261001T020830Z.txt`), app text (`menu-text-20261001T020830Z.txt`), and
  `compare-20261001T020830Z.txt`: **10 windows match, 0 mismatches.** Examples visible in the shot:
  Claude gmail 5-hour 92 % (API used 92/100, resets 21:09 CDT = 02:09:59Z), Claude hypesports
  5-hour 81 %, weekly 54 %; Codex weekly 30 % (Pro 20x); Copilot premium 1 of 1,000,000; xAI
  $0.00 of $0.00 exhausted; Antigravity 0-1 %, numbers 21 h old, marked unavailable.
- An earlier shot at 02:07:23Z showed long note lines clipped at the right edge; fixed by moving
  status notes to their own short line (`old (reset since)`), rebuilt, re-shot above.
- **Not covered:** accounts Home does not hold (the Grok CLI login CodexBar shows); xAI shows
  "$0.00 of $0.00, exhausted" exactly as Home's collector reports it (why Home sees a $0 limit
  was not investigated here); Home's own staleness rule (idle accounts are
  only re-measured when used or on "Refresh now"); the "Iguana Necktie" Claude window is the label Home
  reports for that window, shown as-is (origin not investigated); CodexBar's widgets (plugins/other apps cannot feed them).

## Review (2026-10-01)

Independent code review (subagent) of `31695ad`: decoding matches the Go structs, no secret leaks,
date parsing verified. Fixed in `64fd1f0`:
- MUST: optional calls (overview, per-credential detail) swallowed 401/403 with `try?` — a rotated
  key could have sent several failing requests in one poll. Now auth errors always propagate.
- MUST: "Refresh now" before the first load sent `credential_ids: []`, which Home reads as "all"
  (`quota_recollect.go:9`). Now guarded and hidden until accounts are loaded.
- SHOULD: one poll at a time; the auth pause survives relaunch (`authPaused`); default URL is the
  Ark's MagicDNS name `home-server-the-ark.taile4a41.ts.net` (does not resolve off the tailnet) and
  the ATS exception is scoped to `*.taile4a41.ts.net` instead of "allow everything"; the client is
  main-actor isolated and reads the Keychain off the main thread with a 10 s cap; per-account
  24 h usage comes from `/usage/aggregates?group_by=credential` (the overview's top list is capped
  at 10); the bar ignores disabled/never-measured accounts; windows whose reset time has passed say
  "old (reset since)".
- NITs: cached date formatters, currency without a limit, `credits` unit, launchctl bootout wait,
  `pkill` of a hand-started copy, visible codesign errors, Apple-Silicon note.
- Auth-pause test: fake server on `localhost:18999` answering 401 → exactly **1** request, bar
  `CPA ⚠︎`, `authPaused=1`; a relaunched instance sent **0** requests. Restored to the real Home
  afterwards (`CPA 92%`).
- Fleet local review (`local-pr-review`, gpt-6-astra) of PR #2 at `dd3376b`: **MUST 0 · SHOULD 2**.
  Both fixed rather than left open: a failed per-credential detail read now flags the account
  ("some limits didn't load") and puts `?` on the bar instead of a quietly low headline (issue #3);
  "Refresh now" shares the one-request-at-a-time guard with polling (issue #4).

## Close-out

### For Chanse — what you'll see, how to read it, how to turn it off

- **What's new:** a small orange or white **`CPA 92%`** in your menu bar, just left of CodexBar's
  icons. It's a separate little app called **Home Usage**, and it starts by itself at login.
  CodexBar is unchanged; it simply can't see the accounts Home uses (details above), so Home Usage
  sits next to it instead of inside it.
- **The number** is the fullest limit across your Claude and Codex accounts right now, the one
  most likely to slow the fleet down. White means under 80 %, orange 80 % or more, red 95 % or
  more. Home switches between your two Claude accounts on its own, so one account at 92 % doesn't
  mean you're blocked; the dropdown shows the other one.
- **Click it** to see every account Home holds: each limit (5-hour, weekly, monthly…) with how much
  is used and when it resets in your local time, plus how many requests and tokens that account
  handled in the last 24 hours. The bottom line is the whole fleet's last 24 hours. "numbers from
  5 h ago" means Home hasn't re-checked that account because nobody has used it lately.
- **Refresh now** makes Home re-check the accounts with Claude, OpenAI, etc. on the spot. It
  refreshes by itself every 2 minutes anyway, without bothering the providers.
- **If you see `CPA ⚠︎`**: Home refused the app's key, so the app stopped asking, on purpose (Home
  locks out a computer after five wrong tries). Ask me to fix the key, then click **Retry once**.
- **Turn it off:** click it → **Quit Home Usage** (it comes back at your next login). To remove it
  for good, ask me, or run `tools/home-menubar/uninstall.sh` in this repo.
- **One thing I changed in Bartender:** it hid the new icon automatically, so I moved it to
  Bartender's "shown" list (your old Bartender settings are backed up first). You can still drag it
  anywhere in Bartender like any other icon.
- **Suggestion (your call, nothing changed):** CodexBar's Claude icon shows "–" because the Claude
  login on this Mac isn't the one the fleet uses. If it bothers you, turn off just the Claude
  provider in CodexBar's settings; Home Usage covers it now.

### Outcome and evidence

- **Outcome:** Home's accounts (2× Claude, Codex, Copilot, xAI, Antigravity) are in the menu bar
  with Home's own numbers, through a small native app next to CodexBar. CodexBar could not be
  pointed at Home: no CPA provider, plugins get no menu bar icon and keep secrets in plain text,
  and upstream has refused every CPA provider PR.
- **Acceptance, by brief step:**
  1. Research from CodexBar's source (v0.60.3 + main) and Home's code, with file:line, in Findings ✅
  2. Options table with pros/cons, effort, upkeep and key storage ✅
  3. Recommended (c), built, laptop-only and reversible (`uninstall.sh`) ✅. Nothing on the Ark,
     Home or the nodes was changed, so no fleet-locks were needed. The only Ark access was
     read-only: `docker inspect` and reading the key into the Keychain.
  4. Proof: real menu bar screenshot with the dropdown open, plus an API snapshot taken in the same
     second, 10/10 windows match (`docs/fleet/ws-0008-proof/*20261001T020830Z*`); not-covered list
     in Proof ✅
  5. Plain-English write-up at the top of this Close-out ✅
- **Reviews:** subagent review (2 MUST + 6 SHOULD + NITs, all fixed); fleet `local-pr-review` on
  PR #2 twice: `dd3376b` MUST 0 / SHOULD 2 (#3, #4 fixed in `256d29e`), `256d29e` MUST 0 /
  SHOULD 1 (#5, failed manual refresh now shown, fixed in the next commit).
- **Live (laptop only):** `~/Applications/Home Usage.app`, LaunchAgent
  `com.chansearrington.home-usage`, Keychain item `cpa-home-management`/`home-usage`, Bartender
  `Show` entry (backup `~/Library/Preferences/bartender-backup-pre-ws0008-20260930T205348.plist`).
- **Follow-ups (none blocking):**
  - If Home's management password is ever rotated, update the Keychain item; the app will show
    `CPA ⚠︎` until then, by design.
  - A read-only management key in Home would let this app (and similar tools) stop holding the
    root secret. That is upstream-sized work and was not started.
  - Optional for Chanse: turn off CodexBar's Claude provider, which shows `–` on this Mac.
- **Cleanup:** after merge, remove worktree `.claude/worktrees/ws-0008-codexbar` and branch
  `ws-0008/codexbar` (local and remote); `/tmp/ws8-*`, `/tmp/cbsrc` and `/tmp/ws8-proof` deleted.

## Removed (2026-10-01)

Chanse: "I don't want this app that you made. Thank you for making it, but that doesn't give me what
I really want. Is there a way for multiple accounts to be tracked inside of CodexBar? And please
uninstall and remove the application that you created."

- `tools/home-menubar/uninstall.sh --forget-key`: LaunchAgent booted out and deleted, app, log and
  defaults deleted, Keychain item `cpa-home-management`/`home-usage` deleted. Verified: not running,
  files gone, key gone.
- Bartender: 10 `home-usage` references removed (Bartender quit, prefs edited, relaunched); backup
  `~/Library/Preferences/bartender-backup-pre-homeusage-removal-20260930T213231.plist`.
- Source `tools/home-menubar/` deleted from `fleet` (still in git history at PR #2's merge).
- What Chanse actually wants is multiple accounts inside CodexBar itself — researched next.

## Multi-account inside CodexBar (research 2026-10-01, spot-checked by the orchestrator)

- CodexBar (v0.60.3 and v0.70.0, released 2026-09-30) supports several accounts per provider, shown
  inside that provider's one menu bar icon (layout `stacked` = all at once, up to 6; already set).
  One icon per account is not built (issue #1843, open).
- Ways in: Claude = pasted claude.ai `sessionKey` cookie / OAuth access token per account, or a
  "claude-swap" adapter (`ClaudeSwapAccountReader.swift:38` runs `<exe> --list --json`); Codex =
  in-app "Add account" (own `codex login` in a CodexBar-managed home); Copilot = in-app GitHub
  sign-in per account.
- Safety: never copy Home's Codex/Claude refresh tokens into CodexBar — a refresh on the laptop
  revokes Home's copy. Separate logins (cookie, own `codex login`) are independent of Home's.
- Key point for accuracy: limits (5-hour, weekly) are per account, so CodexBar watching the same
  account Home uses shows the same % no matter which machine spent it. CodexBar's token/cost
  counts, however, come from this Mac's own logs and will never include the minis' traffic.
- Current config: no `tokenAccounts`; one Codex managed account that duplicates the laptop's own
  login.

## Resolved (2026-10-01) — both Claude accounts inside CodexBar

- CodexBar 0.60.3 → Settings → Providers → Claude → **Claude cookies = Manual** reveals the "Claude
  credentials" list; Chanse added two claude.ai `sessionKey` accounts ("Hype", "Gmail"). These are
  web sessions independent of Home's OAuth credentials (no refresh-token sharing).
- Only one account can be "selected" — that only picks which drives the menu bar number (one icon
  per provider; per-account icons = steipete/CodexBar#1843, open). Settings → Menu → Multi-account
  layout = **Stacked** (already set) shows a card per account in the Claude dropdown.
- Verified with `codexbar usage --provider claude --all-accounts --format json`: both accounts read
  via `web`; Hype 5-hour 100 % (matches Home's 429 cooldown on hypesports), Gmail 25 %. Chanse
  confirmed both cards visible.
- CodexBar's "Cost" figures are local session logs priced at API list price — not real spend and not
  fleet-wide.
