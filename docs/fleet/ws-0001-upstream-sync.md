# ws-0001 — Upstream sync

Workstream brief for the `fleet` fork of CLIProxyAPIHome. Created 2026-09-24.

- Branch: `ws-0001/upstream-sync` (based on `fleet` @ `9d8bf4e`)
- Worktree: `.claude/worktrees/ws-0001-upstream-sync`
- herdr space: `CLIProxyAPIHome | ws-0001 | Upstream sync` (agent alias `cpahome-ws-0001`)
- Lands on: `fleet` (fast-forward when done). Nothing here goes to upstream.

## Goal

Bring the fleet's deployed line up to the latest upstream Home, confirm whether the fork is still
needed, and leave an image on the Ark ready for a `ship` card. Deploying is NOT part of this
workstream — it happens only on a decided `ship` card (see `FLEET.md`).

## Facts at start (verified 2026-09-24)

- Fork point (merge-base of `fleet` and `upstream/dev`): `9110602` (merge of upstream #113).
- `fleet` carries 9 commits on top of that: the Claude unified rate-limit fix (upstream PR #115),
  the allowed_warning de-preference (upstream PR #116), and FLEET.md docs.
- `upstream/dev` is 3 commits ahead of the fork point: `26a0afb` (Devin PKCE OAuth + Devin model
  registry) = tag **v1.0.73** (released 2026-09-15), then `c1b9580`/`04fac5f` (registry: forward
  native model capabilities). `upstream/main` is 2 behind `upstream/dev`.
- Only one file is changed on both sides: `internal/cliproxy/auth/types.go`.
- Upstream PRs #115 and #116 are still OPEN → upstream has not absorbed the fork's fixes.
- CPA SDK pin (`go.mod`) is `v7.2.83` on both `fleet` and `upstream/dev`. Latest CPA release is
  v7.3.16 (2026-09-24). The fleet's nodes run CPA 7.2.154. Bumping the pin is upstream's call and
  is NOT in scope unless it turns out to be required for a model the fleet needs.
- Known upstream vet finding `internal/cluster/refresh.go:172:2: unreachable code` — leave it.
- Known open defect on the fork: agent-os issue #717 (the de-preference is dead code on the
  scheduler fast path). Not fixed here; it is its own workstream after this one lands.

## Acceptance criteria (all must be verifiable, not asserted)

1. `ws-0001/upstream-sync` is `upstream/dev` (at `04fac5f` or newer, recorded by SHA) plus the
   fork's 9 commits, rebased in order, with no merge commits. `git log --oneline upstream/dev..HEAD`
   shows exactly the fork's commits.
2. `types.go` conflict (if any) resolved so both the Devin additions and the fork's
   `RateLimitWarnings` handling are present. Show the resolved hunk in the close-out.
3. On the Ark, inside the `golang:1.26-bookworm` container via
   `/mnt/user/appdata/cpa-home-build/run-go.sh`: `gofmt -l .` prints nothing, `go vet ./...`
   prints only the known refresh.go line, `go test ./...` is fully green (with `/tmp` on tmpfs),
   and `go build -o /out/ws-0001-check ./cmd/home` succeeds. Paste the tail of each.
4. A written answer to "does upstream now remove the need for the fork?" — with evidence: for each
   fork commit, either "still needed, upstream has nothing equivalent (checked <where>)" or
   "superseded by upstream <sha>, dropped".
5. A written answer to "what does 'newest models' need?" — which models the fleet is missing,
   whether they arrive from Home's registry, the CPA SDK pin, or the CPA node version on the
   minis, with file/line evidence. If the answer is "CPA node upgrade on the minis", say so and
   stop; that is agent-os work.
6. `docs/management/` unchanged unless a Management API surface changed in the rebase (then
   updated per AGENTS.md).
7. `fleet` fast-forwarded to the result and pushed to `origin/fleet`; `main` and `dev` fast-forwarded
   to `upstream/main` / `upstream/dev` and pushed (mirror rule in FLEET.md). Old worktree removed.
8. Image built on the Ark per `agent-os/docs/runbooks/cpa-home-build-and-rollback.md`, tagged
   `cpa-home:1.0.73-claude-fleet-<short-sha>` (note: 1.0.73 now), with the panel mirrored in. The
   disk-space gate in the runbook is checked BEFORE the build. A `ship` card raised in the fleet
   Inbox with the tag and rollback tag. No container restart.

## Tasks

Sequential unless marked ‖ (can run in parallel with its sibling).

1. Rebase: `git rebase upstream/dev` on this branch; resolve `types.go`; confirm criterion 1.
2. ‖ Fork-need audit (criterion 4): diff each fork commit against upstream/dev; check PRs #115/#116
   review threads for anything merged elsewhere.
3. ‖ Newest-models audit (criterion 5): compare Home's registry + the CPA SDK pin + the minis' CPA
   version against the models Chanse expects; produce the table.
4. Ark verification (criterion 3). Needs task 1. Only inside the container; never on the laptop.
5. Fast-forward `fleet`, `main`, `dev`; push (criterion 7).
6. Build the image + raise the ship card (criterion 8). Needs 4 and 5.
7. Close-out: append results to this doc; remove the worktree; close the herdr space.

## Out of scope / follow-ups

- **ws-0002 (to be scoped): GitHub account as models across the fleet.** Neither Home nor CPA
  has a GitHub/Copilot provider today (Home `internal/auth/`: antigravity, claude, codex, devin,
  kimi, meta, vertex, xai; CPA repo code search: 0 hits for "copilot"). This is a new provider,
  mostly in CPA (it makes the API calls); Home would only distribute the credential. Scope it
  against CPA first; decide whether to build it or wait for upstream.
- **ws-0003 (queued): scheduler-path fix for agent-os #717.** Base on the post-sync `fleet`.
- CPA node upgrades on the minis (7.2.154 → 7.3.x) are agent-os work, not this repo.

## Close-out

### Tasks 1-3 (2026-09-24, laptop only; nothing touched on the Ark, nothing pushed)

**Task 1 — rebase (criteria 1, 2): done, verified.**

- `upstream/dev` at rebase time: `04fac5f31514604dd80d314736a8c1f4ffceeab5` (merge of upstream
  #118, "registry: forward native model capabilities"). `upstream/main` is at `26a0afb`, 2 behind.
- `git rebase upstream/dev` applied all fork commits with **no conflicts**. `git log --oneline
  upstream/dev..HEAD` shows exactly the fork's commits, 0 merge commits. The count is **10**, not
  the 9 in "Facts at start" — the 10th is this brief's own commit.
- `types.go` needed no manual resolution: upstream's hunk (a `meta-api-key` case in `indexSeed`,
  now line 346) and the fork's hunk (`RateLimitWarnings` field at line 63, `RateLimitWarning` type
  at line 130, `Clone` copy) are in different regions of the file. Both are present at HEAD. The
  fork's rebased diff against upstream/dev is byte-identical to its pre-rebase diff against
  `9110602` for every file including `types.go`.
- Criterion 6: upstream's own Devin OAuth commit changed `docs/management/api.md` alongside its
  handlers. The fork's commits touch no Management API surface. Nothing to update.

**Task 2 — fork-need audit (criterion 4): upstream does NOT remove the need for the fork.
All 10 commits still needed.**

| fork commit (post-rebase) | verdict | evidence |
|---|---|---|
| `7c6634b` fix(auth): honour Claude unified rate-limit resets | still needed | `git grep -i anthropic-ratelimit upstream/dev` = 0 hits; `claude_ratelimit.go` absent upstream; upstream `result.go:1044` `parseUsageRetryHints` still lacks the headers arg |
| `02caa02` test: doc comment name | still needed | edits fork-only `claude_quota_scope_test.go` |
| `477309d` fix(cluster): Claude headers survive usage sanitizing | still needed | upstream `quota_ingestion.go` still allow-lists only `X-Codex-*`; fork adds the `Anthropic-Ratelimit-Unified-*` allowlist at lines 616-629 |
| `387cef3` docs: determinism scope comments | still needed | comment-only edits to fork-only code |
| `bd09641` feat(claude): de-prefer on allowed_warning | still needed | `git grep -i 'RateLimitWarning\|allowed_warning' upstream/dev` = 0; upstream `selector.go:223` `collectAvailableByPriority` returns 3 values, fork returns 4 (`warned`) |
| `e5b8d6b` fix(claude): clear per-window mark | still needed | depends on `RateLimitWarnings`, absent upstream |
| `d9c7b7c` docs: correct race claim | still needed | comment fix in fork-only selector code |
| `027aa1f` docs: FLEET.md (+ CLAUDE.md `@FLEET.md` line) | still needed | upstream has no FLEET.md. Note: this is the one commit that edits a file upstream owns (`CLAUDE.md`); no conflict this cycle |
| `b38a62c` FLEET.md tmpfs/out/vet notes | still needed | fleet-only |
| `d360e01` this brief + `.gitignore` `!docs/fleet/` | still needed | fleet-only |

- Upstream PRs #115 and #116: both **OPEN**, `REVIEW_REQUIRED`, `MERGEABLE`, **0 reviews, 0
  review comments, 0 issue comments** (checked via `gh pr view` and the raw
  `pulls/N/reviews`, `pulls/N/comments`, `issues/N/comments` endpoints). Searches of upstream PRs
  and issues for "rate limit", "allowed_warning", "unified" find nothing else. Maintainers merged
  their own #118 on 2026-09-17, so they are active but have not looked at ours. The PR branches
  are still based on `9110602`; GitHub reports them mergeable, so no forced rebase is required.
- `go.mod` / `go.sum`: identical to upstream/dev. Both pin CPA SDK `v7.2.83`, no `replace`.

**Task 3 — newest-models audit (criterion 5): no Home change, no SDK bump, no CPA node upgrade
is needed for "newest models". The catalog is a remote feed both sides already pull.**

How the model list actually flows (Home → node, never node → Home):

1. Home embeds `internal/registry/models/models.json` as a boot-time **fallback only**
   (`internal/registry/model_updater.go:27-28`, parsed in `init()` at 68-73). At startup and every
   3 hours (`model_updater.go:18-20`, `78`, `91-103`) it re-downloads the live catalog from
   `https://raw.githubusercontent.com/router-for-me/models/refs/heads/main/models.json` (fallback
   `https://models.router-for.me/models.json`, lines 22-25) and hot-swaps every credential's model
   list without a restart (`internal/home/runtime.go:340` starts it;
   `internal/home/auth_apply.go:118-146` re-registers on change;
   `internal/home/models.go:171-330` builds each credential's list from `registry.GetClaudeModels()`
   etc.). This was already true on the deployed line (`9d8bf4e`).
2. The node's `/v1/models` comes **from Home**: RESP `get {"type":"models"}` →
   `internal/respserver/get/default.go:51-52,61-73` → `get/models.go:36-136` `buildModelsJSON`.
   Dispatch is gated on Home's registry (`internal/home/runtime.go:952` → `1111-1124`
   `registry.LookupModelInfo`).
