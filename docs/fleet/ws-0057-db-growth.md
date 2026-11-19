# ws-0057 — SQLite growth and preservation-first retention proposal

Investigation date: 2026-11-19 UTC. Source: fleet `248ebb9`; traced files match deployed `7091c86`. Analyst model: `gpt-6.1-sol[1m]`. This is an internal evidence report for the lead, not a user-facing Markdown deliverable. No production change, pruning, checkpoint, or compaction was performed.

## Finding

This is mainly retained usage, retained database application logs, billing history, and their many indexes—not request-body files or cluster events. Automatic age retention does not exist for these three database history tables. The apparent one-day +2.5 GB also overlaps today's index deployment and must not be called a sustained daily ingestion rate.

All GB below are decimal; GiB is explicitly labeled. Filesystem sizes at investigation:

- Live `home.db`: **48,946,569,216 bytes**.
- Live WAL: **1,282,350,032 bytes**. WAL contains page versions, possibly repeated; do not add its size to logical history growth. Never delete it to recover space.
- Read-only cold backup `home-cold-ws0054-20261119T164721Z.db`: **46,286,286,848 bytes**.
- Historical October 7 backup: **3,461,537,792 bytes** (FLEET.md:87–90). Cold-copy increase: **42.825 GB** over about 43 days, roughly **0.996 GB/day** averaged across traffic, schema/index additions, and page occupancy. This is not a measured constant daily rate.
- Live minus cold: **2,660,282,368 bytes**. Cold copy predates the two wide dashboard indexes; live schema contains both. FLEET.md:100–105 measured 1.74 GiB for these definitions at 3,050,397 usage rows. Scaling to today's row count is only an estimate; exact live index sizes were deliberately not scanned. The delta cannot be attributed wholly to new requests.

## Cold-copy page measurements

`dbstat` read completed **19:39:51–19:42:54 UTC (183 seconds)**. Source opened `mode=ro&immutable=1`, with `query_only=ON`; immutable is used only for the frozen backup, never live WAL. Temp directories were explicitly `/mnt/cache/appdata/cpa-home-build/tmp`. Complete per-object numerical evidence is in `ws-0057-sizes.tsv`.

| Table family (table + all its indexes) | Bytes | Share of DB |
|---|---:|---:|
| usage | 27,177,877,504 | 58.7% |
| log | 11,907,653,632 | 25.7% |
| billing_charge | 7,165,677,568 | 15.5% |
| kv_store | 29,437,952 | 0.064% |
| cluster_events | 2,465,792 | 0.0053% |
| All other families | under 4 MB combined | under 0.01% |

Top ten objects overall:

| Kind | Name | Allocated bytes |
|---|---|---:|
| table | usage | 16,767,619,072 |
| table | billing_charge | 5,108,830,208 |
| table | log | 3,982,942,208 |
| index | idx_log_client_time | 1,565,655,040 |
| index | idx_log_level_time | 1,320,787,968 |
| index | idx_log_time_order | 1,305,923,584 |
| index | idx_log_home_request | 825,311,232 |
| index | idx_log_created_at | 708,108,288 |
| index | idx_usage_quota_activity | 705,277,952 |
| index | idx_log_timestamp | 662,544,384 |

Top ten **tables** separately: usage 16,767,619,072; billing_charge 5,108,830,208; log 3,982,942,208; kv_store 29,208,576; cluster_events 2,113,536; cpa_node_quiescence 462,848; billing_model_price_import_preview 159,744; auth 151,552; billing_model_price_import_operation 57,344; codex_refresh_journal 36,864 bytes.

Top ten **indexes** separately: idx_log_client_time 1,565,655,040; idx_log_level_time 1,320,787,968; idx_log_time_order 1,305,923,584; idx_log_home_request 825,311,232; idx_log_created_at 708,108,288; idx_usage_quota_activity 705,277,952; idx_log_timestamp 662,544,384; idx_log_request_id 614,780,928; idx_usage_auth_time 611,700,736; idx_usage_root_session_time 550,965,248 bytes.

Page size is 4096; page count 11,300,363; freelist 222 pages (**909,312 bytes**); `auto_vacuum=0` (NONE). Allocated objects have **32,864,988,895 payload bytes** and **12,081,978,781 unused bytes inside pages**. That 12.082 GB is not a freelist and cannot be recovered by `incremental_vacuum`; not all of it is necessarily recoverable by a packed rebuild. B-tree overhead and overflow-page occupancy remain. A VACUUM alone does not implement retention or stop future growth.

## Growth by day: rows measured, physical bytes estimated

