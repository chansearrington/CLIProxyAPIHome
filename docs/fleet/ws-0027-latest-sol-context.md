# Released fleet updates and Claude Code Sol 1M option

Status: deployed and verified — Home 1.1.1, five CPA 8.0.29 nodes, five Claude Code 2.1.300 clients, and Mac CodexBar 0.75.0. The existing large usage-overview timeout remains. Final native acceptance passes; all four fleet locks released and independently confirmed unlocked.

The user authorized checking and updating Home, nodes and related components, and requested
GPT 6.1 Sol with high reasoning and a 1 million token context in Claude Code's model picker.

## Baseline and scope

- Home: `cpa-home:1.1.0-claude-fleet-4e35cd4`, image
  `sha256:6c082596440ce369d22004b8cbbfb24dfa45500671adc12feacd8c382a1fb817`.
- Five healthy CPA 8.0.27 nodes; Copilot fork 0.3.7 loaded and reported healthy.
- Latest released Home is 1.1.1, CPA 8.0.29. Copilot fork 0.3.7 is current.
- Claude Code on the Mac is already 2.1.300, matching the current upstream changelog.
- The Mac node already exposes `gpt-6.1-sol`. A real `/v1/messages` request to
  `gpt-6.1-sol(high)` returned HTTP 200 and the requested marker. The saved Claude Code
  picker instead lists the older GPT 5.6 Sol; menu configuration is the immediate gap.
- The separately released CLIProxyAPI Management Center is not Home's panel. Home builds
  embed `router-for-me/Home-Management-Center`; do not substitute the other product's UI.

## Guards and evidence

All four canonical fleet locks acquired with a background heartbeat keeper. Before any Home
write, a detached `VACUUM INTO` backup finished integrity, SHA256 and permissions checks.
All Go validation/builds run only on Ark inside `golang:1.26-bookworm`, with caches and source
on the array and an executable 3 GB tmpfs. Current Docker disk headroom is 93 GB.

