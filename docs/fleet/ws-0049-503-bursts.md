# ws-0049 — MacBook fast 503 bursts, November 13

Research only. No code, configuration, database, container, or fleet policy changed. No requests to a model were made. This file is the agent working report for the workstream lead, not the user-facing deliverable.

Model: `gpt-6.1-sol[1m]`.

## Summary

**The immediate cause is confirmed:** the MacBook node lost its Home subscription heartbeat, marked its Home connection unhealthy, and its global HTTP gate immediately rejected new requests with empty HTTP 503 responses. That gate also covers model-list and hello requests. This is not evidence of a model provider rejecting hundreds of requests.

**A concrete Home-side bottleneck is identified:** the dashboard capabilities endpoint calls `UsageTokenBreakdownV2Ready`, which checks the entire usage history through the scheduler's single SQLite writer connection. Its unordered query can ignore the existing small partial index and scan the full accounting-version index instead. Home logged that exact query taking **3.356 seconds during B1**, longer than the verified **3-second** heartbeat deadline, and **2.378 seconds during B2**, with concurrent operations queuing behind it. B1 has **five** Home subscription-liveness failures within half a second. That is strong evidence against a MacBook-only network problem.

B3 and B4 have the same node failure mechanism and same-second Home liveness failures, amid broad database latency and expensive dashboard queries. However, the unordered readiness scan was **not** logged in those two windows. Shared Home resource/database pressure is the leading explanation there, not a proven single offending statement. Heavy dashboard COUNT queries use the separate read pool, so it would be wrong to claim those COUNTs directly monopolized the writer connection.

Confidence: **high** for the subscription gate and Home-side involvement; **high/strong evidence** for the capabilities scan contributing to B1/B2; **moderate** for the exact initiating cause of B3/B4.

## Timestamp correction

The handoff's UTC burst times are **one hour early**. The raw node timestamps are 10:14, 10:26, 10:50 and 11:34, not 09:14, 09:26, 09:50 and 10:34. Both raw node stdout and forwarded node logs use UTC−6 for these events.

This was checked against identical request IDs in raw stdout and Docker's UTC-prefixed forwarded lines, rather than assuming a timezone:

- Request `f54e1194`: node 09:13:43; Docker UTC 15:13:43.0325Z.
- Request `7676adfe`: node 09:50:16; Docker UTC 15:50:16.5031Z.
- Request `6d357575`: node 10:34:23; Docker UTC 16:34:23.5037Z.

The initial examination of the handoff's 15:xx windows found unrelated slow queries. Those apparent correlations must not be used as burst evidence. The table below uses the corrected UTC windows.

## Evidence table

All events are November 13, 2026. Counts below were recomputed from the local node stdout log, not copied from the handoff.

| Burst | Raw node clock and corrected UTC | Node evidence | Home evidence, Docker UTC |
| --- | --- | --- | --- |
| B1 | 10:14:17–19 CT = **16:14:17–19Z** | **49 HTTP 503**: 47 POST messages, 1 GET models, 1 HEAD hello. At 10:14:17: subscription timeout, failed in-flight snapshot, replay-cache EOF/closed connections. Reconnect discovery EOF at :18; retry backoffs 0.686 s and 1.606 s. | **Five** `failed to refresh subscription liveness: context canceled` warnings at **16:14:17.004, .252, .356, .425 and .450Z**. Unordered pending-accounting query ends **16:14:18.1308Z**, duration **3356.229 ms**, implying start around **16:14:14.7746Z**. Other writer-path lookups take 1–2.1 s and complete around the failures; KV cleanup times out at 16:14:17.6909Z. |
| B2 | 10:26:27–38 CT = **16:26:27–38Z** | **115 HTTP 503**: 111 POST messages, 4 GET models; **35 in the first second**. Subscription closed, in-flight publish failed, replay-cache connections closed at :27. Discovery EOFs at :28, :29 and :32; retry backoffs grow to **6.839 s**. Separate later 503 at 10:33:30 CT = **16:33:30Z**. | Liveness failure **16:26:27.1552Z**. Same unordered accounting readiness query ends **16:26:27.2019Z**, duration **2378.230 ms**, implying start about **16:26:24.8236Z**. API-key/access, KV, user and log queries pile up with ~1.9–2.45 s durations. Discovery certificate/cluster queries take ~0.76–1.64 s. |
| B3 | 10:50:01–05 CT = **16:50:01–05Z** | **90 HTTP 503** POST messages, plus **1 HTTP 502** GET models. In-flight publish failure at :01; subscription closed at :02; discovery EOF at :03; backoffs 0.794 s and 1.655 s. | Liveness failure **16:50:01.9608Z**. KV lookup takes **2007.096 ms**, completing 16:50:01.9630Z. Widespread lookup/log latency. A dashboard usage COUNT times out after **23093.745 ms** at **16:50:00.3782Z**; another takes **30546.180 ms**, ending 16:50:26.9751Z. These COUNTs are read-pool pressure, not proof of a writer lock. No slow unordered readiness probe found in this window. |
| B4 | 11:34:18–21 CT = **17:34:18–21Z** | **79 HTTP 503** POST messages, plus **1 HTTP 502** GET models. In-flight publish, replay-cache closed connection/EOF and subscription timeout at :18; discovery EOF at :19; backoffs 0.982 s and 1.514 s. | Liveness failure **17:34:18.6139Z**. Broad writer-path latency before the event. Dashboard COUNT times out after **38691.713 ms** at **17:34:16.3777Z**; another times out after **28948.711 ms** at 17:33:52.4727Z. No slow unordered readiness probe found in this window. |

