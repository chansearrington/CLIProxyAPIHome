# ws-0006 — Copilot plugin polish

Status: both fixes written + tested on the Ark (commit 04c02ef); PR + review next

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
