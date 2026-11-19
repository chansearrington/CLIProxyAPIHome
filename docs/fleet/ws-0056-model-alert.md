# Codex model availability alert

## Purpose and boundary

This is an alert, not a model updater. Once an hour while the MacBook is awake, it asks OpenAI which Codex models each enabled account offers and compares them with Home's currently served model IDs. It never edits Home configuration, the maintainer catalog, priorities, access, or node services. There are no new Python packages or Go changes.

The MacBook is the scheduler because both intended readers are Claude Code sessions there. Running on the always-on Ark would still need a sleeping-Mac delivery hop. A single MacBook launchd job is smaller and avoids an undeliverable Ark alert queue. Sleep postpones checks; there is no claim of hourly coverage while the MacBook is asleep or logged out.

## Request contract and evidence

The Ark collector uses these **v8-only** management routes:

1. `GET /v8/management/credentials`: enumerate every enabled Codex OAuth credential. Do not filter on `status=active` or `unavailable=false`: an account with exhausted generation quota may still list models.
2. `GET /v8/management/quota/credentials?provider=codex&limit=200&offset=0`: join saved `plan.name` by `credential_id`, following pagination. This is a read, not a quota refresh. The credentials list itself does not expose a plan. Unknown/missing plans fail that account's check rather than inventing a tier.
3. `GET /v8/management/models?scope=available`: compare against Home's live available model IDs, not a bundled or fetched static catalog.
4. `POST /v8/management/requests/api-call`, once per enabled Codex credential, with this body:

```json
{
  "auth_index": "credential-id-from-the-v8-list",
  "method": "GET",
  "url": "https://chatgpt.com/backend-api/codex/models?client_version=0.167.0",
  "header": {
    "Authorization": "Bearer $TOKEN$",
    "Accept": "application/json"
  }
}
```

Home replaces `$TOKEN$` on the server. The script never downloads an OAuth credential, reads an ID/access/refresh token, or refreshes anything itself. Home retains its normal token-substitution/refresh ownership. The collector reads its management password from `MANAGEMENT_PASSWORD` in its environment; it never opens a secret file or inspects container environment variables.

Source checks:

- `internal/managementhttp/routes_v8.go`: maps `/api-call` to `/requests/api-call`, `/auth-files` to `/credentials`, and retains Home's `/models` and `/quota/*` resources.
- `internal/cluster/management/api_call.go`: request fields, server-side token substitution, and `status_code` plus string `body` response envelope.
- `internal/cluster/management/oauth.go`, `authFileEntry`: the list exposes credential ID/index and a display email, but no subscription tier or provider token.
- `internal/quota/codex.go`, `codexPlan`: saved plan naming, including Team, Plus, Pro 20x and Pro 5x. Provider names not mapped there are retained, rather than relabeled.
- `internal/registry/codex_client_models.go` and `codex_client_models_updater.go`: Home validates `slug`/`visibility` in the Codex catalog and refreshes it from `router-for-me/models`, not from the account-specific OpenAI endpoint.
- Home's pinned CPA SDK v8.0.53 `internal/runtime/executor/codex_executor_execute.go` uses the `https://chatgpt.com/backend-api/codex` base. Its request helper supplies a Codex user agent for execution; those execution headers are **not** guessed to be requirements of the model-list request.
- OpenAI's `codex-rs/codex-api/src/endpoint/models.rs` (`ModelsClient::request_url`) constructs a GET to `models` with the `client_version` query. The inspected file blob was `99016987069c9d14b541a6e3a03dafe344f77e36`.

Important distinction: **Home does not implement account-specific OpenAI model discovery.** The exact `0.167.0` URL and minimal headers above come from the lead's successful manual request on November 17, not an invented Home constant. The version is pinned in `check.py`; if OpenAI later changes its client gate or response contract, the check fails visibly rather than silently assuming zero new models.

## Intentional hidden models

Only explicit `visibility: "list"` entries are alert candidates. Entries with `visibility: "hide"` that Home lacks are returned separately as `hidden_missing`, for the local log only. Missing or unknown visibility is a failed check, not permission to hide a model. `supported_in_api` is not used to suppress anything.

This is backed by the actual redacted upstream response: `gpt-reserve` is `hide` even though `supported_in_api` is true. `codex-auto-review` and `gpt-6-chat-thinking` are also `hide`, although Home already carries them. OpenAI's Codex model manager describes picker filtering by visibility and explicitly checks `ModelVisibility::List`. No arbitrary model-name exclusions or blanket old-model filter exist.