3. In Home mode the CPA node **disables its own catalog refresh**: CPA `cmd/server/main.go:763-778`
   at v7.2.159, `modelCatalogUpdaterPlan` returns `startModels = !homeEnabled` and logs "Home mode:
   remote models.json updates disabled". So the node binary version does not decide which model ids
   exist.
4. Home imports no registry code from the CPA SDK module (only `sdk/pluginapi`, `sdk/pluginstore`,
   `sdk/cliproxy/auth`, `sdk/config`, `sdk/api`, `sdk/auth`, `sdk/pluginhost`), so the `v7.2.83` pin
   is irrelevant to the model list.

Model coverage (Remote = router-for-me/models `main` on 2026-09-24; Home embedded = this repo at
HEAD; CPA columns = that tag's own embedded `models.json`, informational only since nodes do not use
it in Home mode):

| model id | Home embedded | Remote feed | CPA 7.2.159 (minis, actual) | CPA v7.3.16 | arrives via | action |
|---|---|---|---|---|---|---|
| claude-fable-5-1 | no | yes (since 2026-09-02, `c3b5f27`) | yes | yes | remote feed → Home | none; proven live 2026-09-09 (agent-os STATE.md line 118) |
| claude-opus-5-5 | no | yes (since 2026-09-22, `02ea1f8`) | no | yes | remote feed → Home | none; Home picks it up within 3 h of that commit. Not yet referenced anywhere in agent-os |
| claude-sonnet-5 / claude-opus-5 / claude-fable-5 / claude-haiku-4-5-20251001 | yes | yes | yes | yes | both | none |
| gpt-5.6-sol / terra / luna, gpt-5.5 | yes | yes | yes | yes | both | none |
| gpt-6-astra | no | yes (2026-09-04) | yes | yes | remote feed | none |
| gpt-6-sol / gpt-6-luna | no | yes (2026-09-22) | no | yes | remote feed | none |
| gemini-3.8-flash, gemini-3.8-flash-high | no | yes | yes | yes | remote feed | none |

