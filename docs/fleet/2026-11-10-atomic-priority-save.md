# Atomic editable-auth field saving — decision 979

Status: candidate only; not deployed. Base: 44a3c27cd10a01bb328fcb9cc33fb140a98b2030.

## Change

The shared editable-auth field handler previously selected a full account before opening its write transaction, then persisted that old snapshot. A concurrent refresh, reconnect or operator update could be overwritten. The selector now identifies the account only; supplied fields are applied to the current row using Repository.MutateAuth inside the existing auth/policy transaction. Both legacy and v8 routes share this handler.

Explicitly requested fields remain last-writer-wins. Priority accepts integer, integer string or null; invalid types and nested priority paths fail without persisting auth/policy changes. Zero, omission and null are distinct.

## Checked evidence

Sol builder; independent Opus G05 reviewer. Parent checked the actual output, diff and source hashes.

Ark Docker: golang:1.26-bookworm, array-backed caches, exec tmpfs, final runs with networking disabled. Six source/document hashes matched the tested copies. No live credentials, configuration or database were used.

- Original base plus the new regression: all ten controlled interleavings FAIL with `PATCH overwrote unrequested current auth state` (expected baseline failure; management package 0.603s).
- Candidate: `go test -v -count=1 -timeout=180s ./internal/cluster/management -run "Test(PatchAuthFile|ApplyOAuthFieldPatch)"` PASS, management package 1.701s: 18 top-level tests plus 56 subtests.
- `CGO_ENABLED=1 go build -o /out/test-output ./cmd/home` PASS; output removed.
- gofmt and `git diff --check` PASS.
- Independent Opus review: PASS, no confirmed bugs. Checked transaction locking, rollback, identity/version/event preservation, mixed patches, strict priority validation and shared route/caller coverage.

The regression commits refresh/reconnect/disable/quota/operator changes after selector read and before PATCH transaction. It compares the resulting complete account with the newer account plus only requested fields, and verifies auth/policy versions. It uses deterministic query hooks, not timing sleeps. Refresh uses the injectable Antigravity transport and shared real finalization path; quota collection itself is not exercised.

The initial fixture failed because the Claude refresher ignored its mocked transport and sent two OAuth requests with obviously fake grants. Both failed; no real credential was used. The fixture was corrected and external networking disabled before baseline and final runs. That failed attempt is not counted as a pass.

## Limits and release gate

Not run: full suite, race detector, PostgreSQL execution, real-provider refresh or live candidate routing. SQLite was exercised. The reviewer confirmed that PostgreSQL locking is retained by construction, not by an executed PostgreSQL test.

Separate status PATCH and import whole-record writers retain their pre-existing stale-write risk; this change fixes the shared field PATCH, not every writer. Those paths need a separate follow-up.

No image build, restart or deployment is authorized by this source PR. Follow FLEET.md and the existing agent-os release process: decided ship card, freshly proven independent non-relay recovery, exact image identities, preservation checks and agreed cutover minute. Automatic ordering remains off until safe release and live verification.