Daily numeric scan completed **19:43:05–19:44:33 UTC (88 seconds)**; SQL is `ws-0057-measure.sql`, complete date series is `ws-0057-daily.tsv`. No stored string values were returned. The backup contains **4,081,983 usage rows**, **14,054,637 application-log rows**, **4,054,549 billing rows**, and **19,579 cluster events**, dated September 7/8 through the partial November 19 day. October's approximately 150,000 usage rows have become 4.08 million (~27 times as many). History is accumulating, and traffic rose markedly.

Below are seven complete UTC days, **November 12–18**, excluding the partial backup day. Physical bytes/day are **estimated** as rows/day × that table family's current allocated bytes / total rows. They include all that family's indexes, overhead and unused page space. SQLite does not store a historical per-day page allocation ledger: these are not measured daily file deltas, and schema changes/occupancy skew them. Text lengths are exact selected-column bytes, not the entire record or page footprint.

| Table | Rows/day, Nov 12–18 | Selected text bytes/day | Estimated allocated bytes/day (with indexes) |
|---|---:|---:|---:|
| usage | 114,958 | 315,108,950 (payload + tokens + fail + fail_body) | 765,389,439 |
| log | 252,346 | 32,073,531 (line) | 213,797,558 |
| billing_charge | 114,570 | 127,473,009 (price_snapshot + request_summary + cost_breakdown) | 202,482,385 |
| cluster_events | 393 | not measured | 49,459 |

Combined dominant-family estimate: **1.182 GB/day**, not 2.5 GB/day. On November 18 alone there were **78,753 usage**, **220,822 log**, and **78,613 billing** rows. For comparison, complete September 8–30 days averaged **3,497 usage**, **134,346 log**, and **3,467 billing** rows/day; using the same current footprints gives 23.3 MB + 113.8 MB + 6.1 MB/day. The bulk of new growth is increased retained usage/billing traffic, with a substantial constant application-log stream and many index writes per event.

Total stored selected text: usage payload **9,137,864,224**, tokens **867,238,221**, fail JSON **122,233,169**, failure body **3,543,421** bytes; application-log lines **1,760,965,265** bytes; billing price snapshots **2,279,134,705**, summaries **0**, cost breakdown **364,491,351** bytes. Thus blanking standalone failure bodies alone would save only ~3.5 MB before page effects—not gigabytes. Larger payload cleanup needs a field-by-field compatibility review rather than assuming all payload bytes are request bodies.

The covering-index comparison scaled to **4,081,983** usage rows gives **~2.500 GB** versus the **2.660 GB** live/cold delta, supporting an index-dominated explanation for this day's jump. This is corroboration, not exact attribution; the copy and live measurements are at different times and live index page sizes were not measured.

## Writers, readers, and current limits

- `internal/respserver/push/usage.go:32` accepts usage. `internal/cluster/runtime.go:154` persists it via `internal/cluster/usage.go:344–405`. Each new accepted usage record writes structured columns plus original sanitized JSON (`usage.go:65–92,216–241`); failures can be represented in `payload`, `fail`, and `fail_body`. Billing is created in the same transaction (`usage.go:392`). No age-based usage deletion exists in production source.
- Billing writer: `internal/cluster/billing.go:780` (`createBillingChargeForUsageTx`), with price snapshot, summary, and cost details. Billing reads join usage (`billing.go:549–594`); do not delete billing or usage as if they were disposable logs. Billing and usage are the dashboard's historical data, not a redundant persisted daily rollup.
- `internal/cluster/runtime.go:163` calls `internal/cluster/app_log.go:86–100`. Every incoming application log is inserted into table `log`; its line and ten secondary indexes consume substantial space. The only deletion is `DeleteAppLogs` at `app_log.go:103–114`, exposed by `internal/cluster/management/logs.go:151–171`. It deletes **all** app-log rows in one operation: not selective retention, not a safe replacement for batched expiry.
- Request/response body logs are **filesystem files**, written by `internal/respserver/push/request_log.go:23–65,178`; request-log listing is derived from usage, not a separate raw-body DB table. `CleanupLogDirectory` (`push/log_cleanup.go:18–29`) caps local `.log`/`.log.gz` files only, and runs every minute (`cmd/home/main.go:861–882`, wired at :603). Changing its limit will not shrink `home.db`.
- Cluster events are written by `internal/cluster/repository.go:1368–1420`; watchers consume IDs (`events.go:79–107`). No age pruning was found. Their 2.47 MB does not justify introducing replay/watcher risk now.
- In-flight staging, user-security records, KV expiry and terminal rebalance plans have separate expiry paths; none is a retention policy for the large three tables. Usage queue retention defaults to 60 seconds, max 3600 (`internal/config/config.go:928–932`); it is an in-memory delivery queue limit, not persisted history.