Home integration merges the released `v1.1.1` into current `origin/fleet` without conflicts:
PR [#17](https://github.com/chansearrington/CLIProxyAPIHome/pull/17), merged at
`f6ebd6336c194748448ffdbde6292dcffe9bb734`. Local review of `3df71d03` returned
MUST 0, SHOULD 0, NICE 0. Full Home tests and compile passed on the integration tree and
again after the final merge/panel staging. No Go toolchain ran on the Mac.

Verified release archive digests:

- CPA 8.0.29 darwin arm64: `70fcb92a7e045dcbcb52cc63854ef723c1cb77c720923faa45d6081eeece21d0`.
- Official Home 1.1.1 linux amd64: `0eb295dbe9f38398bf2e8edcb5f6da229668b48cd5946febe398dd4e9cf2ebef`.
- CodexBar 0.75.0 universal app: `98d7d415f98b1fe1aaf89b394ea423c8a9acb57c753399bb6597f576f08dc8fc`.

## Backup, image and compatibility

- Backup: `/mnt/user/appdata/cpa-home/data/backups/home-pre-latest-sol-20261015T035102Z.db`;
  6,432,350,208 bytes, mode 0600, integrity `ok`, SHA256
  `ebff5ccd213b754f82e3c469f48a3a58bac20d8e22e41a7467cbb50e088161f9`.
- Final image: `cpa-home:1.1.1-claude-fleet-f6ebd63`, immutable ID
  `sha256:76499632b410686ec14a68abbd26360369bd82cbace1ed77bc4617a2c207ba72`.
- Final exact-input cutover: 2026-10-15 04:20:58.009 UTC; container started 04:20:58.316 UTC.
- Image-only rollback: the baseline immutable image above, additionally tagged
  `cpa-home:rollback-ws0027`. Saved compose:
  `/mnt/user/appdata/cpa-home-build/ws0027/compose-pre-deploy.yml`.
  Restore that compose image and recreate service `home`; retain the latest DB/OAuth tokens.
- An isolated copy of the backup was opened/exported by the new binary and then the old
  image, both with network disabled. All 15 exported files matched; database quick check
  passed. No shadow instance could refresh production credentials.
- Mirrored and independently verified the previous 64 served panel files. Extracted the
  same-release Home 1.1.1 panel from its verified official executable in an isolated,
  network-disabled container with fixture configuration. All 64 newly served assets match
  that official bundle. The separate CPA Management Center release was not installed.

## Claude Code model option

User settings now contain an appended picker entry:

```json
{
  "model": "gpt-6.1-sol(high)[1m]",
  "label": "GPT 6.1 Sol — High · 1M",
  "description": "OpenAI Codex subscription through CPA; high reasoning; 1,000,000-token context"
}
```

The matching per-model settings set high effort and a 900,000-token auto-compact window.
The canonical `gpt-6.1-sol(high)` settings also carry these values. Other picker rows,
tool-search settings, hooks, plugins and unrelated provider context limits are retained.
No global maximum-context environment override was added.

Verified in the actual interactive `/model` picker and `/autocompact` screen on 2.1.300.
Claude Code treats the unknown model's `[1m]` annotation as a 1,000,000-token client window
and strips it before sending `gpt-6.1-sol(high)` to CPA. A recording forwarding proxy
captured `thinking.type=adaptive` and `output_config.effort=high`; requests reached CPA and
completed. The `(high)` suffix also requests high effort through CPA's option parser.

A temporary Home OAuth alias used during investigation was unnecessary and was removed
with a guarded inverse. The original absent alias configuration was restored; no permanent
Home alias or permission change is required. Ordinary `gpt-6.1-sol` remains exposed normally.

Validation went beyond the displayed window:

- A real large Claude-format request through the updated Mac node completed in 13.7 seconds,
  returned its correct marker, and recorded **900,341 input tokens** plus 14 output tokens
  in Home record **446434**, Codex provider, original Codex credential, correct Mac node.
- Fresh Claude Code reported `contextWindow: 1000000`. A traced run recorded a real Bash
  tool call, matching successful tool result, final marker and no permission denials.
- Claude Code still warns internally that this is an unrecognized custom model ID. Its
  client-side cost estimate is marked unknown and max output defaults to 32K; no claim is
  made that Claude Code recognizes every OpenAI-specific capability or price.

Select **GPT 6.1 Sol — High · 1M** in `/model`, or start with:

```sh
claude --model 'gpt-6.1-sol(high)[1m]'
```

## Fleet acceptance

- All six client keys passed Claude Sonnet 5.5 and Sol high requests before deployment,
  after each Home cutover, and on the Mac canary after its node upgrade. Chanse's private
  Copilot request passed. The complete client-key/group/binding structures and all 14
  credentials' static settings matched baseline independently of rotating tokens/runtime state.
- Node executable: CPA 8.0.29, commit `5fc580b1`, 64,510,706 bytes, SHA256
  `d6502c8294c3e374bc2845aa1ee6fa20a218efcec1c94face732896244308d87`.
- Restart-in-place order/times UTC: Mac 04:00:54, Moxy 04:01:46, Hyper 04:02:29,
  Lara 04:03:25, Chip 04:04:26. Every node passed readiness, banner, exact manifest/hash,
  Home membership/plugin reports, and its own client-key Claude/Sol completions.
  Each retains the verified `cli-proxy-api.8.0.27.bak`; no service was stopped/unloaded.
- All four minis' Claude Code clients updated 2.1.286 → 2.1.300 with byte-identical settings.
  Original executables/settings are retained in each node service's `ws0027-client-backup`.
  The Mac was already current. Codex CLI 0.162.1 and Copilot fork 0.3.7 were already current.

## Dashboard failure, rollback and comparison

The first post-deploy dashboard probe failed, so Home was rolled back before diagnosis.
The aggregate probe also omitted required `group_by`; its initial 400 was a probe error,
not a server regression. The corrected request passes on both images.

With the exact fixed range 2026-10-14 04:00 → 2026-10-15 04:00 UTC:

| Endpoint | Previous image | Home 1.1.1 |
| --- | --- | --- |
| Overview, 24h | 500, deadline exceeded, 11.16 s | Same 500/error, 10.59 s |
| Aggregates, `group_by=model` | 200, 5.40 s | 200, 5.23 s |
| Usage records | 200, 0.04 s | 200, 0.08 s |
| Request events | 200, 0.04 s | 200, 0.08 s |
| Overview, fixed 1h range | 200, 0.08 s | 200, 0.13 s |

After establishing baseline parity, the candidate was redeployed and native routing passed
again. The new `/credentials/model-states` endpoint also returns 200. The large overview
query remains an existing performance limitation, now reproducible at this 24h range on the
larger database; this update does not claim to fix it or increase its timeout.

## CodexBar 0.75.0

- Verified release digest and app signature, then tested the candidate against a private
  config copy. An initial stale copied 82 access token failed; after reading the current
  Home-synced config, all ten OAuth usage requests passed. No Home authorization was replaced.
- Installed app 0.75.0 (168). Old signed app and private config/preferences backups retained at
  `/private/tmp/cpa-latest-upgrade/codexbar-rollback/`.
- Installed default-source CLI also returns usage for all ten Claude accounts. The new
  provider routing code treats the selected OAuth token account as authoritative.
- All prior provider/account settings are preserved, apart from current access-token/account
  selection changes. Normal migration adds a Claude reset-visibility marker and 35 new
  disabled provider entries; none was enabled. Home remains the refresh-token owner.
- OpenAI Web Extras and warning sounds remain disabled; silent notifications remain enabled.
  A foreground 3-second sample about four minutes after launch shows the normal main-thread
  event loop, without the previously captured audio/WebKit block signatures. Earlier attempted
  background samples did not create captures and are not used as evidence. This is a bounded
  observation, not proof that every future UI hang is eliminated.
- The scheduled token-sync/account-follow job succeeded after installation with all ten
  mappings, selecting 82 in agreement with the latest successful Mac/key-6 Claude usage.

## Upstream audit notes

Latest-release APIs were rechecked after deployment: Home 1.1.1, CPA 8.0.29, Claude Code
2.1.300, CodexBar 0.75.0, Codex CLI 0.162.1 and Copilot fork 0.3.7 remain the current releases.
Unreleased Home `dev` changes were inspected but not substituted for the released update.

Recent reports reviewed include CPA #6631 (standalone file-backed config write race),
#6637 (model-catalog client-version negotiation), and #6643 (concurrent success clearing a
newer quota cooldown, reported with Antigravity). Home uses its own DB-backed serialized
configuration path, and Claude Code's configured picker bypasses the reported discovery
gap. These reports are not all claimed fixed: Home's success transition also resets model
state without a request-start fence; a cooldown chronology correction needs
separate deterministic regression tests and CPA/Home contract analysis before it can be
claimed resolved.
The performed acceptance establishes current tested routing, not the absence of every
upstream issue. Saved primary issue evidence is in the private working directory.

