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

(filled in at the end)
