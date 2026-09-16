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
  upstream `dev`. Open upstream PRs from this fork today: #115 and #116.

## Building and testing — the Ark only, always inside Docker

- No Go toolchain is installed on the laptop, and none should be. `go build`, `go vet`, `go test`
  and `gofmt` run only inside the `golang:1.26-bookworm` container on the Ark.
- Source, Go build cache, module cache and temp dir live on the Ark's data array under
  `/mnt/user/appdata/cpa-home-build/{src,gocache,gomod,tmp}` and are bind-mounted into the
  container. That keeps every build artifact off the Ark's small Docker disk image.
- The Dockerfile compiles with `CGO_ENABLED=1` (glibc-linked). A binary compiled outside the
  Dockerfile must use the same builder image and run on `debian:bookworm`; a static build is a
  deliberate change that must be documented, not an accident.
- The management panel is not in this repo (`internal/managementasset/static/` holds only a
  `.gitkeep`). A deployable image needs the panel mirrored from the running instance first. A binary
  compiled from a bare checkout ships a blank panel and is a smoke test only, never a deploy.
- `docker build` on the Ark is gated on the Ark's Docker disk having enough free space. The gate,
  the tag convention (`cpa-home:1.0.72-claude-<line>-<short-sha>`) and the rollback procedure are
  in the agent-os repo: `docs/runbooks/cpa-home-build-and-rollback.md`.

## Deploying

- The Ark's `cpa-home` container is live for the whole fleet. A new image is deployed only on a
  decided `ship` card in the fleet Inbox, following the runbook above. Never restart or replace it
  ad hoc.
- Never stop the fleet's `cpa-home-node` services on the minis; restart in place only.

## Secrets

- Nothing in this repo may contain the Home management password, any node's `home_jwt`, an API
  key, or an OAuth token. Test fixtures use obviously fake values.
