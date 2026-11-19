# Working plan

Goal: explain SQLite growth with cold-copy measurements and recommend a non-destructive next step. No live writes, cleanup, checkpoint, compaction, deployment, request-body reads, or secret reads.

Scope: trace usage/request-log/event writers, cleanup scheduling, database-backed configuration, and relevant indexes. Measure table/index pages and date-grouped numeric counts/lengths only on the read-only cold backup. Any production read is limited to filesystem sizes and cheap selected configuration/indexed metadata.

Acceptance: top ten table/index page sizes; daily row/byte estimates with assumptions; retention implementation and selected runtime values; preservation-first retention/compaction proposal with space/time limits; reproducible commands/results; committed and pushed report on ws-0057/db-growth, no PR.

Sequence: (1) source trace and safe tool discovery; (2) cold-copy measurements; (3) reconcile growth including new indexes and WAL; (4) document uncertainties and plan; (5) check documentation diff, commit, push, report to lead. Source tracing and remote read discovery are independent; measurement depends on tool discovery. No helpers are launched.
