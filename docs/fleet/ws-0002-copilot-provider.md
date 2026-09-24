# ws-0002 — GitHub Copilot as a provider across the fleet

Workstream brief for the `fleet` fork of CLIProxyAPIHome. Created 2026-09-24.

- Branch: `ws-0002/copilot-provider` (based on `fleet` @ `5c212e9`, the post-ws-0001 line: upstream 1.0.73 @ `04fac5f` + fork commits; rebased 2026-09-24 from the pre-ws-0001 base `9d8bf4e`)
- Worktree: `.claude/worktrees/ws-0002-copilot-provider`
- herdr space: `CLIProxyAPIHome | ws-0002 | Copilot provider` (agent alias `cpahome-ws-0002`)
- ws-0001 (upstream sync) landed before this work started; no further rebase is planned. Nothing
  here goes to upstream Home unless it is a clean fix.

## Goal

Every fleet node can serve GitHub Copilot models through credentials Home manages. A Copilot
subscription is **one more account with its own allowance**: its models are exposed with correct
labels, none are excluded, and for any model that other accounts also offer, the Copilot allowance
is pooled with theirs so a request is served by whichever account has quota.

## Chanse's decisions (2026-09-24, verbatim intent)

- Do NOT exclude any model, Claude included. Label them correctly.
- Multiple accounts matter, including multiple Copilot accounts. Each is a separate credential
  with its own token allowance.
- The allowance under a Copilot account must be usable alongside the allowance under the other
  accounts — "rolled into the rest of the tokens" — not siloed.

## Facts at start (verified 2026-09-24)

