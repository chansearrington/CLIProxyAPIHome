# Claude Code 503 after a MacBook reboot

Status: service recovered naturally; preventive transport fix passed tests and private network rehearsal; review pending.

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

The change applies Linux `TCP_USER_TIMEOUT` to accepted subscription sockets, using
the existing CPA heartbeat timeout (3 seconds on this fleet). It reaches the underlying
TCP socket through both protocol-mux buffer layers and TLS. It only bounds
unacknowledged control-connection data; it adds no inference timeout or payload change.
Non-Linux Home builds retain their current transport behavior. Tests and a private
network-blackhole rehearsal must pass before a production image change.

The exact kernel retransmission state of the already-ended incident was not captured.
The socket lifetime, duplicate-membership rejection, reboot timing, and recovery are
directly observed; the buffered unacknowledged-data mechanism is supported by source
and will be checked independently in the rehearsal.

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
pilot/system probe key, not Hyper's personal key and not an unidentified interactive user.
