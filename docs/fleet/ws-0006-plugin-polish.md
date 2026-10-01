# ws-0006 — Copilot plugin polish

Status: done 2026-10-01 — plugin v0.3.7 live on Home + all five nodes; 19/19 proof, chat-stream usage real

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

### Rollout (2026-10-01, under all four fleet-locks 01:05:00Z → 01:07:54Z)

- Locks: `fleet-lock hold` moxy → hyper → lara → chip (canonical
  `cpa-home-runtime-20260907-root` / `cpa-home-root`, auto-renew every 100 s), taken only after all
  four were free for two checks 2 min apart (ws-0005's driver had held them until ~01:02Z).
  Released cleanly. Home image at the time: `cpa-home:1.0.73-claude-fleet-16f609d` (ws-0005's);
  no image change here.
- Config re-fetched under the locks and re-checked: 7 changed lines, 6 API keys, credential root
  omitted.
- **Backup:** `data/backups/home-pre-ws0006-20261001T010500Z.db`, `VACUUM INTO` via nohup,
  **2,409,762,816 bytes, integrity ok, sha256
  `2e3543f730e4f55539d65fdb6fde04a4593e5fe051468536d3ede485a12124a1`** (01:05:00Z → 01:07:37Z).
- `PUT /config.yaml` 01:07:37Z → HTTP 200 `{"changed":["config"],"ok":true}`.
- Home (`linux/amd64 .so`) and every node logged `plugin hot reloaded active_version=0.3.7
  retired_version=0.3.6` at 01:07:38-40Z; no retry loop, no restart, no panic. Node check 10 s
  later: **all five `healthy true | reported_ok | 0.3.7 installed loaded`** (MacBook, Moxy, Hyper,
  Lara, Chip).
- Rollback (not needed): `PUT` the same config with the 0.3.6 pin (URLs/sha/size in ws-0002's
  record); the prepared file was deleted at close-out with the other Ark working files.

### Proof (2026-10-01 01:08Z)

- Native model from every agent key through its own node (`claude-haiku-4-5-20251001`, each box's
  OpenClaw `cpa-gui` key, nothing printed): Moxy, Lara, Chip, Hyper **200 "ok"**; MacBook key 6
  **200 "ok"**.
- Lara's key on `copilot/gpt-5-mini`: **503 `auth_not_found: no auth available`** (still refused).
- **Format matrix 19/19 non-empty** through the MacBook node: `copilot/gpt-5-mini`,
  `copilot/claude-haiku-4.5`, `copilot/gpt-5.6-sol` × chat / responses / messages × json / stream,
  plus `copilot/gpt-5.6-sol` `/v1/messages` `max_tokens` 10. All answered "ok".
- **Chat-stream usage now real:** `copilot/gpt-5.6-sol` chat stream `prompt_tokens=11
  completion_tokens=5 total=16` (0.3.6 baseline 00:3xZ: `prompt_tokens=0`). Home's `usage` table
  agrees: on 0.3.6 the gpt-5.6-sol chat stream (05:55:03Z, 00:13:20Z) and Claude-format stream
  (05:55:06Z) were stored with `input_tokens 0`; on 0.3.7 every chat and messages row is 11 / 5.
- Requests since the change (to 01:09Z): 86, 1 failed — a native `claude-opus-5-5` call on the
  MacBook cancelled by its client (499 `context canceled`), unrelated.
- Citations (fix 2) cannot be triggered on demand through Copilot; covered by the unit test
  that runs the real CPA Claude → Responses converter.

## Close-out (2026-10-01)

**Outcome.** Both review SHOULDs deferred in ws-0002 are fixed and live fleet-wide in plugin
v0.3.7: chat (and Claude-format) streams to Copilot's Responses-only models now report real input
token counts to clients and to Home's usage ledger, cached tokens are no longer double-counted in
Claude-format usage, and Claude citations reach Responses clients as annotations.

**Acceptance criteria.**
1. Failing-then-passing tests for each fix (3 tests); Ark `golang:1.26-bookworm` gofmt empty, vet
   clean, full plugin suite ok ✅
2. PR #3 reviewed with `local-pr-review` (MUST 0 / SHOULD 0 / NICE 0), CI green, merged `1a2925b` ✅
3. Release v0.3.7 by the fork's Actions; checksums + sizes recorded above; Linux library
   byte-identical to an Ark build ✅
4. Version bump via Home config under the guards; Home + all five nodes `0.3.7 installed loaded`
   by hot reload, no restart ✅
5. 19/19 non-empty; chat-stream usage real (11 vs 0); Lara still refused for `copilot/` ✅
6. No requests from other workstreams (ws-0005 recorded none) ✅

**What is live.** Plugin fork `chansearrington/cliproxyapi-copilot-plugin` `fleet` @ `1a2925b`,
release v0.3.7, pinned in Home's `plugins.configs.cliproxyapi-copilot`. Home image unchanged by
this workstream (`cpa-home:1.0.73-claude-fleet-16f609d`).

**Follow-ups (not done here).**
- **Non-streamed `/v1/responses` on Copilot is stored in Home's usage ledger with 0 tokens**
  (`copilot/gpt-5.6-sol`, 01:08:45Z on 0.3.7 and 05:55:01Z on 0.3.6), although the client receives
  the usage (11 / 5). The plugin passes that body through unchanged and reports no usage itself, so
  the node's (CPA 8.0.4) usage extraction for a plugin's Responses JSON is the likely gap. Pre-existing,
  not caused by this change; needs a look on the CPA side.
- Offer the v0.3.5-0.3.7 patches upstream (ws-0004 owns upstream offers; `offer/*` branches in the
  fork must stay).

**Cleanup done (01:15Z).** Plugin worktree and branch `ws-0006/plugin-polish` removed (local and
remote; merged in `1a2925b`), local `fleet` fast-forwarded. Ark working files `/root/ws0006`
(config copies holding API keys) and the build dir `cpa-home-build/plugin-ws0006` deleted; laptop
temp files deleted. Kept on purpose: the backup `home-pre-ws0006-20261001T010500Z.db` and tag
`v0.3.7`.