- Upstream CPA has **no native Copilot provider**. Maintainer position (issue #4317, 2026-07-15):
  Copilot sits behind Microsoft/GitHub's gateway, so it will not be a top-level provider.
- CPA PR #5661 (NaveDanan, 2026-09-09, +1423/-16 in 29 files) adds a native provider with device
  login and quota. Bot-reviewed only, no maintainer review, base `dev`. Do not depend on it.
- Community plugin **`arthur-sommer-etc/cliproxyapi-copilot-plugin`** (MIT, Go): official plugin
  ABI v1 / registration schema 2 via `sdk/pluginabi` + `sdk/pluginapi`, written against CPA
  v7.2.118. Registers `AuthProvider` (GitHub device-code OAuth, host-owned credential storage),
  `ModelProvider` (discovery from Copilot `/models`), `ProviderExecutor` (non-stream, SSE,
  restricted HTTP). Releases: v0.3.3 (2026-08-05); last push 2026-09-08; 2 open issues. It
  supports excluding model prefixes to avoid collisions — **we will not use that** (see Design).
  Uses the public device-flow client id `Iv1.b507a08c87ecfe98` (not a secret); GitHub token goes
  to the host's auth storage; the short-lived Copilot token stays in process memory.
- Upstream Home already has a plugin story: `internal/cluster/management/plugin_store*.go`,
  `plugin_oauth.go`, `internal/cluster/plugin_tasks.go`, `internal/home/plugin_sync*.go`;
  Management API routes `/plugins`, `/plugin-store`, `/plugin-store/:id/install|uninstall`,
  `/plugin-store-auth[...]`; header `X-CPA-SUPPORT-PLUGIN` = 1 only for CGO builds (our
  Dockerfile is `CGO_ENABLED=1`).
- Fleet nodes run CPA 7.2.15x (ws-0001's lead recorded 7.2.159 — verify on a box). Latest CPA is
  v7.3.16. Plugin framework is ON on the nodes; zero plugins installed (agent-os memory).
- Home has channel groups and model groups (`internal/cluster/`); CPA has model aliases. These
  are the tools for "label correctly AND pool".
- No Go toolchain on the laptop. Builds only on the Ark inside `golang:1.26-bookworm`
  (`FLEET.md`). The plugin's own build also targets `golang:1.26-bookworm`.

## Design question to answer FIRST (deliverable, needs Chanse's yes)

"Label correctly" and "pool with other accounts" conflict if done naively: a distinct label is a
distinct model name, and CPA pools credentials per model name. Produce a table:

| Copilot model id (as discovered) | Native equivalent on other accounts (exact id) | Fleet-facing name | Pooled? | Notes (behaviour differences) |

and a proposed mechanism (Home model group / CPA alias / plugin config) that gives every Copilot
model a visible, correctly-labelled entry AND puts it in the same pool as the same model on the
other accounts. Call out any real behaviour differences when a request lands on Copilot instead of
the native account (context limit, tool use, caching, streaming), because pooling means a session
may switch accounts mid-conversation. Recommend, don't decide.

## Scoping questions (deliverable 1, with file/line and version evidence)

1. Does Home's plugin store accept a third-party plugin (a URL/artifact we host) or only a
   curated list? If only curated: what is the smallest Home change to allow ours?
2. Does the nodes' CPA version satisfy the plugin's ABI v1 / schema 2? Verify against
   `sdk/pluginabi` at the exact CPA tag the minis run, not at `main`.
3. Does the GitHub device-code login flow work through Home's `/plugin-store-auth` so the
   credential is Home-managed and distributed to every node, or is it node-local only? Multiple
   Copilot accounts must each be a separate credential.
4. Does usage/allowance from Copilot flow into Home's usage ingestion and the quota view like the
   other accounts, so the sentinel-style "disable when exhausted" pattern works?
5. Route check: plugin (recommended) vs vendoring PR #5661's native provider into a CPA fork.
   State the maintenance cost of each honestly.

## Acceptance criteria (verifiable, not asserted)

1. Scoping report answering 1–5 in this doc, with evidence.
2. Model mapping table + mechanism approved by Chanse in plain text in the ws-0002 pane.
3. Plugin built reproducibly on the Ark (`golang:1.26-bookworm`), source pinned by commit SHA,
   artifact checksum recorded here. No build on the laptop.
4. Installed on ONE canary node first (propose which; the MacBook's own local node is the
   natural candidate because it is not a fleet agent's node). Node stays healthy; the other nodes
   untouched.
5. Copilot account(s) logged in via the agreed flow; nothing secret in this repo — obviously-fake
   values only in fixtures.
6. All discovered Copilot models appear in the canary node's model list with the agreed labels;
   none excluded.
7. Pooling proven with evidence: for one shared model, a request is served by the Copilot
   credential while the native account is disabled/exhausted, and by the native account when the
   Copilot one is; Home's usage view attributes each to the right credential.
8. Any Home code change: tests in the container, `docs/management/api.md` updated if the API
   changed, `gofmt`/vet clean (known refresh.go vet line excepted).
9. Full suite green on the Ark; rollout to the remaining nodes and any Home image only on a
   decided `ship` card. Never stop a mini's `cpa-home-node`; restart in place.

## Tasks

1. ‖ Read the plugin source (auth storage contract, model discovery, exclusion config, executor).
2. ‖ Read Home's plugin store / plugin-store-auth / plugin-sync code paths on `upstream/dev`.
3. ‖ Read CPA `sdk/pluginabi` + `sdk/pluginapi` at the minis' exact tag; read PR #5661 for the
   native design as a comparison.
   (1–3 are independent; use subagents.)
4. Scoping report (criterion 1) + model mapping proposal (design question). **STOP — Chanse's yes.**
5. Build the plugin on the Ark (criterion 3). **STOP** before touching any node.
6. Canary install + login + model list (criteria 4–6). One node only.
7. Pooling proof (criterion 7).
8. Home changes if any (criterion 8).
9. Full tests on the Ark, close-out here, ship card.

## Stop points

Before: installing anything on any node or the Ark; logging in any account; any `git push`;
any Home image build. Ask in plain text in this pane; no AskUserQuestion dialogs.

## Scoping report (task 4, written 2026-09-24)

Research inputs, all read-only: the plugin source at commit `7f16b60` (tag v0.3.3 + 2 docs
commits), CPA at `ac02da6c` (v7.2.159) and `c404af96` (v7.3.16), Home at `upstream/dev`
`04fac5f` (= the `fleet` base, 1.0.73), Home's SQLite on the Ark, every node's binary, and the
public Copilot documentation. Full research notes with file:line evidence are in
`docs/fleet/ws-0002-research/` (four files). Nothing was installed, built, logged in, or pushed.

### Facts that changed or were wrong in the brief

1. **The nodes run CPA 7.3.16, not 7.2.15x.** Verified on every box at 09:16Z with
   `cli-proxy-api --version`: `Version: 7.3.16, Commit: c404af96, BuiltAt: 2026-09-24T00:11:20Z`
   on moxy, lara, chip, hyper and the MacBook. At 08:57Z they were still 7.2.159; another
   workstream upgraded them in between (install manifests record `core_upgrade` 7.2.159 → 7.3.16,
   reason: the Claude Code version cloak). Scoping below holds for both commits; the plugin SDK
   changes between them are additive (schema stays 6, ABI stays 1).
2. **The nodes are macOS arm64, not Linux.** All five binaries are `Mach-O 64-bit executable
   arm64`, built on a GitHub macOS runner, cgo-enabled (the binary contains the
   `pluginhost._Cfunc_cliproxy_dlopen` symbol and `loader_unix.go`). The darwin release has no
   `no-plugin` variant. So plugins CAN load on the nodes, but they need a **`darwin/arm64`
   `.dylib`**, which the community plugin does not publish (linux/amd64 `.so` only).
3. **Home already pushes `plugins: {enabled: true}`** to every node (Home DB `config` table);
   the nodes' bootstrap `home-runtime.yaml` says `false` but Home's copy wins at runtime. The
   Ark's `plugins/` bind mount is empty and all four plugin tables (`plugin_status`,
   `plugin_tasks`, `plugin_store_auth`, `plugin_store_auth_key`) have zero rows.
4. **Home compiles against CPA SDK `v7.2.83`** (`go.mod:11`, same on `upstream/dev` and
   `fleet`). That SDK's plugin host speaks registration **schema 1** and refuses any plugin
   declaring a higher schema. This is the single hard blocker found (Q2).
5. **Copilot billing changed on 2026-06-01.** "Premium requests" only survive on legacy annual
   plans. Every current plan gets a monthly pool of AI credits (1 credit = $0.01) charged per
   token at the vendor list price. Pro = 1,500/month, Pro+ = 7,000, Max = 20,000, Business =
   1,900/seat pooled, Enterprise = 3,900/seat pooled. Pro cannot see Opus/Fable/GPT-5.5/Sol.
   When exhausted: Pro/Pro+/Max stop unless a USD budget for extra usage is set; Business/
   Enterprise overage is on by default and an admin can turn it into a hard stop.
6. The laptop does have `go1.26.5` installed (contrary to FLEET.md). It was not used and the
   rule stands: nothing is built on the laptop.

### Q1. Does Home's plugin store accept a third-party plugin? — Yes, with zero Go changes

- The store is a catalogue + config writer, not an artifact host. `GET /plugin-store` fetches
  the official registry (`raw.githubusercontent.com/router-for-me/CLIProxyAPI-Plugins-Store/
  main/registry.json`, hard-coded in the SDK) **plus every URL in the existing config key
  `plugins.store-sources`** (`internal/config/plugin_config.go:17-18`; SDK
  `internal/pluginstore/registry.go:86-111`). Tests already cover a private registry URL
  (`internal/cluster/management/plugin_store_test.go:460,647`).
- `POST /plugin-store/:id/install` (`plugin_store.go:232-301`) downloads nothing. It pins the
  registry entry's artifact list (`goos`, `goarch`, `url`, `sha256`) into the DB config under
  `plugins.configs.<id>.store`, sets `enabled: true`, and pushes the config to every node.
- Each node then asks Home `GET plugin-sync` over RESP/mTLS, gets one `{url, sha256}` for its
  own OS/arch, downloads it, verifies the checksum, writes `<id>-v<ver>.dylib` into its
  `plugins/darwin/arm64/` dir, loads it, and reports `plugin-status`
  (`internal/home/plugin_sync_plan.go:27-123`; SDK `internal/homeplugins/sync.go`).
- **If a node's platform has no artifact, its whole plugin-sync plan errors**
  (`plugin_sync_plan.go:126-131` + SDK `pluginstore.SelectArtifact`), so the manifest must carry
  both `linux/amd64` (for Home itself) and `darwin/arm64` (for the nodes).
- Three ways to register ours, cheapest first: (a) hand-write the `store` manifest into Home's
  config via `PUT /config.yaml` (schema-version 2, type `direct`, two artifacts) — no registry
  hosting at all; (b) host a `registry.json` on any HTTPS host and add it to
  `plugins.store-sources`, then install from the panel; (c) `github-release` type, which makes
  Home call `api.github.com` at install and at every sync. Recommendation: (a) for the canary,
  (b) later if we want the panel's update button.
- Docs mismatch to fix if we touch this area: `docs/management/api.md:897` says nodes resolve
  artifacts "from that registry"; the code pins them at install time.

### Q2. Does the plugin's ABI/schema fit? — Nodes yes, Home no (one-line fix)

| Side | ABI | Schema it speaks | Rule | Plugin declares | Result |
|---|---|---|---|---|---|
| Nodes, CPA 7.3.16 (`c404af96`) | 1 | 6 | ABI must equal; plugin schema must be ≤ host (`internal/pluginhost/loader_unix.go:154-157`, `rpc_client.go:75-77`) | ABI 1, schema 2 | **loads** |
| Nodes at 7.2.159 (`ac02da6c`) | 1 | 6 | same | same | loads |
| Home, SDK v7.2.83 | 1 | **1** | same rule (`rpc_client.go:67` in that SDK) | schema **2** | **refused** |

- Schema 2 (CPA commit `30efd7c4`, 2026-07-28) only added the request-lifecycle method
  `request.complete`. The Copilot plugin does not use it, and every method, capability flag and
  `ModelInfo`/`AuthData` field it does use already exists in v7.2.83 (checked field by field).
- Plugins are cgo `dlopen` shared libraries speaking JSON, not Go `plugin` packages: the plugin
  does not need the host's Go toolchain or module versions, only a cgo host on the same OS/arch.
- **Fix options:** (1) patch the plugin to declare `schema_version: 1` (one line at
  `cmd/cliproxyapi-copilot/dispatch.go:163`) — recommended, harmless on the nodes because 1 ≤ 6;
  (2) bump Home's CPA SDK pin from v7.2.83 to ≥ v7.2.103 — a large dependency move for Home with
  its own test risk; worth doing later as its own workstream, not on this critical path.
- Why Home must load the plugin at all: see Q3.

### Q3. Does the device-code login flow work through Home, fleet-wide? — Yes, if Home loads the plugin

- Correction to the brief: `/plugin-store-auth` is **not** provider login; it stores download
  credentials for private registries/artifacts. Provider login for a plugin is the generic
  `GET /v0/management/<provider>-auth-url` → `GET /get-auth-status?state=` poll, which only
  exists when the plugin is loaded **inside Home's process**
  (`internal/cluster/management/plugin_oauth.go:151` → `runtime.HasPluginAuthProvider`). That
  requires `plugins.configs.<id>.load-in-home: true`, a `linux/amd64` artifact, and Home built
  with CGO (it is: `Dockerfile:19`).
- The plugin's `StartLogin` returns GitHub's `verification_uri_complete` (the URL already
  contains the user code) so Home's known gap (it returns only `url` + `state`, not the plugin's
  metadata) does not bite. `PollLogin` does the device-code polling itself. The plugin never
  writes files; it returns an `AuthData` record and Home upserts it into the same `auth` table
  as every native credential (`plugin_oauth.go:229-236`), then dispatches it to nodes over RESP
  exactly like the others. **Not node-local.**
- **Multiple Copilot accounts:** the credential ID is derived from the GitHub login
  (`copilot-<login>.json`, `internal/provider/storage.go:100-118`), so two GitHub accounts are
  two credentials; logging the same account in twice overwrites it. One GitHub account can hold
  only one Copilot plan, so "multiple Copilot accounts" means multiple GitHub accounts (or one
  Copilot Business org with N seats, whose credits are pooled by GitHub).
- Model registration is the second reason Home must load the plugin: for a plugin provider Home
  gets the credential's model list by calling the loaded plugin's `ModelsForAuth`
  (`internal/home/models.go:214`). If no plugin is loaded for that provider, the credential ends
  up with **no models and is never dispatched** (`models.go:453`).
- Token lifecycle: the stored GitHub token (`gho_`, non-expiring) is the only secret persisted.
  Each node exchanges it for the short-lived Copilot token in memory (`copilot_internal/v2/
  token`), refreshed 5 min before expiry. Refresh of the GitHub token is a no-op re-emit.
- On a Home-managed node the plugin receives its stored JSON rebuilt from the auth's `metadata`
  (CPA `internal/pluginhost/adapters_executors.go:1047-1062`); Home flattens the plugin's storage
  JSON into metadata when saving (`internal/home/plugin_runtime.go:460-485`), so the round trip
  works without changes. Home's downstream sanitiser strips `refresh_token`; the plugin's field is
  `github_refresh_token` and is empty with the default client anyway.

### Q4. Does usage/allowance flow into Home like the other accounts? — Usage yes, quota view no

- **Usage/cost:** the node's usage reporter tags each record with `provider = "copilot"` and the
  dispatched `auth_index`; Home's usage ingestion has no provider allowlist and stores it verbatim
  (`internal/cluster/usage.go:107-245`). Per-credential usage, cost and the usage views work
  unchanged. Cosmetic: Home's token-accounting classifier has no rule for `copilot`, so the
  cache/reasoning breakdown may show as unclassified.
- **Quota dashboard / `quota_window`:** hard-coded allowlist `claude, antigravity, codex, kimi,
  xai` (`internal/cluster/quota_snapshots.go:1814-1821`). Copilot credentials will not appear in
  the quota page and there is no plugin hook to feed it. Adding it = extend the allowlist + write
  a collector against GitHub's `copilot_internal/user` credits endpoint (real code, ~1–2 days).
- **"Disable when exhausted":** GitHub answers an exhausted credit pool with **HTTP 402**. The
  plugin passes the upstream status through unchanged (`internal/provider/errors.go:29-40`). Home
  treats 402 as a **30-minute suspension of that model on that credential**
  (`internal/cliproxy/auth/result.go:261-268`), not the credential-wide quota state that 429
  triggers (`result.go:279`). Rate-limit 429s from GitHub (5-hour and weekly session limits) do
  hit the quota path. Practical effect: an exhausted Copilot account is retried every 30 minutes
  per model and each retry costs one failed request before Home moves on. Fix options: (1) patch
  the plugin to map 402 → 429 (PR #5661 does exactly this natively); (2) the sentinel-style
  poller, which for Copilot must read the credits endpoint itself because Home's quota view
  cannot. Recommend (1) now, (2) only if the dashboard matters.

### Q5. Route: plugin vs vendoring PR #5661 into a CPA fork

| | Community plugin, forked and patched by us | Vendor PR #5661 into a CPA fork |
|---|---|---|
| Upstream position | Plugin framework is upstream's sanctioned answer for non-first-party providers | Issue #4317 (2026-07-15): Copilot "will not be a top-level provider". PR open 15 days, bot review only, **CONFLICTING** against `dev` in 9 hot files (329 commits behind) |
| What we own | A 4.2k-line, single-author, dormant-since-2026-08-07 repo, plus our 4 patches (schema 1, 402→429, endpoint order, darwin build). Rebuild only when GitHub's internal API or the CPA plugin contract moves | 29 files, +1,423 lines inside CPA's core executor/registry/selector. Every CPA release (17 minor tags in 11 days recently) = re-resolve conflicts, rebuild the node binary for darwin/arm64 ourselves, redeploy all five nodes. We would be forking CPA for the first time |
| Home work | Same either way: alias fix, optional quota collector | Same, plus Home must learn the `github-copilot` auth shape |
| Failure blast radius | A plugin panic fuses the plugin, the node keeps serving native traffic | A bug is in-process in every node |
| Pooling / naming | Raw ids, tools = Home alias (below) | Raw ids, same tools; the PR's own docs point at prefix/alias |

**Recommendation: the plugin, forked under `chansearrington/cliproxyapi-copilot-plugin` so
the four patches are ours and pinned by SHA.** Vendoring #5661 is technically clean against
7.2.159 today but is a permanent CPA fork against an explicit upstream "no".

Honest caveats that apply to both routes: every third-party Copilot client (this plugin, #5661,
copilot-api, opencode's own path) impersonates VS Code Copilot Chat (client id
`Iv1.b507a08c87ecfe98`, `Copilot-Integration-Id: vscode-chat`, `X-Initiator: user` on every
call). GitHub's terms have no "authorised clients only" clause, but the Acceptable Use Policy
bans "excessive automated bulk activity" and GitHub tightened Copilot account vetting in 2026.
No 2026 blocking of unofficial clients was found. This is a risk Chanse accepts or not.

### Other constraints found

- **Building the darwin artifact.** A cgo `c-shared` library cannot be cross-compiled from
  the Ark's Linux container; it needs a Mac. The plugin's open PR #2 does it with GitHub macOS
  runners. Options: (a) GitHub Actions `macos-14` runner in our fork — reproducible, checksummed,
  no laptop toolchain, artefacts as release assets (recommended); (b) build on the MacBook with
  its existing Go — breaks FLEET.md's rule; (c) build on a mini — touches a fleet agent's box.
  The `linux/amd64` artifact for Home builds on the Ark in the existing
  `golang:1.26-bookworm` container, glibc-matched to Home's `debian:bookworm` image.
  macOS Gatekeeper may refuse an unsigned or quarantined `.dylib` loaded via `dlopen`; the
  canary step must ad-hoc sign it or clear the quarantine attribute (OS behaviour, not CPA).
- **Canary isolation.** All five fleet API keys are bound to channel group 1 ("Fleet shared",
  the five native credentials); a new Copilot credential outside that group should not be
  dispatched to them. Proposed canary: a second channel group (the five + Copilot) bound only to
  the MacBook's key. The plugin artifact itself still reaches every node via plugin-sync (that is
  how Home works); the minis simply load it and never receive a Copilot credential. Verify the
  channel-group filter end-to-end in task 6 before relying on it.
- **Home must be rebuilt and redeployed** for the alias fix (and to load the plugin), so this
  workstream ships a Home image, gated on a `ship` card like ws-0001.
- Client formats: the plugin only accepts Responses and Anthropic Messages, but CPA's host
  translates OpenAI chat-completions and Gemini requests into Claude format for it
  (`internal/pluginhost/adapters_executors.go:462-503`), so every endpoint the fleet uses works.

## Model mapping proposal (design question; needs Chanse's yes)

Sources: Copilot ids from models.dev's `github-copilot` provider (generated from the live
`/models` endpoint, last updated 2026-09-23) cross-checked against GitHub Docs; fleet ids from
the MacBook node's `/v1/models` today (54 ids). Ids marked † are documented on Copilot but I
could not verify the exact string without a login; task 6's first `/models` call settles them.