Two facts corrected against the brief's "Facts at start": the minis run **CPA 7.2.159**
(agent-os STATE.md line 74, PR #682, 2026-09-16), not 7.2.154; and "newest models" is gated by
neither Home's binary, the SDK pin, nor the node version.

Caveats:
- The remote feed has also **removed** ids the embedded file still carries: `gpt-5.4`,
  `gpt-5.4-mini`, `gpt-5.3-codex-spark` (codex), `gemini-3-flash-agent`,
  `gemini-3.5-flash-low/extra-low` (antigravity). After a successful refresh those vanish from the
  fleet catalog. agent-os references `gpt-5.4-mini` about 20 times and `gpt-5.4` about 11 times;
  if anything is pinned to them it will break. That is a feed decision, flagged for agent-os.
- Not verified from the laptop: that the Ark's container actually reaches the feed today. The
  2026-09-09 Fable 5.1 proof is strong indirect evidence (the deployed line's embedded file lacks
  that id). Direct check, read-only, when on the Ark:
  `docker logs cpa-home 2>&1 | grep "model refresh"` — expect "startup model refresh completed
  from https://raw.githubusercontent.com/router-for-me/models/..."; "fetch failed from all URLs"
  means egress is blocked and Home is stuck on the embedded fallback.
- "Id exists in the catalog" is not "the node's executor handles a brand-new model family". Nothing
  on the fleet's wanted list is in that situation.