Selected live config was read by exact primary keys only, opening live with `mode=ro` and a 1000 ms busy timeout. Values matched the cold copy: `request-log=false`, `logging-to-file=false`, `usage-statistics-enabled=true`; `logs-max-total-size-mb`, `error-logs-max-files`, and queue-retention keys absent. Defaults are **0 (file-size cleanup disabled)**, **10 error files**, **60-second queue**, respectively (`config.go:860–864,920–932`). This does not disable database application-log ingestion. No usage/log-table retention setting exists to turn on.

## Recommendation (proposal only)

1. **Keep every usage row and every billing row initially.** Preserve all dashboard totals, historical breakdowns, session links, and billing audit data. Do not start by deleting usage or blanking payload JSON: record details and summaries still parse payload (`internal/cluster/usage_observability.go:745–762,2434`), and ingestion duplicate matching also reads payload (`usage.go:429–453`). Stored daily aggregate replacement does not currently exist; adding one is a separate larger change requiring exact accounting and dashboard compatibility proofs.
2. **Add seven-day retention for database application logs**, configurable and initially disabled until approved. Small scope: configuration field/default/validation in `internal/config/config.go`; DB-backed/v8 mapping beside existing observability log options; bounded repository method beside `DeleteAppLogs` in `internal/cluster/app_log.go`; a cancellable maintenance loop beside `cmd/home/main.go:861` using the existing database connection and leadership conventions. Use a fixed captured UTC cutoff on trusted ingestion `created_at`, existing `idx_log_created_at`, deterministic `(created_at,id)` batches, about 500 rows maximum per transaction initially, and a short per-batch deadline. Yield between batches; do not run all backlog deletion in one transaction. Recheck age in the delete predicate. Clock-injected tests: old/new/boundary rows, backdated node timestamps, restart/resume, cancellation, failed transaction, and usage/billing unchanged. Document behavior in Management API English and matching language docs. Observe writer latency, heartbeat membership, WAL/disk growth; stop automatically on unhealthy routing or exhausted space. No automatic VACUUM in the maintenance loop.
3. **Cap raw log files separately**, recommending 512 MiB per Home/node as a starting proposal, retaining `request-log=false`. This existing setting is the least code route for filesystem bodies, but it is not the DB-growth fix. Inspect total numeric file sizes before selecting a different cap; no error files were opened here. Recent file debugging will expire under the cap.
4. After seven-day DB-log retention is proven, consider trimming **old duplicated failure/body fields**, not token/accounting columns. First prove all payload-dependent summaries, billing, exports and dedupe remain correct on an isolated copy. Keep a minimal structured error summary, avoid a second archival copy of secrets/body content, and measure whether the saving warrants code. Do not implement this blindly from the space estimate.
5. Do not drop indexes merely because two have similar names; expression/collation, normalized provider filters and query ordering can differ. Usage index changes require real query-plan/latency comparisons for every affected dashboard filter. The newly deployed covering indexes intentionally cost disk to reduce read time.

## Expected size and what this does not solve

For a fixed seven-day cutoff at **2026-11-12 16:47:21 UTC**, indexed `log.created_at` count on the backup finds **1,773,629 survivors** (12.62% of 14,054,637). Its exact observed ingestion range is September 7 23:23:21 through November 19 16:47:21 UTC. Retention eligibility must use this trusted ingestion time, not CPA-supplied timestamps used for the daily traffic table.

Proportionally scaling current log-table/index allocation suggests **10.405 GB** of pages become unnecessary, with the log family falling from **11.908 to ~1.503 GB**. Holding everything else fixed gives **~35.881 GB cold-copy equivalent** before accounting for today's new indexes (~2.500 GB estimated), newer traffic, and changes from packing pages. A first operational target is therefore **roughly 36–39 GB after log expiry and compaction**, not a return to October's 3.5 GB. It is an estimate, not a measured compacted size; VACUUM may pack the existing free space further. Measure the disposable-copy result before promising a final size.

With usage and billing kept, their estimated combined **~0.968 GB/day** remains. Seven-day log retention bounds the log history, but does **not** solve long-term growth of the 34.344 GB usage+billing families. A longer-term decision must either accept that footprint, move analytical history to a separate store with verified queries, or add exact permanent aggregate history plus a finite raw-row horizon. This report recommends keeping full usage/billing first because deleting them now would destroy existing dashboard history and billing evidence. A retention policy is a data-loss choice requiring explicit approval; no existing config can provide history-preserving rollups automatically.

