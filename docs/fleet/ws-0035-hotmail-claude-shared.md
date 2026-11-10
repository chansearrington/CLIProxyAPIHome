# Hotmail Claude account shared setup

Status: complete — Hotmail shared at priority 2; eleven-account CodexBar sync, six client keys, Sonnet/Opus and four isolated new-account node proofs pass; policy restored and locks released. Lara completed upgrade setup, restored Workbench/gateway/CPA and passed a native Claude request; all five memberships healthy.

The user added `chansearrington@hotmail.com` as a Claude subscription and requested the same
CPA Home/CodexBar setup as the other Claude accounts. Home already has the enabled, active
credential `8abb895e-9809-4be0-b2e4-21d1fa3081d0`, created October 29. It initially has no
priority or channel binding. This is separate from the existing Codex account with the same email.

Anthropic's OAuth profile independently confirmed the exact email, an active `claude_pro`
organization, Stripe subscription and `default_claude_ai` rate-limit tier. Do not infer Max
status from the other subscriptions. Preserve the original credential UUID/history and Home's
sole refresh ownership.

## Routing and guards

Add Hotmail to shared channel group 1, available to the six original client keys. Preserve the
existing shared Claude order, placing Hotmail after Hyper Rally and before private Microsoft.
Because shared priorities currently occupy every integer from 2 through 10, increment those
nine priorities by one and give Hotmail priority 2. Microsoft remains priority 1 in private group 2;
Codex priorities, existing key scopes and other provider settings remain unchanged.

Home's full fleet policy was saved, its active Chip catalog task was allowed to finish, then
policy was paused. The first pause attempt encountered the active-task guard; the successful
retry confirmed no active task. All four canonical locks returned `acquired:` and renew
independently. Home reports all five fleet worker versions at CPA 8.0.47.

Lara subsequently lost reachability and its lock renewal failed. The pending mutation driver
was stopped before any credential/access write. Tailscale reports Lara offline, and direct
LAN/tailnet SSH probes time out. Independent per-node renewal preserves the other locks and
retries Lara without stealing; fresh positive checks on all four are required before resuming.
The user was asked to bring Lara online. This does not change the completed local CodexBar setup.

During the initial blocked attempt, the complete original maintenance policy was restored before yielding, and the Moxy/Chip/Hyper
locks were released. No unattended mutation driver remains. Home readback confirms Hotmail's
priority remains unset and no channel binding exists. The detached backup continues read-only
validation and will leave its result under `ws0035/backup.json`; verify it and reacquire all four
locks before resuming. This pending state was superseded by the successful resumption below.

A detached `VACUUM INTO` backup of the approximately 21.5 GB live database must pass full
integrity, size/mode and SHA256 checks before credential/access writes. Offline integrity
validation uses a bounded 256 MiB cache, without changing live SQLite settings. Private evidence:
`/mnt/user/appdata/cpa-home-build/ws0035/` and `/tmp/cpa-hotmail-claude/`.

Completed verified backup: `/mnt/user/appdata/cpa-home/data/backups/home-pre-hotmail-claude-20261029T170251Z.db`,
18,021,421,056 bytes, mode 0600, full integrity `ok`, SHA256
`33e07c23e83898acad4756098e5b3d88d35a16994c449474db9b5eb613fdc8c3`.

After the user restored Lara's SSH, her normal Tailscale/Ethernet routes remained intermittent.
Wi-Fi `192.168.1.226` authenticated against the already trusted Lara host identity. A temporary
fleet-hosts override used that verified route for locks/checks; no fleet host map or routing
policy was permanently changed. All four locks were reacquired/renewed, with a release and
fresh `acquired:` for Lara to remove ambiguity from an interrupted prior acquisition.

## CodexBar

Added account `5FA01F0F-A69A-4037-93DB-E96A30629704`, labeled with the exact Hotmail email,
to the front of the stored Claude list for stacked-menu visibility. Existing selected UUID,
the other ten accounts and unrelated providers/settings are preserved. The new Home UUID/email
is pinned in the existing 120-second sync settings. Only the access token is projected locally;
no refresh token or Management password leaves Ark.

The provider profile and staged CodexBar 0.82.0 OAuth usage checks succeeded for the new account.
All eleven accounts were returned; ten succeeded and Hyper Rally's usage endpoint reported
an Anthropic rate limit. The installed helper's eleven-account sync succeeded at 17:04:39 UTC.
Config/settings updates held the shared sync lock and config lock, used private snapshots and
atomic compare-and-swap publication, and preserved current rotating tokens. Native app-control
inspection timed out, so no visual menu proof is claimed.

## Verification and rollback

