# CPA account audit — November 3, 2026

Status: complete — 18 credentials inspected; all 17 enabled accounts rechecked; live Claude,
Codex, Copilot and Grok routing succeeds. Nine credential errors are provider quota limits;
eight credentials are active; one Antigravity credential remains intentionally disabled.

Checked November 3 around **10:23–10:28 AM Central (CST)**. Reset times below are Central,
rounded to the nearest minute and are estimates supplied by the provider. Normal activity
can consume recovered quota before the operator next checks the panel.

## What the labels mean

`error` records a failed model request. It does not, by itself, mean a broken login or an
account that cannot serve any models. Read its HTTP code/message alongside quota and current
per-model availability. All nine current top-level error messages here are HTTP 429 / usage
limit errors, independently corroborated by fresh provider quota reads. No enabled account
showed a 401/expired-token authentication failure during this audit.

`Partially collected` can be a quota observation from inference response headers that does
not contain every optional quota detail. It is distinct from failed authentication. Hype's
Codex account continues to receive partial header observations while successfully serving
requests. A normal full quota probe succeeded during this audit.

## Every account

| Provider | Account | Current finding | Action / next reset |
| --- | --- | --- | --- |
| Claude | `chanse.arrington@gmail.com` | Weekly quota exhausted; 429 | Wait until approximately Nov 3, 7:00 PM |
| Claude | `chanse@hypesports.live` | Weekly quota exhausted; 429 | Wait until approximately Nov 3, 3:00 PM |
| Claude | `character82@gmail.com` | Weekly quota exhausted; 429; also retains old Sonnet 3.7 404 | Wait until Nov 7, 9:00 PM; use a supported model |
| Claude | `chip@byfca.org` | Weekly quota exhausted; 429 | Wait until approximately Nov 3, 3:00 PM |
| Claude | `developer@hypesports.live` | Active; approximately 15% weekly quota remains | Working now; current preferred eligible shared Claude account |
| Claude | `moxy.kit@outlook.com` | Weekly quota exhausted; inference 429 despite separate extra-usage allowance | Wait until Nov 6, 3:00 AM; extra-usage telemetry does not establish that the refused request is eligible |
| Claude | `the.lara.vance@gmail.com` | Active; approximately 54% weekly / 32% five-hour quota remains | Working; five-hour window resets about Nov 3, 11:40 AM |
| Claude | `the.vanderblooms@gmail.com` | Weekly quota exhausted; 429; also retains old Sonnet 3.7 404 | Wait until Nov 4, 3:00 AM; use a supported model |
| Claude | `hyper.rally@outlook.com` | Active; approximately 26% weekly / 87% five-hour quota remains | Working; five-hour window resets about Nov 3, 11:30 AM |
| Claude | `chansearrington@hotmail.com` | Active; five-hour and weekly utilization reported 0% | Working; weekly reset Nov 7, 3:00 PM |
| Claude | `carringt@microsoft.com` | Active; monthly extra-usage allowance available | Working state; remains private MacBook fallback, not shared with agents |
| Codex | `chanse.arrington@gmail.com` | Weekly quota exhausted; usage-limit error | Wait until Nov 8, 2:39 PM |
| Codex | `chanse@hypesports.live` | Active; approximately 97% weekly quota remains | Working now; current preferred eligible Codex account |
| Codex | `developer@hypesports.live` | Weekly quota exhausted; usage-limit error | Wait until Nov 4, 12:36 AM |
| Codex | `chansearrington@hotmail.com` | Weekly quota exhausted; usage-limit error | Wait until Nov 4, 8:09 AM |
| Copilot | `carringt_microsoft` | Active; healthy Enterprise quota | Live request passes; private access scope preserved |
| xAI / Grok | `chanse.arrington@gmail.com` | Active; live Grok request passes, but zero-dollar monthly-spend window is labeled exhausted | Quota telemetry does not reflect this working inference route; do not re-login or treat the badge alone as a failed account |
| Antigravity | `chanse.arrington@gmail.com` | Already disabled; old quota collector reported an unsupported/invalid response shape | Preserved as disabled; old quota snapshot is not current availability evidence |

The additional configured provider is `agent-os-home-canary`, a synthetic test provider with
unsupported quota collection, not another user account requiring a login.

## Verification and changes made

- Inspected Management credential metadata, every saved quota window, all per-model error
  states through a read-only SQLite projection, node membership and fleet version reports.
  No OAuth tokens, API keys, management password or raw credential downloads were printed.
- Triggered one normal on-demand quota round for the 17 enabled Claude/Codex/Copilot/xAI
  accounts. The previous quota-endpoint 429 for account 82 resolved on this round. Fresh
  results corroborate the exhausted accounts rather than just reusing their old error badges.
- Native requests through the MacBook's existing loopback CPA node returned HTTP 200:

| Model | Actual account | Home usage record |
| --- | --- | --- |
| Claude Sonnet 5.5 | Developer Claude | 2302572 |
| Sol 6.1 | HypeSports Codex | 2302579 |
| Grok 4.7 Build Fast | Gmail xAI | 2302573 |
| Copilot Sol 6.1 | Microsoft Copilot | 2302574 |

- All five nodes are healthy with plugin reports `reported_ok`, running CPA 8.0.53 according
  to Home. Existing priority order, provider identities, enabled/disabled states and client
  scopes were retained. No Home configuration, image, node binary, native login or credential
  history was replaced. No reset credits were consumed and no paid plan/billing setting changed.

## Residual model errors and fixes

Account 82, Developer and Vanderblooms retain a historical 404 for
`claude-3-7-sonnet-20250219`. That is a model-not-found result, not an OAuth failure.
Use the current `claude-sonnet-5-5` / `claude-opus-5-5` models. Developer is currently active
and serving current models despite this old per-model record. The bounded recent-traffic read
shows current Opus 5.5 successes; no matching recent account traffic was found for Vanderblooms.
An old canceled Astra request on Codex Gmail is also retained as HTTP 499; its present Sol
block is independently confirmed real weekly exhaustion.

There is no safe reason to erase provider refusal evidence solely to make a badge green.
Home's deployed collector automatically checks quota blocks about hourly and after saved
reset times, and clears stale quota blocks only when a provider read confirms quota is back.
A future provider refusal can block it again normally. Retired-model state and the Grok
zero-limit telemetry mismatch are separate observability issues from the genuine exhausted
weekly windows; neither justifies rotating every account's login or overriding quotas.

Private sanitized evidence: `/tmp/cpa-account-audit-20261103/`, including baseline/final
account summaries, quota observations and live routing/attribution proof. No public messages
or diagnostic uploads were sent.
