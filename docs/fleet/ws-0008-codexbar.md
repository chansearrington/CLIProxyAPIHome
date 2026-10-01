# ws-0008 — CodexBar and CPA: usage in the menu bar

Status: started 2026-10-01

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
