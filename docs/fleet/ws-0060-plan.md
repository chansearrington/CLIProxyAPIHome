# ws-0060 working plan

Goal: build, never deploy, authenticated durable upgrade notice receiver and fix permanent rejection handling.

Scope: internal/fleet/schedule.go and regression tests; tools/fleet-announce stdlib receiver, Mac relay, tests; deployment instructions. Existing payload and policy stay unchanged; an environment-supplied Bearer header adds authentication without a new secret-bearing policy field. Initial URL-query idea was rejected after checking the API contract, which explicitly forbids URL credentials. Relay reuses installed ws-0056 send helper and same Orchestrator terminal resolution, with Ark outbox retained through Mac sleep.

Acceptance: 2xx only after fsync; authenticated and validated bounded notices; durable dedupe and pending relay across restart; no raw request logging; synthetic Python tests; Ark-only gofmt/vet/focused tests/full internal/fleet and compile; permanent critical 4xx audited/discarded without rollout result mutation; ordinary 4xx touches only its own window. Commit/push PR against fleet; no merge or deployment.

Sequence: inspect sender and delivery path; fix sender/tests; implement receiver/relay/tests; remote verification; self-review; commit/push/PR. Single builder, no subagents. Risk: hc acceptance is not proof of reading; crash between send and acknowledgement can duplicate delivery. Explicitly document both.
