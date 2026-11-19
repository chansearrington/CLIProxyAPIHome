-- Read-only cold backup only. Output is dates, counts and byte lengths, never stored text.
PRAGMA query_only=ON;
PRAGMA temp_store=FILE;
SELECT 'usage_daily',date(timestamp),count(*),sum(length(CAST(payload AS BLOB))),sum(length(CAST(tokens AS BLOB))),sum(length(CAST(fail AS BLOB))),sum(length(CAST(fail_body AS BLOB))) FROM usage NOT INDEXED GROUP BY date(timestamp) ORDER BY date(timestamp);
SELECT 'log_daily',date(timestamp),count(*),sum(length(CAST(line AS BLOB))) FROM log NOT INDEXED GROUP BY date(timestamp) ORDER BY date(timestamp);
SELECT 'billing_daily',date(created_at),count(*),sum(length(CAST(price_snapshot AS BLOB))),sum(length(CAST(request_summary AS BLOB))),sum(length(CAST(cost_breakdown AS BLOB))) FROM billing_charge NOT INDEXED GROUP BY date(created_at) ORDER BY date(created_at);
SELECT 'events_daily',date(created_at),count(*) FROM cluster_events NOT INDEXED GROUP BY date(created_at) ORDER BY date(created_at);
