# Reconnect an existing OAuth account

Status: implemented and verified in isolation, 2026-11-05 UTC — full backend suite and compile pass; panel typecheck/lint/format/build and eleven focused frontend tests pass. Production cutover has not been performed.

The user requested a Reconnect action after discovering that Add credential creates a new
UUID and the current panel does not clearly explain an authentication-disabled account.

## Behavior

The account list displays **Sign-in required** for terminal authentication failures. Open
the account's **View** panel and select **Reconnect**, then sign into the displayed account.
The sheet offers only that account's provider and does not offer upload/import modes.
Closing or restarting the flow cancels the prior reconnect session in Home.

Home's optional `credential_id` selector binds a Claude or Codex login session to the
existing record. A successful callback replaces OAuth token fields and the token-derived Codex subscription tier under the current
credential's DB lock and atomically completes the login session. It preserves the UUID,
creation time, filename, label, routing fields, channel bindings, historical usage and quota
snapshots. Fresh authorization reenables the account and clears authentication/refresh
failures while preserving valid quota cooldowns. Clearing the old refresh lease fences an
earlier in-flight refresh from overwriting the new authorization.

The provider-returned email must match the pinned original account. Codex also requires
the same account/workspace ID. Wrong-account sign-ins, changed/deleted targets, invalid
tokens, cancelled/expired sessions and callback replay cannot replace credentials.
Ordinary Add credential remains unchanged. Other providers do not advertise reconnect yet.
The panel requires the backend capability before exposing the action, protecting older
backends from silently treating reconnect as a new-account flow.

The matching Home Management Center fork uses the existing Home 1.1.1 panel generation at
upstream `3813264`, rather than incorporating the unrelated upstream upgrade. It has its own
`fleet` branch. No database schema change or CPA node payload change is required.

## Validation

All builds and tests ran on Ark in the existing `golang:1.26-bookworm` wrapper. Node 24 and
Bun 1.3.0 binaries were extracted from official Docker images into the isolated workstream
tools directory; nothing was installed on the laptop.

- Full `go test -vet=off ./...` passes, followed by compilation with the generated panel embedded.
  Vet remains excluded for the documented preexisting unreachable-code finding.
- Focused DB tests verify original UUID/history/channel binding/routing preservation,
  reenablement, wrong identity/workspace rejection, cancellation, expiry, deletion and replay.
  Auth tests verify valid credential/model quota cooldowns survive reauthentication.
- Frontend typecheck, lint, formatting and embedded production build pass. Eleven focused
  tests cover the pinned v8 request, cancellation, ordinary-add compatibility, error copy,
  rendered Reconnect action, and capability gating on older backends.
- Knip reports the same two unused exports and seven unused exported types on both the
  untouched panel baseline and the candidate; no new dead-code findings were introduced.
- A separate Home instance with one fake, disabled Claude account reports both new flags,
  issues a reconnect session, cancels it, returns `Authentication cancelled`, and retains
  exactly the same credential IDs. No production tokens were used in this check.
- The computer-use browser host was unavailable; no interactive visual browser verification
  or completed real-provider sign-in is claimed. The two affected real accounts still need
  their owners to sign in after the feature is deployed.

Review found and the change fixes two additional cases: late sign-in URL responses are
cancelled after the sheet closes or the flow is superseded, and reconnect updates a changed
Codex subscription tier while retaining unrelated routing attributes. Regression tests cover
both paths.

## Operations

Live Home remains `cpa-home:1.1.1-claude-fleet-897d9c6`, image
`sha256:c0bd56efb239136db6f2fad35235b59470e147dda48480d1a24fdae5fe925766`.
No upstream upgrade or account mutation is part of this feature preparation.

Private build, test, staging and backup evidence is under
`/mnt/user/appdata/cpa-home-build/{ws0040-reconnect,tmp/ws0040-reconnect}/` on Ark.
Maintenance policy was saved before pausing it; four canonical fleet locks are held with
independent 60-second heartbeats during verification. Restore policy and release locks
before waiting for a deployment decision. A production cutover uses the existing ship-card
procedure, fresh verified backup, original image rollback and native inference acceptance.