## Safe disk reclamation plan

Deletion frees database pages for reuse, but will not generally reduce the file. Current `auto_vacuum=NONE` means `incremental_vacuum` is ineffective. Switching to INCREMENTAL itself requires a full rebuild; it is not a cheap escape hatch. Prefer one planned offline `VACUUM` after retention is reviewed and rehearsed, rather than daily compaction.

Before an announced window, rehearse on a **new disposable copy**, never modify either verified retained backup. Compare exact usage/billing counts, accounting sums, dashboard responses, schema/index definitions, and current refresh-journal safety. Record actual prune/rebuild timings, peak extra disk and final size; verify integrity/checksums only on isolated results. Current investigation is read-only, so no VACUUM benchmark is claimed.

Space: reserve **at least two times the current DB size as additional working headroom (~100 GB today)** for a conservative in-place VACUUM budget, plus current WAL growth, backup and sort/rebuild overhead. SQLite's journal/rebuild needs vary; do not budget only the expected final smaller file. Cache volume had **811,954,659,328 bytes available (~812 GB)** at this reading, which is not a reservation. Put **both `SQLITE_TMPDIR` and `TMPDIR`** on the data/build volume, not a small container `/tmp`. Confirm before the window.

Timing: read-only page inventory took 183 s for 46.286 GB (~253 MB/s logical scanned file rate, not a measured raw disk benchmark). Existing ws-0054 evidence reports ~150 s copy, ~190 s checksum and ~750 s FULL integrity. A rebuild has reads, writes, ordering, and many indexes; a **10–30 minute VACUUM-only planning placeholder**, plus pruning and at least ~15–20 minutes verification/copy allowance, is not a measured duration or outage promise. Do not schedule the outage until disposable-copy rehearsal replaces this placeholder. Announce measured total plus recovery margin.

During approved work: independently test direct operator/model recovery; pause policy and take the established four operation locks; stop Home cleanly and fence every possible writer following the corrected ws-0054 runbook; verify current WAL-aware state and rotating-credential journal rules; create a fresh verified backup under the two-version rule; prune/rebuild only while stopped. Never remove a WAL manually, restore an older full DB over new authorizations, or copy a frozen backup back into service as a rollback shortcut. Restart with the unchanged current credentials, validate all nodes, permissions, preserved usage/billing totals, recent log reads, and both request families; restore saved policy exactly and release locks. Keep patch updates off. Set an independent recovery watchdog with a tested readiness check against the reviewed container, not a deleted predecessor.

## Checks and limitations

- Production reads: filesystem stat/free-space; primary-key scalar configuration query; sqlite_master names for two indexes. No live `dbstat`, table count, VACUUM, ANALYZE, checkpoint, integrity scan, request content or secret read.
- Cold-copy reads: page/freelist/auto-vacuum pragmas; sqlite_master object names; dbstat page aggregation; numeric daily counts and byte lengths. Exact SQL for daily scan is `ws-0057-measure.sql`.
- Initial page-analysis attempt failed before SQLite ran because host lacks the external `time` executable; rerun used explicit UTC start/end timestamps and completed with exit 0. Daily scan completed exit 0, as did the indexed seven-day count. No build/test needed for documentation-only changes; Go code unchanged.
- Local Python assertion check: **PASS: 272 page objects; totals reconciled; four daily series; numeric-only evidence; report key claims present.** This verifies row totals, size total, date-shaped labels and numeric-only fields, plus the SQL's four aggregate queries. Whitespace check is `git diff --cached --check` after staging named evidence/report files.

Reproduce the page inventory only on a frozen read-only copy:

```sql
PRAGMA query_only=ON;
SELECT s.type,d.name,s.tbl_name,SUM(d.pgsize),SUM(d.payload),SUM(d.unused),COUNT(*)
FROM dbstat d JOIN sqlite_master s ON s.name=d.name
GROUP BY d.name ORDER BY SUM(d.pgsize) DESC;
```

Use host `sqlite3 'file:<cold-copy-path>?mode=ro&immutable=1'` with both temp-directory environment variables set to the build data volume. For daily measurements, pipe `ws-0057-measure.sql` to that same command. The only additional retention count was `SELECT COUNT(*) FROM log WHERE created_at >= '2026-11-12 16:47:21';`; created-at bounds used its existing index. Do not substitute the live path for either heavy query.
- No old October database remains available under the two-backup policy; historical per-table growth cannot be reconstructed exactly. No live per-index measurement, production compaction, actual VACUUM timing, filesystem body-volume scan, or provider request test was performed.
