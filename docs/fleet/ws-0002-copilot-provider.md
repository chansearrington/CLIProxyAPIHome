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
   Priority is decided in Home alone (its selector reads the credential's `priority` from the
   DB record). CPA 7.3.16's own commit `c404af96` fixed plugin credentials losing `priority`
   on refresh (upstream issue #6089), but that only affects standalone file-backed nodes; on
   the fleet it is inert. Task 6 must verify Home keeps a plugin credential's priority across
   its own refresh cycle.
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

### Chanse's decisions on the proposal (2026-09-25, interactive Q&A in the ws-0002 pane)

These supersede the pooling parts of the brief's Goal, "Chanse's decisions (2026-09-24)", the
Design question, and the mapping proposal above, which stay as the record of what was proposed.

1. **Separate, not pooled (changed his mind from 2026-09-24).** Verbatim intent: "I don't want
   my GitHub Copilot to be labeled and used in the pool. I want it to be separate ... If I choose
   Opus 5.5, I'm using my pooled Anthropic accounts, but if I choose Copilot Opus 5.5, I'm using
   only my GitHub credentials." Still: no model excluded, Claude included.
2. **Naming: `copilot/<GitHub's own id>`**, e.g. `copilot/claude-opus-5.5`, `copilot/gpt-5.5`,
   `copilot/gemini-3.8-flash`. Mechanism, zero Home code: the plugin stamps every Copilot
   credential with `prefix: copilot`, and Home's existing `force-model-prefix: true` setting drops
   the bare id (`internal/home/models.go:691-733`). `force-model-prefix` only affects credentials
   that have a prefix; verified read-only on the Ark 2026-09-25 that none of the six current
   credentials has one, so native model names are unchanged. The Group B alias rules and the
   `OAuthModelAliasChannel` Home fix are **no longer needed**. The 128K-cap failover risk is gone
   because a session only lands on Copilot when the client asks for a `copilot/` model.
3. **Mac build: GitHub Actions macOS runner in the fork.** Linux build on the Ark as planned.
4. **Fork `chansearrington/cliproxyapi-copilot-plugin` and carry five patches:** (1) declare
   plugin schema 1 for Home's SDK v7.2.83; (2) set `Prefix: "copilot"` on every `AuthData` the
   plugin returns (login, parse, refresh), which also covers what upstream PR #1 fixes; (3) map
   upstream 402 to 429; (4) prefer `/v1/messages` for Claude-family ids; (5) darwin/arm64 build
   and release job (from upstream PR #2). Offer (1)-(4) back upstream as PRs.
5. **Account: Chanse's Microsoft work seat**, GitHub account `carringt_microsoft`
   (enterprise-managed user), Copilot Enterprise assigned by the `my-copilot` enterprise,
   1,000,000 AI credits/month resetting on the 1st, internal-only models listed (e.g. "GPT-5.6 Sol
   Fast (internal only)", "GPT Daybreak Blue", "mai-experimental"). **The lead recommended a
   separate personal account on Pro+ and advised against this; Chanse chose the work seat.**
   Concerns raised and recorded: the seat is employer-owned and likely outside employee
   acceptable-use rules; fleet prompts (personal data) would be processed and logged under the
   enterprise; the plugin presents itself as VS Code, which on an enterprise seat reads as
   bypassing enterprise controls; internal-only models get exposed to the fleet; enterprise
   policy may block the path. Technical consequences: the plugin uses the enterprise API host
   from the token exchange (`endpoints.api`, handled at `internal/provider/token.go:141-157`); the
   stored GitHub token for this account lives in Home's DB on the Ark and is dispatched to nodes
   like every other credential. "Multiple Copilot accounts" stays a later option.

### Acceptance criteria changed by these decisions

- Criterion 6 now reads: all discovered Copilot models appear in the canary node's model list
  as `copilot/<id>`, none excluded, and **no bare Copilot id** appears.
- Criterion 7 (pooling proof) is replaced by a **separation proof**: a request for
  `copilot/<model>` is served only by the Copilot credential; a request for the native name
  (e.g. `claude-opus-5-5`) is never served by it, including while every native credential for
  that model is disabled; Home's usage view attributes each to the right credential.
- Home changes expected: only config (`force-model-prefix: true`, plugin store manifest,
  `load-in-home: true`); no Go change unless the canary shows otherwise.

## Task 5 record — plugin built (2026-09-28)

**Source.** Fork `chansearrington/cliproxyapi-copilot-plugin` (public; `main` mirrors upstream
`7f16b6011e93266f6d317166ef7cb6f0262f511f`). Branch `fleet`, release commit
**`1d51c025e23de056f8988ace84e2e1fb900a7a8e`**, tag **`v0.3.4`**. Five commits on top of upstream,
one per agreed patch, each with tests, plus `FORK.md`:

| Commit | Patch |
|---|---|
| `f888311` | declare registration schema 1 (Home's SDK v7.2.83 host accepts it; nodes accept ≤ 6) |
| `e48c953` | every credential gets `Prefix: "copilot"` (login, parse, refresh) |
| `01a885e` | upstream 402 (credits exhausted) reported to the host as 429 `quota_exhausted` |
| `1ffe0ac` | Claude-family models prefer `/v1/messages` when Copilot offers it |
| `1d51c02` | darwin/arm64 Make/package targets, macos-15 CI + release jobs, `scripts/smoke-load.py` (dlopen + `cliproxy_plugin_init` check on every package), version 0.3.4, metadata points at the fork |

**Verification.**
- On the Ark, `golang:1.26-bookworm` (go1.26.8 linux/amd64), source cloned from a git bundle
  of the exact commit into `/mnt/user/appdata/cpa-home-build/plugin-src`: `gofmt -l` empty,
  `go vet ./...` clean, `go test -count=1 ./...` all packages ok; the eight new or changed tests
  ran and passed by name.
- Fork CI run `36384438799` (commit `1d51c02`): tests on ubuntu and on macos-15 (go1.26.8
  darwin/arm64) ok; darwin library is `Mach-O 64-bit dynamically linked shared library arm64`,
  `codesign --verify`: "valid on disk / satisfies its Designated Requirement" (linker ad-hoc
  signature, so the Gatekeeper concern is closed), links only `libSystem`, and loads via dlopen.
- Release run `36384564452` (tag `v0.3.4`): validate, build-linux-amd64 (tests + build in
  `golang:1.26-bookworm`), build-darwin-arm64 (macos-15), release: all success.
- **Reproducible:** the Linux library GitHub built is byte-identical to the one the Ark built
  (inner `.so` sha256 below). The Ark-built library also loads on Debian bookworm glibc 2.36,
  the same base as Home's runtime image.

**Artifacts** (release `v0.3.4`,
`https://github.com/chansearrington/cliproxyapi-copilot-plugin/releases/download/v0.3.4/<file>`):

| File | Size (bytes) | sha256 of the zip (what Home pins) | sha256 of the library inside |
|---|---|---|---|
| `cliproxyapi-copilot_0.3.4_linux_amd64.zip` (for Home) | 13250064 | `60a67e8ff82a746da8e289c105a21ebd791b2547f4af3b9183f182379a9a6b93` | `c91f122ef423b7f4f142685c2345ab57e950d22e95a6f817d55bdd5be2e4bb81` (= Ark build) |
| `cliproxyapi-copilot_0.3.4_darwin_arm64.zip` (for the nodes) | 7142109 | `0e989ee4613d159bec86ef37661ed89d7b1a3c3b265ad648748345a265ca42ba` | `94d787e57019ff501d366f37fad2ed28bd60df79ec67cca7b2e9c9a5f771c979` |
| `checksums.txt` | 217 | `f43b5cb82a0547b421dafab38003804ab820a281d83db4b12ffc8d276b13b9fa` | — |

Each zip holds exactly one file at its root, `cliproxyapi-copilot.so` / `.dylib`, which is the
layout the node installer requires (CPA `internal/pluginstore/install.go:318-390` at `c404af96`).
Plugin id = `cliproxyapi-copilot`; config key = `plugins.configs.cliproxyapi-copilot`.

**Draft Home config for task 6 (NOT applied):**

```yaml
force-model-prefix: true
plugins:
  enabled: true            # already true in Home's DB
  configs:
    cliproxyapi-copilot:
      enabled: true
      load-in-home: true
      store:
        id: cliproxyapi-copilot
        version: 0.3.4
        schema-version: 2
        name: GitHub Copilot subscription provider
        description: Fleet fork of arthur-sommer-etc/cliproxyapi-copilot-plugin
        author: chansearrington
        repository: https://github.com/chansearrington/cliproxyapi-copilot-plugin
        install:
          type: direct
          artifacts:
            - {goos: linux,  goarch: amd64, url: https://github.com/chansearrington/cliproxyapi-copilot-plugin/releases/download/v0.3.4/cliproxyapi-copilot_0.3.4_linux_amd64.zip,  sha256: 60a67e8ff82a746da8e289c105a21ebd791b2547f4af3b9183f182379a9a6b93, size: 13250064}
            - {goos: darwin, goarch: arm64, url: https://github.com/chansearrington/cliproxyapi-copilot-plugin/releases/download/v0.3.4/cliproxyapi-copilot_0.3.4_darwin_arm64.zip, sha256: 0e989ee4613d159bec86ef37661ed89d7b1a3c3b265ad648748345a265ca42ba, size: 7142109}
```

**Found while preparing task 6 (read-only, `fleet` code):**
- Home's plugin config is global. Once installed, **every node downloads and loads the plugin**
  (plugin-sync has no per-node targeting); there is no config-only way to put it on one node.
- Dispatch is scoped per API key by channel group (`internal/cluster/api_keys.go:944-990`,
  `internal/home/runtime.go:891-911`): a Copilot credential in its own channel group can only be
  used by keys bound to that group. All five fleet keys are bound to group 1 today.
- The model list sent to nodes is **not** scoped per key (`internal/respserver/get/default.go:62-78`
  → `buildModelsJSON`): once logged in, `copilot/…` names appear on every node's `/v1/models`;
  requests for them from keys outside the Copilot group are refused.

## Task 6 plan — approved by Chanse 2026-09-28 (users + Copilot-only channel group)

Chanse's idea, adopted: model the agents as Home users so usage is attributable per agent by
name, and give only Chanse's user access to Copilot. Mechanism, verified in `fleet` code:
users are an owner/billing layer (`internal/cluster/users.go`); **access** is decided by the API
key's channel groups (`internal/cluster/api_keys.go:944-990`). A user-owned key is blocked only
when the user has `credits <= 0` without `credits_unlimited`, or an exceeded period limit
(`internal/cluster/user_period_limit.go:835-870`), so every user is created with
`credits_unlimited: true` and no limits. Usage reports group by user
(`/usage/aggregates?group_by=user`, `/usage/overview` top users).

Steps, each verified before the next; every one reversible:
0. Online backup of `home.db` into `data/backups/` (sqlite `.backup`).
1. Create users `chanse`, `moxy`, `chip`, `hyper`, `lara` (`credits_unlimited: true`).
2. Bind API keys 2-6 to their users (Hyper, Chip, Moxy, Lara, MacBook Pro → chanse); key 1
   (unnamed, no channels) left unbound. Prove one real request per agent key still succeeds.
3. `force-model-prefix: true` and the plugin store manifest from the task 5 record
   (`load-in-home: true`). Prove Home loaded it and every node reports `plugin-status` ok and
   stays healthy.
4. Chanse logs in `carringt_microsoft` through Home's device-code flow.
5. New channel group "Copilot — Chanse only" = the Copilot credential; bind it to the MacBook
   key in addition to group 1. The Copilot credential is not added to group 1.
6. Proof: `copilot/<model>` answered via the MacBook key and attributed to the Copilot
   credential; the same request with an agent key is refused; native names never route to
   Copilot; no bare Copilot id in any model list.

## Task 6 progress record (2026-09-29, times UTC)

**Step 0 — backup.** My first backup used sqlite's online `.backup`, which restarts every time
Home writes and never finished (agent-os `docs/runbooks/cpa-home-build-and-rollback.md` §8 already
says: use `VACUUM INTO`). The coordinator killed it and took the replacement
`data/backups/ws-0002-before-users-and-copilot-2026-09-29T16-36-25Z.db` (2,169,200,640 bytes);
verified here: `quick_check` ok, 6 credentials, 6 API keys, 5 live users. **This is the rollback
point for everything below.**

**Steps 1-2 — users.** The five users already existed (created 2026-09-09): Moxy 2, Chanse 3,
Chip 4, Hyper 5, Lara 6 (user 1 `moxy.kit@outlook.com` is soft-deleted). No key was bound. Chip
had `credits_unlimited: false` with 0 credits, which would have blocked Chip's key the moment it
was bound; set to `true` first (16:3xZ). Keys matched to machines by sha256 fingerprint prefix
of each mini's OpenClaw `cpa-gui` key (no key printed). Bound one at a time, each followed by a
real request from that machine (all HTTP 200 "ok"): key 5 Lara→Lara, 2 Hyper→Hyper, 4 Moxy→Moxy,
3 Chip→Chip, 6 MacBook Pro→Chanse; all keep `channels: [1]`; key 1 (unnamed, no channels) left
unbound. Zero `user_credits`/`user_period_limit` refusals afterwards;
`/usage/aggregates?group_by=user` now reports Chanse, Chip, Lara, Hyper, Moxy.

**Step 3 — plugin install, and a CPA node bug.** `PUT /config.yaml` at 16:41:37Z with exactly
three changes (`force-model-prefix: true`, the `plugins.configs.cliproxyapi-copilot` block from
the task 5 record, the `openai-compatibility` credential root omitted so Home leaves it alone).
All six API keys were carried verbatim: a config replace soft-deletes any key missing from the
upload (`internal/cluster/repository.go`, `replaceAPIKeysTxWithStats`), and bindings of kept keys
are preserved (verified after). Home loaded the plugin in-process at once (`/plugins`:
registered, `oauth_provider: copilot`).

Every node then looped on `failed to stage home config; retrying error=load home plugins:
plugin cliproxyapi-copilot installed but not loaded` (~2,000 lines/min into Home's log) while
still serving on its previous config. **Cause (CPA v7.3.16 `c404af96`, upstream bug):** on a
hot config update the node installs the new plugin file, then `MarkLoadResults` checks
`pluginHost.PluginRegistered` (`sdk/cliproxy/service_home.go:183-186`,
`internal/homeplugins/sync.go:677-712`) **before** the new config is applied, and plugins are
only loaded when config is applied, so a newly added plugin can never pass. The config worker
retries the same payload forever and never dequeues a newer one
(`service_home.go:664-682`), so rolling the config back (16:43:15Z) did not help. The startup
path is ordered correctly (`cmd/server/main.go:677-690`: `pluginHost.ApplyConfig` then
`MarkLoadResults`), but exits the process if a load fails.

Recovery: re-applied the plugin config (16:44:24Z) and restarted nodes in place with the
runbook command `launchctl kickstart -k gui/$UID/ai.openclaw.cpa-home-node`, the MacBook first
(16:44:25Z; about 10 s of 503s while Home released the old membership), then one at a time with
health + plugin-registered + real-request checks: Lara 16:45:22Z, Hyper 16:45:44Z, Moxy 16:46:11Z,
Chip 16:46:30Z. Each back in 14-20 s. Result at 16:47Z: all five nodes `plugin_report_state:
reported_ok`, plugin `loaded` 0.3.4, no retry lines after 16:46:30Z, zero failed requests in the
prior 10 minutes, native model list unchanged (55 ids; the only new one, `claude-sonnet-5-5`,
comes from the Anthropic accounts).

**Lesson for any future plugin add or version bump through Home:** expect this loop on every
node; the fix is a restart in place per node. Follow-up: report upstream to
router-for-me/CLIProxyAPI with the file/line evidence above.

**Step 5 prepared ahead of the login (16:47Z).** Channel group 2 "Copilot — Chanse only" created
(empty until the Copilot credential exists). Key 6 (MacBook Pro, user Chanse) bound to
`channels: [1, 2]`. Key 1 (the 2026-09-07 pilot canary key, stored only on the Ark, unused since
2026-09-08) was unscoped (`channels: []` = any credential), so it was scoped to `[1]`; agent keys
2-5 stay `[1]`. A new Copilot credential is reachable only through key 6. MacBook request after
the change: HTTP 200.

**Step 4 — waiting on Chanse.** Device code started 16:48:17Z via `GET /copilot-auth-url`
(`https://github.com/login/device?user_code=…`); not approved, expired 17:03:19Z ("GitHub device
code expired"). Next: start a fresh code when Chanse is present, then add the new credential to
channel group 2 and run the separation proof (step 6).

**Step 4 retry and a Home bug (2026-09-30).** Fresh code at 02:28:33Z; Chanse approved at
GitHub and the plugin's poll succeeded, but Home failed the save at 02:29:39Z: `cluster plugin
oauth: save auth failed provider=copilot error=auth index is required`
(`internal/cluster/management/plugin_oauth.go:229-236` → `internal/cluster/repository.go:84-89`,
which requires `ID == Index`, a UUID). The plugin returns its own identifier
(`copilot-<login>.json`) and no index, so **every plugin login fails on Home 1.0.73**; native
logins get a UUID from `EnsureOAuthPayloadUUID`, plugin logins get nothing. Not fixed upstream as
of `upstream/dev` `e16d6f1`. Nothing was stored.

**Fix, shipped on Chanse's in-pane approval ("Ship it now, I approve", 2026-09-30).** Commit
`9ba431f` on `fleet`: `cluster.EnsurePluginAuthIdentity` gives a plugin-created auth a
deterministic UUID from provider + plugin identifier (re-login updates the same record, a
different account or provider gets a different one), keeps the plugin identifier as `FileName`,
leaves existing UUIDs alone; called once before `UpsertAuth` in the plugin login path. 4 new tests.

- Ark, `golang:1.26-bookworm`: gofmt clean; vet = only the known `refresh.go:172`; **full suite
  33/33 test packages ok, rc 0**; compile ok. The suite **fails with a 1 GB `/tmp` tmpfs**
  ("No space left on device" while linking 4 cgo test binaries in parallel); a 3 GB tmpfs passes
  (Ark had 86 GB RAM available). FLEET.md updated.
- `origin/fleet` had moved to `7171da7` (ws-0001 deploy record, docs only); branch rebased onto it,
  code tree byte-identical to the tested one, `fleet` fast-forwarded to `9ba431f` with
  `--force-with-lease`.
- Four fleet-locks held (canonical `cpa-home-runtime-20260907-root` / `cpa-home-root`, heartbeat
  every 60 s, no failures) from before the backup to after validation; released 02:4xZ.
- `VACUUM INTO` backup `data/backups/home-pre-ws0002-20260930T023857Z.db`, 2,224,803,840 bytes,
  integrity ok, sha256 `2c661cfc398d2d16b6508d37edd8e8cdde67a6c946c907c8df8e8fe5a38817ce`.
- Runbook Block 1 verbatim (host → Tailscale address): `FULL_SHA=9ba431f6475da421e71a091d82825f5ec05e5982`.
  Panel mirror `/tmp/panel-mirror/static-full` copied in (64 files / 3,919,143 bytes) and removed
  after. Block 2 verbatim except `EXPECT_SHA` and the `1.0.73-claude-fleet` tag lines. Disk 99 GB free after.
- **Image `cpa-home:1.0.73-claude-fleet-9ba431f`, ID
  `sha256:f83c5493b72083db8d730c4a69a5c173501ba3007ba83fd4b701f8d894ce4dcf`, 185,503,088 bytes.**
  Binary markers new/running: `EnsurePluginAuthIdentity` 1/0, full SHA 3/0,
  `isClaudeRateLimitHeaderKey` 1/1, `1.0.73-claude-fleet` 3/3, control `CLIProxyAPIHome` present in both.
- Ship card **`BqIotjm7HX`** (type ship, risk yellow) raised as the Inbox record of the pane approval.
- Swap 02:41:19Z: rollback tag `cpa-home:1.0.73-claude-fleet-2b43b07` verified = running ID
  `sha256:60179681754f…` first; compose backed up as `docker-compose.yml.pre-ws0002-<ts>.bak`, one
  `image:` line changed, `docker compose up -d home`. Running `sha256:f83c5493b720…`, restarts 0,
  no panic; Home loaded the Copilot plugin; 5/5 nodes healthy with plugins `reported_ok`; one real
  request from every agent key and the MacBook: HTTP 200; 23 requests after the swap, 0 failed.
  **Rollback = compose back to `cpa-home:1.0.73-claude-fleet-2b43b07` after re-verifying its ID.**

**Step 4, third attempt:** code at 02:42:16Z on the fixed Home, not approved, expired 02:57:17Z.
Waiting on Chanse.

## Close-out

(filled in at the end)
