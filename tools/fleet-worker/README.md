# CPA Home node-local fleet executor

Home owns the maintenance policy, timing, verified releases, canary/serial rollout, rollback and
results. This package executes only the current node's typed tasks using its existing verified
mTLS certificate. It stores no Management secret and requires no SSH coordinator on Home or a laptop.

On a node, from this directory:

```sh
python3 -B install.py --host home.example --agent moxy
```

The agent name selects the node-local agent-os maintenance lock (`moxy`, `hyper`, `lara`, `chip`).
For a laptop CPA node without OpenClaw, omit `--agent`. The first adapter is darwin/arm64.
Certificate files remain owned by CPA in `~/.cli-proxy-api/`; they are never copied to Home or
into this package. The server's CA and hostname are verified for every connection.

The installer adds `ai.openclaw.cpa-home-fleet-worker`, polling every 60 seconds. Polling does not
choose policy or an update target: Home sends at most one typed task with an exact lease token.
The local run lock, maintenance lock, retained receipts and server lease prevent duplicate work.
A certificate can read/report only its own registered node tasks. Model sync preserves operator
model choices and unrelated configuration. Binary changes reuse the existing verified restart-
in-place/rollback script; no service is stopped or unloaded by a node upgrade.

Catalog tasks update OpenClaw and Claude Code independently. A CPA-wired Claude Code
installation is a catalog target even without OpenClaw, so laptop menus participate in
Home's normal schedule. Claude Code receives the latest native Fable, Opus, Sonnet and
Haiku models from its own key-scoped CPA listing. Encoded provider aliases are excluded.
The generated entries appear before existing custom picker options; defaults and sessions
are never changed. Older built-in choices remain controlled by Claude Code itself.

Only rows that still exactly match the worker's ownership record are replaced on a future
model release. Edited/generated rows become user-owned. Ownership and two retained private
configuration backups live in the existing worker state directory. Writes detect concurrent
edits; an ownership-publication failure rolls back only the affected picker field.

Settings, bundle hashes and private receipts live under
`~/.openclaw/services/cpa-home-fleet-worker/` and `~/.openclaw/state/cpa-home-fleet-worker/`.
The worker prints status codes, operation and job ID, never lease tokens or credentials. Home's
Management API `GET /v8/management/fleet` shows the fleet-wide policy and history.

To pause this executor only:

```sh
launchctl bootout gui/$(id -u)/ai.openclaw.cpa-home-fleet-worker
```

Pause Home policy before manual fleet maintenance. If Home reports an active task, let its owner
finish/reconcile before changing ownership. Keep receipts and binary/config backups; do not reset
whole configurations or restore old OAuth state. The older MacBook fleet coordinator is retired.

After an enabled Home plan is received, normal polling prunes only top-level release archives
and completed task receipts. It keeps the two most recently downloaded archives (file modification
time), plus installed, manifest rollback, Home-accepted and current task before/target versions.
Rollback still uses the verified service-root `.bak` binary; those backups are never pruned here.
Success, failure and blocked receipts are retained for seven days for diagnosis and report retries.
Home's current task is always kept, even after its lease expires, so retrying a report cannot
repeat execution. Running, unknown or unreadable receipts stay. Cleanup is skipped while Home
is unavailable or disabled, because a disabled plan does not disclose its active task.

Cleanup has a one-second cooperative work budget and removes at most 100 files per poll; backlog
is drained across polls. An incomplete directory scan deletes nothing, rather than guessing the
newest archives. File operations can exceed that budget on a stalled filesystem. Only regular
files with exact version-archive or UUID-receipt names qualify; no directories or symlinks are
followed. Settings, locks, acceptance/latest records, staged binaries, catalog backups and old
workstream folders are untouched. Cleanup failures log fixed codes and do not prevent tasks.

Offline worker tests, including both naming conventions, from this directory:
`python3 -I -m unittest discover -p '*test*.py'` and
`/usr/bin/python3 -I -m unittest discover -p '*test*.py'`.
Home's Go state-machine and security tests run on the Ark with the rest of the Home suite.
