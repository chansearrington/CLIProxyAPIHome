# FLEET.md — how this fork is used by the agent-os fleet

This is Chanse Arrington's fork of CLIProxyAPIHome. It runs one instance of Home on the Ark (an
Unraid server) as the central model router for the agent-os fleet. Everything upstream's
`AGENTS.md` says still applies; this file only adds the fleet-specific rules.

## Branches

- `main` and `dev` mirror upstream (`router-for-me/CLIProxyAPIHome`). Never commit fleet changes to
  them; fast-forward them from `upstream` only.
- `fleet` is the deployed line. It is `dev` plus the fleet's own fixes, rebased forward as upstream
  moves. The live image on the Ark is always built from a commit on this branch.
- A change meant for upstream gets its own `feat/…` or `fix/…` branch off `dev`, and a PR against
  upstream `dev`. Open upstream PRs from this fork today: #115, #116, #123 (ws-0002 plugin-login fix) and #124 (ws-0004
  plugin model-discovery fix, fleet commit `95ba2db`).

## Building and testing — the Ark only, always inside Docker

- No Go toolchain is installed on the laptop, and none should be. `go build`, `go vet`, `go test`
  and `gofmt` run only inside the `golang:1.26-bookworm` container on the Ark.
- Source, Go build cache, module cache and an output dir live on the Ark's data array under
  `/mnt/user/appdata/cpa-home-build/{src,gocache,gomod,tmp}` and are bind-mounted into the
  container by `/mnt/user/appdata/cpa-home-build/run-go.sh '<command>'`. That keeps every build
  artifact off the Ark's small Docker disk image (measured 2026-09-16: a full gofmt + vet + test +
  compile run grew the Docker disk by zero).
- Inside the container `/tmp` is an exec-enabled tmpfs (`--tmpfs /tmp:exec,size=3g`; 1g ran out of
  room linking cgo test binaries in parallel on 2026-09-30, measured in ws-0002), not a bind
  mount. Go's test temp dirs on the bind-mounted Unraid array (a FUSE share) hit "directory not
  empty" on cleanup and produced three false FAILs in `internal/cluster` on 2026-09-16; on tmpfs the
  full suite is clean. Docker's default tmpfs is `noexec`, which breaks `go test` outright, so the
  `exec` option is required. Write compile output to `/out/<name>` (the array), never to `/tmp`.
- `go vet ./...` reports exactly one finding on `fleet`: `internal/cluster/refresh.go:172:2:
  unreachable code`. It is upstream's (a duplicated `return` after an if/else), untouched by this
  fork, and present on `dev` too. Do not fix it here; a vet run whose only line is that one passes.
- The Dockerfile compiles with `CGO_ENABLED=1` (glibc-linked). A binary compiled outside the
  Dockerfile must use the same builder image and run on `debian:bookworm`; a static build is a
  deliberate change that must be documented, not an accident.
- The management panel is not in this repo (`internal/managementasset/static/` holds only a
  `.gitkeep`). A deployable image needs the panel mirrored from the running instance first. A binary
  compiled from a bare checkout ships a blank panel and is a smoke test only, never a deploy.
- `docker build` on the Ark is gated on the Ark's Docker disk having enough free space. The gate,
  the tag convention (`cpa-home:<upstream-version>-claude-fleet-<short-sha>`, Chanse 2026-09-24; earlier images used `1.0.72-claude-<line>-<sha>`) and the rollback procedure are
  in the agent-os repo: `docs/runbooks/cpa-home-build-and-rollback.md`.

## Deploying

- The Ark's `cpa-home` container is live for the whole fleet. A new image is deployed only on a
  decided `ship` card in the fleet Inbox, following the runbook above. Never restart or replace it
  ad hoc.
- Never stop the fleet's `cpa-home-node` services on the minis; restart in place only.

## Plugins (Home-managed)

- Plugins reach the nodes through Home's config (`plugins.configs.<id>` with a pinned `store`
  manifest); every node downloads and loads them, so access is fenced with channel groups, not
  per node. The GitHub Copilot plugin is the fork `chansearrington/cliproxyapi-copilot-plugin`
  (ws-0002; its `FORK.md` lists the patches it carries).
