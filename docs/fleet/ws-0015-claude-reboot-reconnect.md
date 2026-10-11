# Claude Code 503 after a MacBook reboot

Status: reviewed fix deployed and verified; stability checks passed and all fleet locks released, 2026-10-11 05:53 UTC.

## Verified incident

- macOS records shutdown at 00:18 CDT and reboot at 00:19 CDT, October 11.
- At 05:19:00 UTC the old CPA subscription reported a read timeout, followed by
  `no route to host`. Its supervisor stopped deliberately at 05:19:07 UTC.
- New CPA processes initially failed Home discovery while the network was unavailable.
  The surviving process started at 05:20:56 UTC, running the same CPA 8.0.20 binary.
- From 05:21:02 through 05:30:02 UTC, its subscription attempts failed with
  `CPA certificate is already owned by an active membership`. Actual Claude
  `/v1/messages?beta=true` requests returned 503 during that interval.
- Home retained the old subscription socket to the MacBook's port 59098. Other old
  connections reported broken pipes at 05:21:23–28; the subscription finally reported
  a closed connection at 05:30:02. The node's readiness recovered at 05:30:05.
- Home remained on image `sha256:19143382b131c59add5e65ba2e83831a66c4bc127b0262b88a21c6024c833874`,
  started October 10 at 03:22:35 UTC, with zero restarts.
- At 05:32:36–40 UTC, real streaming Opus and Sonnet requests returned 200, `ok`,
  and `message_stop`. A fresh native Claude Code Opus session returned
  `CPA_CLAUDE_OK`, exit 0, at 05:34 UTC. Home usage rows 225907, 225908, and 225937
  record successful routing through Lara's shared Claude subscription for client key 6.
  All five memberships and plugin reports were healthy.

## Cause and proposed prevention

Home refreshes membership liveness before sending each one-second subscription pong
(`internal/respserver/server.go`). Successful socket writes only establish that data
entered the local send buffer; they do not establish that the peer acknowledged it.
The deployed subscription transport has no acknowledgement timeout. After the MacBook
network disappeared during shutdown, the old socket could keep its membership alive
until the operating system eventually failed the connection. A new process cannot
take over a previous process's lifetime safely, so Home correctly rejected the new
subscription while the old lifetime remained active.

CPA 8.0.20's `internal/api/server_middleware.go:40–59` supplies the exact visible
failure: when `home.Current().HeartbeatOK()` is false, its Home heartbeat middleware
calls `AbortWithStatus(503)` without a response body. This rejects the request before
credential dispatch or provider inference, explaining Claude Code's “no body” message.

The change applies Linux `TCP_USER_TIMEOUT` to accepted subscription sockets, using
the existing CPA heartbeat timeout (3 seconds on this fleet). It reaches the underlying
TCP socket through both protocol-mux buffer layers and TLS. It only bounds
unacknowledged control-connection data; it adds no inference timeout or payload change.
Non-Linux Home builds retain their current transport behavior. Tests and a private
network-blackhole rehearsal must pass before a production image change.

The exact kernel retransmission state of the already-ended incident was not captured.
The socket lifetime, duplicate-membership rejection, reboot timing, and recovery are
directly observed; the buffered unacknowledged-data mechanism is supported by source
and was checked independently in the rehearsal below.

## Validation before deployment

- Ark Linux tests inspect `TCP_USER_TIMEOUT` on a real socket wrapped in the exact
  buffer/TLS/buffer chain used by Home. It is 3000 ms; a closed transport returns an
  error, while non-TCP test connections remain supported.
- An isolated Docker network namespace reproduced a reboot-like blackhole by dropping
  server TCP packets after establishing the connection. With the original kernel
  defaults, heartbeat writes continued and the socket remained open for the entire
  10-second observation. With the timeout, the kernel closed it with ETIMEDOUT in
  **3.213 seconds**. No production namespace or firewall was changed.
- Focused RESP/protocol-mux tests, the full Home Go suite, and the Home compile passed
  on the Ark in `golang:1.26-bookworm`. No Go compiler was installed on the MacBook.

## Unassigned client key