The bursts are dominated by very short responses, but **not every response is 0–8 ms**. Some requests already in progress end with 503 after seconds; error-log cleanup also adds local time. The local log records a 6.072-second 503 in B4. Those slower failures should not be conflated with new requests rejected instantly by the heartbeat gate.

Other verified facts:

- Local startup banner: CPA **8.0.69**, commit `f7ae8fee`, built 2026-11-12T18:27:10Z, startup logged **2026-11-12 23:34:51 CT**, i.e. **November 13 05:34:51Z**. The preceding version was 8.0.67. This confirms the new version running; it does not independently prove which manager initiated the upgrade.
- The bursts occurred many hours after startup, not during the upgrade itself. The upgrade's responsibility is not established.
- The earlier raw CT 03:00–07:30 interval contains exactly **129 HTTP 500 POST messages** and **1589 lines containing both credential and profile**. These are a separate symptom class; their root cause was not established in this slice.
- Home inspection at research time: started 2026-11-10T22:58:42.941Z, Docker restart count **0**. One current stats sample was CPU **4.74%**, memory **170.8 MiB**, cumulative block reads **2.46 GB**. These are not historical measurements for the bursts.
- Home's logs directory exists; no top-level `*log` files were returned by the bounded listing. Docker logs provided the usable Home evidence.

## Cause and code path

### Confirmed immediate failure mechanism

CPA source was read directly from upstream tag **v8.0.69**, matching the local banner; no local CPA checkout was found in the bounded GitHub-directory search.

1. `internal/api/server.go:243–245` installs the Home-heartbeat middleware globally before routes.
2. `internal/api/server_middleware.go:336–357` checks `home.Current().HeartbeatOK()`. If missing or false, it calls `AbortWithStatus(503)` before the route handler. It exempts management/plugin-resource routes, not messages, models or hello.
3. `internal/home/client.go:2045–2057` fences dispatch and clears `heartbeatOK` when subscription reception fails.
4. `internal/home/client.go:2069–2077` renews liveness only on a persisted heartbeat PONG. Ordinary config/cluster traffic does not renew it.
5. `internal/home/client.go:2163–2175` enforces an absolute heartbeat deadline. Reconnection failures then add the visible retry backoffs in `sdk/cliproxy/service_home.go:623–629`.

This explains empty, fast, route-wide 503s with no provider execution. Related 502s and already-running provider failures can occur beside the outage without being the cause of the gate.

### Why Home database work can cause it

Home `internal/respserver/server.go:196–210` refreshes persisted subscription liveness **before sending the heartbeat PONG**. `internal/cluster/quiescence.go:373–408` does that refresh in a transaction on the ordinary repository database, including membership verification and update. SQLite has one connection on this path (`internal/cluster/db.go:110`). A long read occupying that connection can prevent heartbeats even without a SQLite `database is locked` error.

A read-only query of the live `lifecycle_config` extracted only revision and the heartbeat timeout field: revision **1**, timeout **3000000000 ns = 3 seconds**.

The capabilities handler is a missed isolation path:

- `internal/cluster/management/usage_observability.go:27–32`: `GetCapabilities` invokes `UsageTokenBreakdownV2Ready`.
- `internal/cluster/token_accounting_backfill.go:224–234`: it uses `r.database()`, then `SELECT id ... WHERE token_accounting_version <> 2 LIMIT 1`, **without an ORDER BY**.
- The similar maintenance helper at lines 166–173 **already uses `Order("id ASC")`**. The observed unordered SQL is therefore not that minute-maintenance helper.
- Normal usage-record queries use `observabilityDatabase()` (`usage_observability.go:655–667`). The separate SQLite read pool is explicitly four connections (`observability_db.go:15–18, 70–74`). The capabilities probe bypasses it.

### Read-only live query-plan evidence

Only index definitions and query plans were read; neither full usage query was manually executed.

Both indexes exist:

```sql
CREATE INDEX idx_usage_token_accounting_version
  ON usage(token_accounting_version);
CREATE INDEX idx_usage_pending_token_accounting_v2
  ON usage(id) WHERE token_accounting_version <> 2;
```

Real output from the Ark's `sqlite3 -readonly`:

