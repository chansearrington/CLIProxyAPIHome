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

Offline worker tests: `python3 -B test_worker.py`. Home's Go state-machine and security tests run
on the Ark with the rest of the Home suite.