**Group A — same id on both sides: pools automatically, nothing to configure (13).**
Home's registry is keyed by the exact model id across all providers, so a Copilot credential
registering `claude-sonnet-5` joins the native credential's pool for `claude-sonnet-5`.

| Copilot id | Native id (fleet) | Fleet-facing name | Pooled? | Notes |
|---|---|---|---|---|
| `claude-sonnet-5` | `claude-sonnet-5` (claude) | `claude-sonnet-5` | yes | Copilot prompt cap 128K (native 200K/1M) |
| `claude-opus-5` | `claude-opus-5` (claude) | `claude-opus-5` | yes | 128K cap; Pro+/Max/Business only |
| `claude-fable-5` | `claude-fable-5` (claude) | `claude-fable-5` | yes | 128K cap; org admin must enable; retention terms |
| `gpt-5.5` | `gpt-5.5` (openai/codex) | `gpt-5.5` | yes | served via Azure OpenAI; `/responses` |
| `gpt-5.6-luna` / `-terra` / `-sol` | same (openai) | same | yes | Sol needs Pro+ |
| `gpt-6-astra` / `-luna` / `-sol` | same (openai) | same | yes | Astra/Sol need Pro+ |
| `grok-4.5` / `grok-4.6` / `grok-4.7` | same (xai) | same | yes | chat-completions only on Copilot |