Applied shared binding **28** and Hotmail priority **2**. Existing shared order is preserved:
Gmail 11, HypeSports 10, 82 9, Chip 8, Developer 7, Moxy 6, Lara 5, Vanderblooms 4, Hyper Rally 3;
private Microsoft remains 1. All six original client keys completed native Sonnet 5.5 requests;
Home attributed usage records **1805340, 1805344, 1805350, 1805353, 1805358, 1805367** to
Vanderblooms for keys 1–6, respectively. Actual eligible traffic remains authoritative;
existing model errors/cooldowns were not cleared or priorities forced for verification.

A temporary group containing only Hotmail and a random temporary client key isolated the new
account without changing production preference. Exact new-account usage:

| Node/model | Home usage record |
| --- | --- |
| MacBook Sonnet 5.5 | 1805383 |
| MacBook Opus 5.5 | 1805388 |
| Moxy Sonnet 5.5 | 1805405 |
| Hyper Sonnet 5.5 | 1805407 |
| Chip Sonnet 5.5 | 1805403 |

Temporary verification key/group/binding were removed. Original keys, channel groups/bindings,
model-group details and full inference-config fingerprint match their baseline. Credential
identity/settings match except the ten intended priority changes. The Home quota probe at
17:34:05 UTC succeeded with fresh healthy Pro windows and zero collection failures.
Four existing online memberships/plugin reports remain healthy. No Home image, CPA binary,
native CLI auth/settings or OpenClaw configuration changed.

The eleven-account CodexBar sync succeeded again at 17:38:34 UTC. It follows actual Mac/key-6
Vanderblooms traffic, while Hotmail remains correctly mapped for future fallback. The complete
pre-resumption Home maintenance policy was restored, and all four operation-owned locks released.

Moxy and Lara's node processes were already intentionally stopped before this operation;
their logs record clean intentional exits and their loopback listeners are absent. Their SSH
and fleet locks work. MacBook/Chip are registered healthy, and Hyper's existing listener serves
models. Preserve the stopped services and report verification limits rather than restarting
them merely to make an account-addition check green. Moxy subsequently returned through existing
services before final acceptance and passed; Lara's stopped CPA service was preserved.

## Lara Workbench recovery

The user's follow-up requested help with Workbench after SSH returned. Astropad Workbench 1.3.0
was installed but absent from processes. It was started as Lara's logged-in desktop user.
The user still saw her offline. A process sample shows Workbench blocked in AppKit
`NSApplication finishLaunching` / `_NSPersistentUIEstablishTalagentCommunication` before its
own networking/logging starts. macOS reports 27.2 (26B123), with `Setup Assistant Update` active;
the user confirmed an OS upgrade with setup screens still pending.

Built-in Screen Sharing is installed but not loaded/listening (5900/3283 closed), and Lara's SSH
session has no noninteractive sudo. Workbench is running but remote usability is not restored.
The user was asked whether they have an administrator SSH terminal or a monitor/KVM to complete
the setup; no setup-state files, licensing/authentication, remote-access privileges or OS security
settings were bypassed or changed. This desktop issue does not block the completed Home setup.

Follow-up: the user ran `launchctl enable` / `bootstrap` with sudo. Screen Sharing's daemon is
now running, and Lara is already a member of its allowed-access group, but the client reports
"Screen Sharing is not permitted". Repeated bootstrap is not a remedy for this authorization
failure. The previous SSH-only recovery instructions were insufficient. macOS's upgrade Setup
Assistant remains active; the Mac is not enrolled in MDM, and `laras-kvm` is offline in Tailscale.
Apple documents GUI or MDM activation on modern macOS:
https://support.apple.com/guide/remote-desktop/use-the-kickstart-command-apd8b1c65bd/mac
Recovery needs a local display/keyboard or working hardware KVM to complete setup and re-enable
Screen Sharing through System Settings. No TCC database, setup-state marker or privacy protection
was modified to bypass that requirement.

Resolved after the user connected a physical keyboard/display and completed macOS setup.
Read-only verification confirms Setup Assistant exited, Workbench is running, Screen Sharing
listens on 5900, and Lara's gateway and CPA node are running. A native Sonnet 5.5 request through
Lara's loopback node using her original client key 5 returned HTTP 200 and the exact reply;
Home usage **1811514** attributes it to Lara's node and the eligible Vanderblooms credential.
All five Home memberships/plugins are healthy again. Hotmail remains active, shared by binding
28 at priority 2; the eleven-account CodexBar sync succeeded at 18:15:11 UTC. No settings or
services were changed during this final verification.

Rollback restores only changed priorities and removes the new shared binding and temporary
key/group/binding. Retain credential identity/history and fresh authorizations. Never restore a
stale whole database over rotating tokens. Local rollback removes only the new CodexBar UUID and
its helper mapping under existing locks, preserving the current other accounts/settings/tokens.