The unowned Home client key (ID 1) matches Ark's original `.canary-client-key` and Hyper's
`~/.openclaw/services/cpa-home-node/enrollment.json` field `canary_client_key`.
The running `hyper-shadow.mjs` passes it to `monitorReadiness`, which polls the local
node's authenticated model catalog. It does not match the readiness keys on the
MacBook, Moxy, Lara, or Chip. Hyper's actual user inference key is a separate key,
ID 2, assigned to Hyper. The readiness key must remain enabled unless its enrollment
reference is replaced coherently.

The latest six inference records for this key are maintenance proofs on October 8–10,
using the MacBook node, including the all-key deployment check documented in ws-0014.
There were no matches in the most recent 15,000 inference records at this inspection.
Catalog readiness polls do not create inference usage records. This is the original
pilot/system probe key. Hyper's personal inference key is separate; the verified recent
uses of the system key are readiness and maintenance checks.

## Reviewed deployment

PR [#14](https://github.com/chansearrington/CLIProxyAPIHome/pull/14) received a clean
local review and synchronous independent validation at
`008cf2cc9b8e34116fb6e4292eed0d452762b34c`: zero MUST, SHOULD, or NICE findings.
It merged as `16e6971a7d32fc03e044714c85110fd9eed919b6`. After fetching origin and
confirming the branch was current, the full Go suite passed again on the merged tree.
All 64 retained panel assets matched the running server before the image build.

All four canonical fleet locks covered the detached backup, image build, cutover,
and immediate verification, with renewal every 45 seconds. The driver stayed in
the foreground throughout. Each lock was released and independently read as UNLOCKED.

- Backup: `/mnt/user/appdata/cpa-home/data/backups/home-pre-subscription-timeout-20261011T054526Z.db`.
- Size: 4,408,655,872 bytes; full integrity `ok`; mode 0600.
- SHA-256: `a1ab8d7261c9750fb41b7346917d8c2f97ee3d847dcb94ba04ddf3bd9a6576b5`.
- Compose backup: `/mnt/user/appdata/cpa-home/backups/compose-pre-subscription-timeout-20261011T054526Z.yml`, mode 0600.
- New tag: `cpa-home:1.1.0-claude-fleet-16e6971`.
- Immutable image: `sha256:a0622434ac0107c827f756a16a4bc0824c69673ba13e0057075d5a1589d20fc0`.
- Cutover/start: **2026-10-11 05:49:04 UTC**, zero container restarts.
- Image-only rollback: `sha256:19143382b131c59add5e65ba2e83831a66c4bc127b0262b88a21c6024c833874`.
  Restore the saved compose image reference and recreate only service `home`.
  Do not restore a stale database over newer rotating OAuth credentials.

After cutover, keys 1–6 each received HTTP 200, `ok`, and `message_stop` from native
Sonnet streaming requests. Ledger rows 225978–225983 confirm Lara attribution,
consistent with the configured priorities and the availability of the higher accounts.
Codex Responses completed with HTTP 200 (225984); Copilot completed for Chanse's key
(225985). The other five keys preserved the existing Copilot denial:
HTTP 503 with a nonempty `auth_not_found` JSON body, verified against the old image
before deployment. This expected scope denial differs from the empty heartbeat-gate
503 involved in the incident.

Native Claude requests on Moxy, Chip, Hyper, and Lara all completed HTTP 200 at
05:49:52 UTC, ledger rows 225986–225989. They used the respective enrollment readiness
keys. A fresh MacBook Claude Code Opus session returned `CPA_RECONNECT_OK`, exit 0,
at 05:49:55 UTC, recorded through the MacBook node in row 225990. All five nodes
were healthy with plugin report `reported_ok`; their new subscription times were
05:49:10 UTC. Key ownership/scopes, bindings, enabled flags, and Claude priorities
matched the pre-deploy snapshot exactly. CodexBar's scheduled four-account token sync
remained successful at 05:49:49 UTC.

Monitoring from 05:50:34 to 05:53:07 UTC completed **50/50** authenticated catalog
checks with zero failures and maximum latency 158.4 ms. All five subscription
lifetimes stayed at 05:49:10 UTC across multiple maintenance cycles, with healthy
membership and `reported_ok` plugin state. Home logged zero subscription transport,
liveness, duplicate-membership, or fencing warnings after cutover, and remained on
the new image with zero restarts. The scheduled CodexBar sync succeeded again at
05:51:50 UTC. The offline HTML update guide now explains the reconnect improvement.