```text
EXPLAIN QUERY PLAN SELECT id FROM usage WHERE token_accounting_version <> 2 LIMIT 1;
QUERY PLAN
`--SCAN usage USING COVERING INDEX idx_usage_token_accounting_version

EXPLAIN QUERY PLAN SELECT id FROM usage WHERE token_accounting_version <> 2 ORDER BY id ASC LIMIT 1;
QUERY PLAN
`--SCAN usage USING INDEX idx_usage_pending_token_accounting_v2
```

The preserved partial index is **not missing**. The unordered query fails to select it in this check. Caveat: the Ark's SQLite CLI is not Home's embedded Go SQLite connection, so its plan is supporting evidence, not a captured embedded-engine execution plan. The production SQL timings independently establish the costly unordered query.

### Ranked initiating causes

1. **Home SQLite writer starvation, with a demonstrated bad capabilities probe (B1/B2): strongest.** Query duration, queued writer work and same-second Home subscription failures align. B1's five failures argue for shared Home impact rather than one laptop losing Wi-Fi.
2. **Shared Home database/CPU/I/O pressure under expensive dashboard reads (especially B3/B4): plausible and best-supported remaining explanation.** Counts take 23–39 seconds while small writer operations also slow down. The read pool prevents direct connection queueing from those COUNTs, but cannot isolate shared CPU, storage or database-engine resources. Historical resource/lock traces were not available to distinguish these mechanisms.
3. **MacBook-specific network stall: less supported overall, not eliminated for every burst.** TCP timeouts alone do not distinguish network failure from an unresponsive Home. B1 has five server-side subscription failures; B2–B4 each expose only one in the checked window. Their Home warnings do not identify the peer, so I cannot map every failure to a named node.
4. **Provider outage, credential-profile warnings, or error-log cleanup as the primary cause: poorly supported for these bursts.** They do not explain a Home-dependent global gate blocking non-model routes. Cleanup warnings are mostly attempts to remove files already gone and can be fallout from concurrent error generation; no per-request error file was opened.

Home forwarded successful traffic from all five node addresses in each bounded burst neighborhood. This does **not** prove all five stayed subscribed throughout: failed nodes cannot reliably forward their own failure logs, and some successful requests were already underway. Five Home failures in B1 are proven; all-node simultaneous outages in every burst are not.

## Proposed fix — do not implement in this research slice

1. **Fix the existing capabilities probe rather than changing CPA's safety gate.** Add the same `Order("id ASC")` already used by the maintenance helper, and use the existing observability read database for this read-only capability check. Both changes are small and directly address a demonstrated writer-blocking query.
2. Add focused regression checks for the actual generated capability query with both production index names present, and for capabilities reads not occupying the scheduler's writer. Existing `fleet_pending_index_test.go:33` validates an ordered query, not the unordered capabilities query. Preserve detection of late legacy rows; do not cache `ready=true` permanently without an invalidation rule.
3. Treat the whole-day usage COUNT/aggregate optimization as a related but separate task. Verify B3/B4 under representative dashboard load while recording writer queue latency, heartbeat commit-to-PONG time, CPU and storage delay. Fixing the demonstrated probe alone must not be advertised as proving every burst fixed.
4. Keep the 3-second lifecycle fencing behavior unless the full concurrency/reclamation contract is deliberately redesigned. Raising a timeout or allowing requests through on stale Home state hides pressure and can affect credential/concurrency safety.
5. After a reviewed fix, require no recurrence in capabilities-plus-dashboard load with active subscriptions, not merely a successful build or one model request. Follow normal recovery and rollout safeguards for any deployment.

No matching later released CPA fix was found: upstream's latest release was still **v8.0.69** at the check. Searches for Home/503 and heartbeat/subscription issues/PRs found older cancellation issues and test-only heartbeat retry/failover PRs (#7063, #7172, #7219), not an established fix for this Home capabilities scan. Home issue #81 tracks slow usage overview work; it is relevant context, not proof of this specific cause.

## What was not checked

- No per-request error logs, session text, `.env` files, secrets, credentials, account emails, private keys or certificate contents were read for this investigation.
- No API writes, model requests, restarts, policy changes, fleet locks, database tuning, integrity scans, data backfills, ANALYZE, or index changes were performed.
- No historical CPU/GC/disk/SQLite mutex profile, packet capture or per-connection wait trace was available. Exact B3/B4 initiating statements remain unproven.
- No local logs from the four other machines were collected, and no authenticated `/nodes` call was made. Home subscription warnings omit peer identity.
- No reproduction, Go build or test was run: this was a read-only incident investigation, not an implementation. The query-plan checks succeeded; they are not a performance benchmark.
- The earlier 129 HTTP 500s and credential-profile failures were counted, not diagnosed. The patch upgrade's scheduler provenance and whether 8.0.67 behaved differently under identical load were not verified.

Task state: investigation complete; proposed remediation and full-load acceptance remain for the workstream lead. Report-only output; no commit or push.