Private working evidence: `/private/tmp/cpa-latest-upgrade/` on the Mac and
`/mnt/user/appdata/cpa-home-build/ws0027/` on Ark. Never commit configuration secrets or
OAuth material. Image rollback retains the current database and rotating tokens.

## Exact source-input verification

A final strict input check found that the shared Go check command's `GOFLAGS=-mod=mod`
removed the `// indirect` annotation from the already pinned `golang.org/x/sys v0.47.0`
requirement. The existing Linux subscription transport imports `x/sys/unix` directly.
No dependency version or Go source changed. Home was rolled back before resolving the
provenance discrepancy.

Restored the exact reviewed `go.mod`, then ran the full suite and compile with
`GOFLAGS=-mod=readonly`; both pass without editing it. Independently compared all **539**
tracked Go/build inputs against the reviewed checkout; all match. Rebuilding from those
exact inputs reproduced the **same immutable image ID** `sha256:76499632b410686ec14a68abbd26360369bd82cbace1ed77bc4617a2c207ba72`.
Thus the annotation normalization did not affect the deployed image. The exact-input build
was redeployed and all six keys passed Claude/Sol again, with the private Copilot request
also passing. Existing dashboard parity applies to this identical immutable image.

The stored priorities and model settings of all **15 active DB auth records** also match
the verified pre-change backup. This is separate from the 14 entries returned by the
credential-list API. No backup DB was restored onto production.

The offline update guide was rendered in headless Chrome and visually checked. Its section
IDs and local links were validated.

Close-out: all five memberships/plugins remain healthy after the final restart; exact
image ID confirmed. All four canonical locks released at 04:22:41–44 UTC and independently
checked unlocked. The source PR is merged; runtime/guide records are committed on `fleet`.