**Group B — same model, different spelling: rename on the Copilot side so it pools (6).**
Copilot uses dots, Anthropic's API uses hyphens. Without a rename these are six extra,
un-pooled Claude entries.

| Copilot id | Native id (fleet) | Fleet-facing name | Pooled? | Notes |
|---|---|---|---|---|
| `claude-opus-5.5` | `claude-opus-5-5` | `claude-opus-5-5` | yes after alias | added to Copilot 2026-09-22 |
| `claude-fable-5.1` | `claude-fable-5-1` | `claude-fable-5-1` | yes after alias | admin-enable + retention terms on Copilot |
| `claude-opus-4.8` | `claude-opus-4-8` | `claude-opus-4-8` | yes after alias | |
| `claude-opus-4.7` | `claude-opus-4-7` | `claude-opus-4-7` | yes after alias | Copilot retires it 2026-10-02 |
| `claude-sonnet-4.6` | `claude-sonnet-4-6` | `claude-sonnet-4-6` | yes after alias | deprecated on Copilot since 2026-09-01 except annual plans; may not appear |
| `claude-haiku-4.5` | `claude-haiku-4-5-20251001` | `claude-haiku-4-5-20251001` | yes after alias | |

**Group C — Copilot-only in this fleet: exposed as-is under provider `copilot` (12).**
Nothing to pool with. `gemini-3.x-flash` are NOT the antigravity `gemini-3.x-flash-high`
entries (different serving config); they stay separate on purpose.

