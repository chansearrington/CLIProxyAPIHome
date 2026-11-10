# MacBook Tailscale receive-loop failure and Claude quota errors

Status: connectivity restored in live tests; recurring IPv6 receive-loop warning remains unresolved. Hotmail and Hyper Rally are healthy; other Claude errors reflect provider limits. Remote-access app clarification is pending.

The user reported Tailscale trouble and Claude errors, then clarified the network problem is
on the controlling MacBook Pro, not Lara. All checks and VPN recovery below target the laptop;
no mini network settings, Home configuration, account priorities or permissions were changed.

## MacBook network evidence and recovery

Tailscale 1.104.4 on macOS 27.2 reported `ReceiveIPv4 is not running` while its control plane
still showed online. Rebind/restun did not clear the warning. A connection cycle restored
reachability, but `ReceiveIPv6` then failed. Restarting the existing macOS VPN service with
`scutil --nc stop/start` replaced the network-extension process and initially cleared health.
A subsequent full app-and-extension restart also initially cleared health, but the IPv6
warning recurred after approximately two minutes. Root-cause evidence in the macOS unified log:

```text
Receive func ReceiveIPv6 exiting with error: *net.OpError,
read udp6 [::]:41641: recvfrom: no route to host
```

This matches the receive-loop failure class reported in the still-open upstream issue:
https://github.com/tailscale/tailscale/issues/20616
The installed version matched GitHub's latest stable release at investigation time. No
unreleased build, automatic recovery daemon, OS IPv6 disablement or corporate extension change
was installed as a speculative fix. Do not claim permanent recovery merely from `Running`.

The first CLI reconnect included `--timeout`, which caused `up` to demand the full saved set of
nondefault preferences and reject the invocation. No-flags `tailscale up` preserves those
preferences and succeeded on the later reconnect. All final preference fields compare exactly
with the private original snapshot; login identity and tailnet IP are retained.

Live TCP checks from the MacBook succeeded to all four minis' SSH ports, Lara's Screen Sharing
port and Ark's Home listener. Workbench's current local logs show `Connected! Announcing ourselves`
and peer-connected events, and its signaling TCP connection is established. All five CPA node
memberships/plugins are healthy; native Sonnet/Opus requests returned HTTP 200. User-level UI
automation timed out, so the verification uses supported CLI, networking and application logs.

Inbound probes from Ark to this MacBook's built-in SSH/VNC ports failed; localhost probes
confirm those services are not listening. This is distinct from Tailscale transport recovery.
The user was asked whether the intended access is Workbench, SSH or Screen Sharing. No inbound
services were enabled without resolving that scope.

Private diagnostics/preferences: `/tmp/macbook-tailscale-repair-20261029/`. Preferences are private;
do not print raw LocalAPI credentials, node keys or authentication URLs. No diagnostics were
uploaded to Tailscale or posted publicly.

## Claude errors are independent provider limits

Home's credential errors contain Anthropic `rate_limit_error`, not transport/authentication
failures. Eight shared accounts report exhausted weekly windows: Gmail, HypeSports, 82, Chip,
Developer, Moxy, Lara and Vanderblooms. Private Microsoft reports its monthly spending limit.
Existing identities, routing priorities and genuine cooldowns were preserved.

Hyper Rally is active with a healthy recent quota snapshot. Real MacBook Sonnet/Opus completions
are attributed to it in Home records **1840633** (Sonnet) and **1840636** (Opus). A subsequent post-VPN-restart Sonnet
request also passed. Hotmail's five-hour reset was due at 21:40 CDT; its old exhausted snapshot
was stale. A targeted normal `/quota/collect` recheck succeeded at **2026-10-30 02:45:01 UTC**:
Hotmail is active/available, five-hour utilization 0%, weekly 13%, fresh healthy quota. Hyper's
recent five-hour/weekly figures are 18%/67%. No quota was reset or fabricated, and no tokens,
account bindings, account priorities or Home/node binaries were changed.

## October 31 account-error follow-up

At 12:03–12:05 UTC, Home's current data confirms exhausted weekly quotas on Claude Gmail,
HypeSports, 82, Chip, Developer, Lara and Vanderblooms. Six show credential errors; Lara is
active but its quota is exhausted. Moxy is active with 1% weekly remaining and extra usage
available. Hotmail and Hyper Rally's stale healthy snapshots were refreshed through scoped
normal collection and both remain healthy. No inspected credential error is an OAuth 401.

Codex Gmail/HypeSports return usage-limit errors and active probes confirm 100% weekly used.
Their resets are November 4 at 07:17 UTC / 06:40 UTC respectively. Developer has 28% weekly
remaining; Hotmail 94%. Scoped normal Codex collection completed successfully for Gmail,
HypeSports and Hotmail. Developer's next real response projected a partial header snapshot
again: `source=response_header`, no collection error, zero failures. A partial observation is
not evidence of broken authentication.

Real native requests through the MacBook both passed: Claude Sonnet 5.5 selected Moxy
(Home record **2016017**), and Codex Sol 6.1 selected Developer (**2016020**). All five nodes
are healthy with plugin reports `reported_ok`; account priorities/scopes were preserved.

Read-only inspection of Vanderblooms' stored model state confirms a retained HTTP 404
`not_found_error` for `claude-3-7-sonnet-20250219`, last updated October 29. No matching request
was found in the preceding 24-hour bounded usage query. `hasModelError` retains the global
error while any model's `LastError` remains; a successful request on a different model cannot
clear it. This is a per-model error in addition to that account's real exhausted weekly quota.
No unsupported model was retried merely to clear a badge, and no genuine quota was reset.