All four current saved Codex plans report **promax**. The collector checks all four independently, not just a representative or an assumed Team/Plus/Pro account. Fixtures also exercise separate Team, Plus and Pro 20x accounts to catch future tier-specific offers.

## Delivery and duplicate prevention

Delivery uses `hc send --json`, not a macOS notification that can disappear in Notification Center. `hc --skill` documents safe queuing around unfinished drafts; `hc schema` exposes the accepted send's `turn`. The runner discovers the current terminal address each time and requires exactly one live `claude-code` match:

- Lead: `CLIProxyAPIHome / 1`, with the Home checkout as its launch folder.
- Orchestrator: `Hype | Orchestrator / 1`, with the Hype checkout as its launch folder.

It never types into a shell, never uses `--force`, and refuses missing/ambiguous/wrong-folder matches. Renamed workspaces fail closed: update `TARGETS` in `alert.py` and reinstall deliberately. A successful send proves accepted terminal input, not that a human or agent has read or acted on it.

Alert bodies contain fixed explanatory text plus validated model IDs and tier names only. They contain no account labels, account IDs, emails, passwords, tokens, or provider response bodies. A TEST uses the obvious `TEST ONLY` prefix.

State is private to the MacBook under `~/Library/Application Support/cpa-model-alert/`:

- `state.json`: per-model, per-recipient successful-send times; failure streak and already-notified recipients.
- `last-run.json`: latest success/failure, missing IDs/tiers, non-alerting `hidden_missing`, checked-account count and pending recipients.
- `runner.log`: launchd's small append-only run summaries.
- `run.lock`: prevents overlapping scheduled/manual runs.

The directory is mode 0700; state replacements are atomic mode-0600 files. Restarting or reinstalling preserves state. An accepted alert is not sent to that recipient again until the model remains absent for 24 hours. A confirmed successful check that sees it resolved clears that incident; a later reappearance starts a new incident. A different account or tier offering the same absent model does not create another hourly alert. Failed recipient deliveries retry next run without repeating accepted deliveries to the other recipient.

Three consecutive incomplete/failed collection runs trigger one generic failure alert per recipient for that streak. A successful check rearms it. Partial runs may alert models actually discovered, but cannot clear older missing incidents. Corrupt/unreadable local state is not silently discarded: the runner exits nonzero and records a fixed local-state error in its log; repair it deliberately to preserve deduplication history.

## Install and verify

The lead performs authenticated calls and installation. The helper only runs isolated fixture tests. No production Home restart, deployment or configuration write is part of this installation.

### Ark collector

Copy `scripts/fleet/model-alert/check.py` to:

```text
/mnt/cache/appdata/cpa-model-alert/check.py
```

The operator supplies a private executable wrapper at:

```text
/mnt/cache/appdata/cpa-model-alert/run-check.sh
```

The wrapper exports the existing management authentication in its own approved way, then invokes:

```sh
exec /usr/bin/python3 -I /mnt/cache/appdata/cpa-model-alert/check.py
```

Do not put credentials or a secret-file reader into this repository. From the MacBook, verify the wrapper's sanitized result:

```sh
ssh -o BatchMode=yes -o ConnectTimeout=10 -o IdentitiesOnly=yes -o IdentityAgent=none \
  -i "$HOME/.ssh/cronos_ark" root@100.110.133.6 \
  /mnt/cache/appdata/cpa-model-alert/run-check.sh
```

Expected: exit 0, `ok: true`, four checked accounts, `missing: {}`. A hidden `gpt-reserve` may appear in `hidden_missing`; it must never be sent as a missing-model alert.

### MacBook launchd job

Run from the intended source checkout using an absolute path:

```sh
/opt/homebrew/bin/python3 -I /absolute/checkout/scripts/fleet/model-alert/install.py
```

The installer copies the runner into the stable Application Support directory, preserving state, and enables `com.cpa-home.model-alert` in the current GUI login. Its plist has `RunAtLoad: true` and `StartInterval: 3600`.

```sh
/opt/homebrew/bin/python3 -I "$HOME/Library/Application Support/cpa-model-alert/alert.py" --test-alert
/opt/homebrew/bin/python3 -I "$HOME/Library/Application Support/cpa-model-alert/alert.py"
launchctl print "gui/$(id -u)/com.cpa-home.model-alert"
```