- Nodes run CPA **8.0.20** (`0f96f568`, since 2026-10-08 UTC, ws-0011). On CPA ≤ 8.0.4, adding a new
  plugin while nodes ran made every node loop on "installed but not loaded" (CPA issue #6225);
  8.0.5+ carries the fix (`bfa5aed`). It is proven by upstream's regression tests, not yet by a
  live plugin add — watch the first one, and if a node still loops, restart it in place. Bumping
  the version of an already-loaded plugin hot-reloads cleanly with no restart.
- After a node restart, Home's `GET /nodes` shows a plugin as `skipped loaded` (library on disk
  already identical); that is healthy. Check for `loaded` + `reported_ok`, not `installed`.

## Live SQLite maintenance index

- On 2026-10-07, the usage-token-accounting maintenance query was measured at **3.18 s** on
  the Ark's approximately 150,000 usage rows, all already at accounting version 2. It scanned
  the full table every minute despite the existing version index. Home uses one SQLite
  connection; the scan delayed subscription heartbeats beyond the 3 s lifecycle timeout,
  disconnecting nodes and causing Claude Code 503s.
- The live database now has this additional index, applied under all four fleet-locks without
  restarting Home or changing its image:

  ```sql
  CREATE INDEX idx_usage_pending_token_accounting_v2
    ON usage(id) WHERE token_accounting_version <> 2;
  ```

  The parameterized maintenance query uses it, returns the same result, and took **0.027 ms**
  on the first check after creation. A synthetic check also verified that a late legacy row is
  still found. This is a persistent database tuning change, not yet an application migration;
  retain it when restoring or replacing the database, and reassess it when the accounting
  schema version changes. Rollback is `DROP INDEX idx_usage_pending_token_accounting_v2`.
- Pre-change backup on the Ark:
  `/mnt/user/appdata/cpa-home/data/backups/home-pre-pending-token-index-20261007T021207Z.db`,
  3,461,537,792 bytes, full integrity check `ok`, SHA-256
  `595c33ef8bb1baec302286c5719d035120f2a7d3da5a48e9cff4572c4987fc97`.

## Secrets

- Nothing in this repo may contain the Home management password, any node's `home_jwt`, an API
  key, or an OAuth token. Test fixtures use obviously fake values.

## Current verified runtime (ws-0011, 2026-10-08 UTC)

- Home is `cpa-home:1.1.0-claude-fleet-9821c83`, immutable image
  `sha256:dad5e5d84bd9931dfacb1927bb35990f37bbe7487f5252e69f411ddc83b94809`.
  Source integrates upstream 1.1.0 and all 81 fleet patches, plus Claude refresh safety,
  serialized SQLite configuration updates and four read-only dashboard connections.
- All five nodes have verified CPA 8.0.20 binaries, manifests and healthy membership;
  Copilot 0.3.7 remains loaded/reported_ok. Restart-in-place rollout passed each node's native
  Claude/Codex/Copilot completions, protected-key fences and Home usage attribution.
- Gmail is enabled and eligible; HypeSports remains first priority but weekly-exhausted.
  Microsoft remains Chanse-only emergency fallback. Fresh native Claude Code selects Gmail;
  fresh native Codex defaults to CPA, executes a real shell tool and shows no daemon mismatch.
- SQLite's writer remains single-connection. Dashboard reads use a separate bounded read-only
  pool so they cannot queue scheduler/heartbeat operations behind analytics. Four-panel 24-hour
  load passes. Seven-day overview can exceed the unchanged 10-second budget on the large database;
  this was reproduced with the old image too and remains a separate optimization follow-up.
- Retain the verified fresh production backup and each node's `cli-proxy-api.8.0.7.bak`.
  Image-only Home rollback targets immutable ID
  `sha256:7a92e9443088eee7a91461ad623791c8c21277c5aebec6a0de5c10897042981e`.
  Do not restore a stale database over newer rotating OAuth authorizations.
- Evidence: [upgrade record](docs/fleet/ws-0011-upstream-fleet-upgrade.md).
  Plain-language offline guide: [what changed](docs/fleet/ws-0011-updates.artifact.html).
