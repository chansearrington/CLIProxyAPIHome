# Automatic Claude subscription badges

Status: deployed and verified — all eight Claude plan badges populated; account 82's Max 20x upgrade verified 2026-10-14 10:55 UTC. All locks released.

The user requested plan badges like the existing Codex badge on Home's Upstream credential
list. The existing plan JSON shape and installed panel already support provider-independent
badges; no panel, database schema or CPA node contract change is required.

## Verified source of truth at deployment

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
payload against its provider profile and the served panel renderer, check native completions for all six client
keys, preserved access/priorities, all five nodes healthy with required plugin reports, and
release all locks. On failed acceptance, roll the image back before investigating.

## Completed deployment

- PR #16: https://github.com/chansearrington/CLIProxyAPIHome/pull/16. Local adversarial review
  at `5fa863af01d3723e40024d1b5d475a0ee3ddbc5f` reported zero MUST, SHOULD or NICE findings,
  with synchronous validation. Reviewed parser/failure behavior at source level; Go results
  were independently executed by the author on the Ark.
- Merged source: `4e35cd463b3c0d9aea442ee01706cd55ae458f1d`. Refetched origin/fleet and verified
  the build source was its current head. The merged-tree full Home suite and compile passed
  again in the Ark container.
- Image: `cpa-home:1.1.0-claude-fleet-4e35cd4`, immutable ID
  `sha256:6c082596440ce369d22004b8cbbfb24dfa45500671adc12feacd8c382a1fb817`, Linux/amd64 CGO build.
- Home cutover: `2026-10-14T00:09:15.214051Z`; container started `00:09:16.558616628Z`.
  No CPA node executable or service was restarted.
- Backup: `/mnt/user/appdata/cpa-home/data/backups/home-pre-claude-plan-badges-20261014T000106Z.db`;
  **5,581,705,216** bytes, integrity `ok`, mode `0600`, SHA256
  `88239929893ca5615188323f8ceda9bf3b1ef071a5a377d5453b64fd5f0ff088`.
- Private compose backup: `/mnt/user/appdata/cpa-home/backups/compose-pre-claude-plan-badges-20261014T000915Z.yml`.
- Image-only rollback: restore the saved compose image reference and recreate service `home`
  using `cpa-home:1.1.0-claude-fleet-16e6971`, immutable ID
  `sha256:a0622434ac0107c827f756a16a4bc0824c69673ba13e0057075d5a1589d20fc0`.
  Retain the latest database and OAuth tokens. No rollback was needed.

### Live acceptance

- All eight Claude snapshot plans match the independently read Anthropic profiles listed
  above. Their existing account IDs and settings remain intact.
- All six keys completed native Codex requests with HTTP 200 and `response.completed`.
  Home records 348751–348756 attribute them to the original Codex UUID, preserving the
  prior identity-preserving reauthorization.
- Follow-up native Claude requests returned HTTP 200 and `message_stop` for keys 1–6.
  Home records 348765, 348766, 348769, 348770, 348773 and 348774 all select **Chip**, consistent
  with the available shared account and unchanged priorities. Copilot, Grok, Antigravity
  Gemini and Codex also completed successfully in this check.
- Credential settings, priorities, client keys and channel bindings matched the pre-cutover
  fingerprint. All five CPA nodes remain healthy with required plugin reports `reported_ok`.
- All **64** served panel assets match the preserved image sources byte-for-byte after the
  restart. The unchanged renderer reads each generic `plan.name` and `plan.premium` payload.
  Native browser verification was attempted but the computer-use runtime could not bind
  Edge's window (`cgWindowNotFound` / app-launch errors). No visual screenshot verification is
  claimed. API badge payloads and the exact served rendering code were verified instead.
- All four fleet locks were renewed throughout and released. Private logs and sanitized
  acceptance results are under `/private/tmp/cpa-plan-*` on the Mac and
  `/mnt/user/appdata/cpa-home-build/ws0022/` on the Ark.

Refresh **Upstream → Account Credentials** to load the plan badges. They are automatic,
independent of quota exhaustion, and updated by Home's regular quota/profile collection.

## Account 82 upgrade — 2026-10-14

After the user upgraded `acct-72bae8`, Home's automatic collection at
`2026-10-14T10:51:27.700888622Z` persisted **Max 20x**, premium true. An independent live
Anthropic profile read returned HTTP 200, organization `claude_max`, rate-limit tier
`default_claude_max_20x`, and subscription status `active`. No manual plan override, new
credential, configuration write or restart was needed.

The original UUID `acct-72bae8` remains enabled at priority **8**;
binding **15** still connects it to shared group **1**, included by all six current client
keys. Independent current profile reads returned HTTP 200 and active subscriptions for
all eight accounts. The current plan list is seven **Max 20x** accounts and Microsoft's **Enterprise**.
The earlier Max 5x deployment observation above is retained as history.

The fresh usage snapshot reported 0% session and weekly usage. Separately, the credential
still carried an earlier Anthropic HTTP-429 routing cooldown through `2026-10-14T15:20:00Z`.
Plan verification does not establish successful inference, and no cooldown was cleared as
part of this read-only plan check.

Follow-up: the earlier routing cooldown was cleared at 11:00:54 UTC during the subsequent
investigation. Real Sonnet/Opus requests then verified 82 is selected ahead of Microsoft;
all six ordinary keys correctly selected higher-priority Gmail after its reset. See the
[routing verification record](ws-0023-82-upgrade-routing-cooldown.md).

## Vanderblooms and Hyper Rally upgrades — 2026-10-15

Independent Anthropic profiles confirm both `acct-03a662` and
`acct-c9c2c0` are now `default_claude_max_20x`. Scoped on-demand quota collection
updated their persisted badges to **Max 20x**, premium true, at 15:30:55–56 UTC. These
badges are automatic; no manual override or code change was needed. All nine shared
Claude subscriptions now have verified Max 20x plans; Microsoft remains Enterprise.
See [ws-0028](ws-0028-max20-email-labels.md) for the backup and verification record.
