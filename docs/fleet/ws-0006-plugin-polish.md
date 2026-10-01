# ws-0006 — Copilot plugin polish

Status: v0.3.7 released + verified; rollout ready, waiting for ws-0005 to release the fleet-locks

- Repo `/Users/chansearrington/GitHub/personal/cliproxyapi-copilot-plugin` (fork, base branch
  `fleet` @ `758ada4`, release v0.3.6 live). Worktree `.claude/worktrees/ws-0006-plugin-polish`
  in that repo, branch `ws-0006/plugin-polish`.
- herdr: `CopilotPlugin | ws-0006 | Plugin polish`, alias `cpaplugin-ws-0006`
- This brief is tracked in CLIProxyAPIHome (`docs/fleet/`), next to ws-0002's record, which
  holds the full plugin history (`docs/fleet/ws-0002-copilot-provider.md`). Update this file in the
  main CLIProxyAPIHome checkout's `fleet` branch (small docs-only commits, pull --rebase first).

## Goal

Fix the two review SHOULDs deferred in ws-0002 and ship them fleet-wide:
1. Terminal usage counts on chat streams to Responses-only models (PR #1 round 2).
2. Text-block citations dropped when synthesising Claude SSE (PR #2).

## Acceptance criteria

1. Each fix has a failing-then-passing test; plugin suite, gofmt and vet clean on the Ark
   (`golang:1.26-bookworm`).
2. PR to the fork's `fleet`, reviewed with `local-pr-review`, MUSTs fixed, merged with evidence.
3. Release v0.3.7 with linux + darwin/arm64 artifacts (the fork's release process from ws-0002;
   if Actions is billing-blocked, build both on the Ark/laptop as ws-0002 recorded and say how),
   checksums + sizes recorded here.
4. Rolled out by a version bump in Home's config (`plugins.configs`, full `api-keys` list carried
   in any `PUT /config.yaml`) under the ORCHESTRATION.md guards; all five nodes report
   `0.3.7 installed loaded` (a version bump hot-reloads; no node restart expected).
5. The ws-0002 19-request format proof re-run: 19/19 non-empty, plus a check that chat-stream
   usage now carries real token counts. Lara's key still refused for `copilot/`.
6. Handle anything listed under "Requests from other workstreams" below (ws-0005 may add one).

## Requests from other workstreams

(none yet)

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

## Record

### Findings (2026-09-30)

- **Fix 1 — chat-stream usage.** Chat streams to Responses-only models go Responses SSE →
  `responsesStreamToClaude` → CPA's official Claude → chat converter, which takes its usage from
  `message_start` + `message_delta` (CPA v7.2.118
  `internal/translator/claude/openai/chat-completions/claude_openai_response.go:233-251`).
  `message_start` goes out at `response.created`, before Copilot reports input tokens, and the
  plugin's `message_delta` carried only `output_tokens`
  (`internal/translate/claude_responses_stream.go` `finish`), so the terminal chat chunk said
  `prompt_tokens: 0`. Also found: Responses `input_tokens` include cached tokens while Claude's
  exclude cache reads; the non-stream path set both, so a Claude-semantics reader counted cached
  tokens twice. Fix: one mapping `claudeUsageFromResponses` used by `message_start`,
  `message_delta` and the non-stream answer.
- **Fix 2 — citations.** `claudeMessageToSSE` (`internal/translate/claude_events.go`) rendered a
  text block as start + `text_delta` only. CPA's Claude → Responses converter turns
  `citations_delta` into `annotations` (`claude_openai-responses_response.go:817-825`), so Responses
  clients lost them. Fix: cited blocks open with `citations: []` and stream one `citations_delta`
  per citation ahead of the text, as Claude does.
- Tests (failing first, on the Ark): `TestResponsesSSEToOpenAIChatStreamCarriesTerminalUsage`
  (was `prompt_tokens 0`), `TestResponsesUsageMapsToClaudeCacheSemantics` (was input 9 + cache 4),
  `TestClaudeJSONCitationsReachResponsesAnnotations` (no `citations_delta`). After the fix:
  `golang:1.26-bookworm` go1.26.8, gofmt empty, vet clean, full plugin suite ok. Builds run from
  a private Ark dir `/mnt/user/appdata/cpa-home-build/plugin-ws0006` (rsync of the worktree,
  3 GB exec tmpfs), so the shared `src` dir is untouched.

### PR, review, release (2026-10-01 UTC)

- PR https://github.com/chansearrington/cliproxyapi-copilot-plugin/pull/3, head `04c02ef`.
  `local-pr-review` (gpt-6-astra): **MUST 0, SHOULD 0, NICE 0**, "No issues found"; it re-ran
  `go test ./...` and the three new tests. Fork CI (tests + both packages): all pass.
- Merged as **`1a2925b`** on `fleet`; tag **`v0.3.7`** on it. Release run `36794568345`
  (fork Actions, ubuntu + macos-15; the repo is public, so not billing-blocked): success.
- Verified locally: `shasum -c checksums.txt` OK; each zip holds one library at its root.

| File | Size (bytes) | sha256 of the zip (what Home pins) | sha256 of the library inside |
|---|---|---|---|
| `cliproxyapi-copilot_0.3.7_linux_amd64.zip` (Home) | 13271393 | `b6b0637708ddf71ddeca9eb1be0a26f10cccdceb22c581164385f6441fea8bd5` | `a242bd2942fa557c9b4c2486e1080687c6ee1ca492f72a1f321480e5adf4ea83` (ELF x86-64; = Ark build) |
| `cliproxyapi-copilot_0.3.7_darwin_arm64.zip` (nodes) | 7152626 | `9b93458d971dc2d0cc78ded51db69f2a8f02beeaea9464ab8527d9ccf8e61ad7` | `4ad2eb09a1ae1ae62d4fe4d7598c4d0b2852487430aed99d6e1a6bfc010bb59c` (Mach-O arm64) |

- **Reproducible:** the Linux library built on the Ark from `v0.3.7` (`golang:1.26-bookworm`,
  the Makefile's flags) is byte-identical to GitHub's (`a242bd29…`).

### Rollout preparation (2026-10-01 UTC)

- Baseline (00:3xZ): Home `cpa-home:1.0.73-claude-fleet-95ba2db`; all five nodes healthy,
  `reported_ok`, plugin `0.3.6 loaded`. **Bug live on 0.3.6:** chat stream to
  `copilot/gpt-5.6-sol` reported `prompt_tokens=0 completion_tokens=5`; `copilot/gpt-5-mini`
  (11) and `copilot/claude-haiku-4.5` (12) were already correct, as expected (only the
  Responses-only bridge was affected).
- Config change built on the Ark from a fresh `GET /config.yaml`: exactly 7 changed lines (version,
  two URLs, two sha256, two integer sizes); all six `api-keys` carried; the `openai-compatibility`
  credential root omitted so Home leaves it alone. Rollback file = same with the 0.3.6 pin.
  Working files in `/root/ws0006` on the Ark (mode 0600; deleted at close-out).
- At lock time ws-0005 held the fleet-locks for its Home deploy; this rollout waits for all four to
  be free for two checks two minutes apart (both workstreams use the canonical session/actor, so
  the lock tool would treat a second acquire as a refresh, not a conflict).
