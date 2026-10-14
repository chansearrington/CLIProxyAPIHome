# Automatic Claude subscription badges

Status: regression tests reproduced; full Ark validation in progress, 2026-10-13.

The user requested plan badges like the existing Codex badge on Home's Upstream credential
list. The existing plan JSON shape and installed panel already support provider-independent
badges; no panel, database schema or CPA node contract change is required.

## Verified source of truth

All eight live Anthropic OAuth profiles returned HTTP 200. Only plan fields were projected;
account IDs, organization names and tokens were not printed or saved in this repository.

- Gmail, HypeSports, Chip, Developer, Moxy and Lara: organization `claude_max`, rate-limit tier
  `default_claude_max_20x` → **Max 20x**.
- Account 82: organization `claude_max`, rate-limit tier `default_claude_max_5x` → **Max 5x**.
- Microsoft: organization `claude_enterprise` → **Enterprise**.

The collector previously called the profile endpoint but only validated its structure.
`internal/quota/providers.go` now derives the existing `QuotaPlan` from organization type,
rate-limit tier and individual account flags. Enterprise/Team organization plans take
precedence over individual flags. Unknown Max multipliers show Max without inventing a
multiplier; unrelated unknown plans remain unlabeled. Recognized Pro and explicitly identified
Free plans are also supported. No plan is inferred from quota sizes, billing amount, email or
Microsoft account ownership.

Failed or malformed profile responses preserve the last verified plan while fresh usage
windows are stored as partial. A successful, valid profile with unknown plan clears an older
plan rather than mislabeling an account that changed subscriptions. Plan badges describe the
subscription independently of quota availability and do not affect priority or user access.

## Validation

The original code fails two new assertions: a Team profile produces no persisted plan, and a
429 profile failure drops the previous Max 20x plan. The updated quota package passes these
checks plus 15 plan parsing cases. Full Home tests and compile run only inside the Ark's
`golang:1.26-bookworm` container with a 3 GB executable tmpfs.

All 64 retained panel assets match the live server byte-for-byte. The installed Upstream
chunk `5468.a4fd9b7e87.js` normalizes generic plan metadata and renders its name without a
Codex-only condition. This exact bundle will be preserved in the Home image.

## Deployment guards

Standing deployment autonomy in ORCHESTRATION.md applies. Review the source PR with
agent-os's local review procedure and resolve all MUST findings. Refresh origin/fleet before
building, preserve all live patches and all 64 panel files, acquire/renew all four fleet
locks, and take a fresh detached integrity/SHA256-verified SQLite backup. Retain the current
immutable Home image for image-only rollback; never restore old OAuth tokens from a stale DB.

After cutover: collect plan snapshots for all eight Claude credentials, verify each badge
against its provider profile in the actual panel, check native completions for all six client
keys, preserved access/priorities, all five nodes healthy with required plugin reports, and
release all locks. On failed acceptance, roll the image back before investigating.