`--test-alert` first collects real data over SSH, then forces `gpt-model-alert-test [test]` through the normal alert-processing and session-delivery path with the TEST prefix. It uses fresh in-memory state and leaves normal dedup history untouched. It must report `check_ok: true` and both `test_delivery` values true. It refuses to send a passing TEST if real collection failed.

The separate ordinary run must show `ok: true`, `missing: {}`, `failures: 0`, and `delivery_pending: []`. Check launchd's 3600-second interval and last exit code; installing a plist is not proof of a successful run.

### Isolated self-test

```sh
python3 -I /absolute/checkout/scripts/fleet/model-alert/selftest.py
```

The assert suite uses fake accounts and an injected HTTP layer, never Home or model inference. It covers tier differences, visibility, malformed/partial responses, the exact api-call body, diff, 24-hour repeats, resolved/reappearing incidents, per-recipient retries, the three-failure streak, atomic/private state, safe terminal resolution, forced TEST isolation and generated launchd settings. `launchctl` is stubbed in this test.

An optional directory argument replays the lead's redacted `credentials.json`, `quota-credentials.json`, `models-available.json` and `upstream-codex-models.json` samples. Those temporary samples are not committed. The measured replay passes with four promax accounts, 100 Home model IDs, no listed missing models, and only `gpt-reserve` in `hidden_missing`.

## Stop or roll back

Disable/unload the MacBook job first:

```sh
launchctl disable "gui/$(id -u)/com.cpa-home.model-alert"
launchctl bootout "gui/$(id -u)/com.cpa-home.model-alert"
```

Keep `state.json` if reinstalling. To remove fully, remove this job's plist and Application Support directory after unloading, then remove only its Ark collector directory and operator-owned wrapper. Home needs no rollback: the alert has never changed its configuration or model catalog.

## Installation acceptance — November 18, 2026

The lead performed the authenticated/live steps, installed the job and returned these results; the helper did not read secret files or run authenticated management calls.

- Live Ark collector: exit 0, `ok: true`, `missing: {}`, `hidden_missing: {"gpt-reserve": ["promax"]}`, four accounts checked, `home_count: 100`.
- Initial unfiltered check exposed `gpt-reserve` as the sole difference. Its measured `visibility: "hide"` led to the explicit visibility rule above; GPT 6.2 Astra and Sol are already present, and no listed model remains missing.
- `install.py`: installed/enabled `com.cpa-home.model-alert`.
- Forced end-to-end TEST: `check_ok: true`, `real_missing: {}`, `test_delivery: {"lead": true, "orchestrator": true}`. Both intended sessions received the clearly labeled TEST line. The test exercised SSH collection and the ordinary alert-processing/delivery path, without writing normal dedup history.
- `launchctl print`: `run interval = 3600 seconds`, `runs = 1`, `last exit code = 0`. This confirms the initial scheduled execution, not a future 24-hour wall-clock observation.
- Isolated Ark assertions: `PASS: redacted real fixture replay: 4 promax accounts, 100 Home models, no listed missing; hidden_missing=gpt-reserve` and the full synthetic suite passed. The first pre-fix test run failed only on the expected fake six-character label; correcting that test expectation produced a passing run. Visibility handling and stronger TEST/installer checks were then added and passed.

The 24-hour repeat and three-failure behavior are deterministic assertions with explicit timestamps/results; no claim is made that 24 hours elapsed during installation. No Home image/configuration, catalog, account priority, access scope or node service was changed.

## Known limits

- Checks pause while the MacBook sleeps or no GUI login is active. Wake resumes launchd scheduling; no always-on guarantee is claimed.
- Missing/renamed/blocked recipient sessions retain pending delivery until a later run can address them. Accepted terminal input is the delivery evidence; no agent-response receipt is inferred.
- If the SSH/auth wrapper, Python executable, `hc`, or local state breaks, inspect the nonzero last exit and runner log. No delivery mechanism can notify a session that no longer exists; there is no second Inbox/notification system here.
- `runner.log` is small but append-only; it is not a replacement for the current `last-run.json` status.
- Saved plan labels are Home's latest quota observation, not a newly fetched subscription claim. Checking every enabled account still avoids losing a catalog because of an outdated tier grouping.
