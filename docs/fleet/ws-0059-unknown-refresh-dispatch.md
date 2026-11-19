# ws-0059 persisted unknown-refresh dispatch fence

## Change

`internal/cliproxy/auth/manager.go:2091` recognizes persisted `refresh_outcome_unknown` alongside transient refresh failures in the shared dispatch predicate. The runtime-only boolean remains intentionally unpersisted. This reaches repository indexes, runtime projections, selectors and quota collectors through existing callers; no payload/schema/config change.

`internal/cluster/refresh_dispatch_reload_test.go:11` persists a fake unavailable Codex credential whose retry deadline has expired, reloads it without the transient flag, and checks the effective fence, repository/record indexes, runtime projection and real selector refusal. No real account data.

## Evidence

The new test first failed against the 1.1.10 integration's unchanged predicate: persisted unknown outcome did not block dispatch; repository index lost refresh fence. Same predicate exists on fleet248ebb9.

Standalone fleet checks on Ark in golang:1.26-bookworm, source `/mnt/user/appdata/cpa-home-build/src/ws0059-m2`, using run-go.sh:

- `gofmt -l internal/cliproxy/auth/manager.go internal/cluster/refresh_dispatch_reload_test.go`: no output.
- `go test -vet=off -count=1 -run 'TestUnknownRefreshOutcomeBlocksDispatchAfterReload|TestRefresh' ./internal/cluster ./internal/cliproxy/auth`: PASS; cluster0.926s, auth0.022s.
- `CGO_ENABLED=1 go build -o /out/ws0059-m2-home ./cmd/home`: exit0.
- `git diff --check`: clean.

No full standalone suite or PostgreSQL run, no image build, no production change. Integrated 1.1.10 gets its own full suite. The PR requires independent review before merge. Model: gpt-6.1-sol[1m].