`gpt-5-mini`, `gpt-5.3-codex`, `gpt-5.4`, `gpt-5.4-mini`, `gpt-5.4-nano`, `gemini-3.5-flash`†,
`gemini-3.6-flash`†, `gemini-3.7-flash`†, `gemini-3.8-flash`, `kimi-k2.7-code`, `kimi-k3`,
`mai-code-1.1-flash`. Also possible, unverified: `claude-opus-4.8-fast`† and `-1m` variants†.

**Nothing is excluded.** The plugin's `excluded_model_prefixes` stays empty; no credential
`prefix` is used (the plugin drops it anyway at HEAD).

### Mechanism (recommended)

1. **Provider key `copilot`, raw ids, no prefix.** Home's model list is grouped by provider
   (`internal/respserver/get/models.go:36-135`), so a pooled id shows once under `claude` and
   once under `copilot`; that is the "correct label". The plugin also sets `owned_by` to the
   vendor (`Anthropic`, `Azure OpenAI`, …) and `display_name` to GitHub's name.
2. **Group B via Home's `oauth-model-alias` for channel `copilot`**, six entries
   `{name: claude-opus-5.5, alias: claude-opus-5-5}` etc., no `fork` (the Copilot spelling
   disappears; the native spelling is the one name). Home dispatches the per-credential upstream
   name, so the node still sends `claude-opus-5.5` to GitHub. **This needs a one-line Home
   change:** `OAuthModelAliasChannel` returns `""` for any provider outside its built-in list
   (`internal/cliproxy/auth/oauth_model_alias.go:324-325`), so aliases are silently ignored for
   plugin channels today. Change `default: return ""` to `return provider` (+ test), which is a
   clean upstream PR. Alternative without a Home change: add an alias map to the plugin fork
   (it already receives the host's alias config); more code, ours to keep.
3. **Pooling policy.** Home's selector round-robins all candidates of equal `priority`. Two
   options:
   - (i) equal priority: traffic splits evenly, Copilot credits drain at the same rate as the
     native quotas;
   - (ii) native first, Copilot as overflow: give the Copilot credentials a lower priority so
     they serve only when every native credential for that model is exhausted or cooling.
   **Recommendation: (ii) to start,** because every Copilot Claude id carries a 128K prompt
   cap and a 16K non-streaming output cap that the native accounts do not; with (i) roughly half
   of long Claude Code sessions would hit a 400 mid-conversation. (ii) still satisfies "served
   by whichever account has quota" and is a one-attribute change per credential to flip later.
   Priority is decided in Home (its own selector reads the credential's `priority`); on the
   node side, CPA 7.3.16's own commit `c404af96` fixed plugin credentials losing their
   `priority` on refresh (upstream issue #6089), so both halves now honour it.
4. **Home-side session affinity** already keeps a conversation on the credential it started on
   while that credential is healthy, which limits mid-conversation switching to the moments the
   pool actually fails over.

### Behaviour differences when a request lands on Copilot instead of the native account

| Dimension | Copilot | Native | Effect on a pooled session |
|---|---|---|---|
| Prompt/context | `max_prompt_tokens` 128K on every Claude and GPT id (Feb 2026 data; 1M only in VS Code/CLI) | Claude 200K or 1M; GPT 272K+ | A long session that fails over to Copilot gets a 400; the main reason for policy (ii) |
| Max output | Claude 32–64K; **16K when not streaming** | 64–128K | Non-streaming long answers truncate; all fleet clients stream |
| Wire path | Claude ids offer `/v1/messages`; plugin currently prefers `/responses` > `/chat/completions` > `/v1/messages` | native | **Plugin patch:** prefer `/v1/messages` for Claude ids so thinking, `cache_control` and Claude Code's beta features survive; on `/chat/completions` Copilot strips thinking |
| Prompt caching | works on `/v1/messages` with explicit `cache_control` (Claude Code sends it); auto on chat-completions; ~5 min TTL | 5 min / 1 h | comparable once the endpoint patch is in |
| Tool use | yes, both formats | yes | none |
| Vision | 1 image max, 3 MB | many | rare in fleet use |
| Rate limits | per-user 5-hour and weekly limits (429 with `user_global_rate_limited`) | vendor tiers | 429 → Home quota cooldown, correct |
| Exhausted credits | 402 | 429 | see Q4; plugin patch 402→429 |
| Model identity | GPT via Azure OpenAI; dateless Claude ids; Kimi may be a fine-tune | pinned | negligible for Claude/GPT |
| Cost | list price per token from the credit pool; Auto model −10% | subscription | none for the request, only which pool drains |

### Plugin patches to carry in our fork (all small, all evidence-backed)

1. `schema_version: 1` so Home's SDK v7.2.83 loads it (Q2).
2. Map upstream 402 → 429 in `upstreamStatusError` so exhausted credits hit Home's quota path (Q4).
3. Endpoint preference `/v1/messages` first for Claude-family ids (behaviour table).
4. `darwin/arm64` build target + release job (from the plugin's open PR #2).
Not needed: PR #1 (prefix preservation), because we do not prefix.

### Home changes to carry on `fleet` (and PR upstream where clean)

1. `oauth-model-alias` applied to plugin channels (one line + test) — clean upstream fix.
2. Optional: Copilot in the quota allowlist + a credits collector — real work, only if the
   quota page matters for Copilot.
3. `docs/management/api.md:897` correction (registry vs pinned artifacts).

### What I need from Chanse before task 5 (plain-text yes/no in this pane)

1. Mapping table + mechanism above: agree, or change any row.
2. Pooling policy: (ii) native-first/Copilot-overflow to start (recommended), or (i) equal split.
3. Where the `darwin/arm64` artifact gets built: GitHub Actions macOS runner in our fork
   (recommended), the MacBook, or a mini.
4. Fork the plugin under `chansearrington/` and carry the four patches: yes/no.
5. Which Copilot plan(s)/accounts will be logged in (Pro cannot see Opus/Fable/GPT-5.5/Sol;
   Pro+/Max or a Business org can), and acknowledgement of the third-party-client risk above.

## Close-out

(filled in at the end)
