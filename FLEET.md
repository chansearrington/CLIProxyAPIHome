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
  upstream `dev`. Open upstream PRs from this fork today: #115, #116 and #123 (ws-0002 plugin-login fix).

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
- Adding a new plugin while nodes run makes every node loop on "installed but not loaded"
  (CPA issue #6225); recover with an in-place restart of each node. Bumping the version of an
  already-loaded plugin hot-reloads cleanly with no restart.

## Secrets

- Nothing in this repo may contain the Home management password, any node's `home_jwt`, an API
  key, or an OAuth token. Test fixtures use obviously fake values.
