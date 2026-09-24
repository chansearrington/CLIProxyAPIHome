# Task 3 — CPA plugin SDK at the fleet's exact commit, and PR #5661 (native Copilot) compared

Read-only research for ws-0002 (GitHub Copilot as a provider for the agent-os fleet).
All file:line references are at CLIProxyAPI commit **`ac02da6c05e18f465aa7e3ed5b0a65a2f060917d`** (tag `v7.2.159`) unless
stated otherwise. Scratch clone: `/tmp/ws-0002-research/cpa` (checked out detached at that commit). Nothing was
installed, built, or modified in any real repository or on any server.

---

## 0. Commit / tag verification

| Fact | Value |
|---|---|
| `git rev-parse v7.2.159^{commit}` | `ac02da6c05e18f465aa7e3ed5b0a65a2f060917d` — starts with `ac02da6c` ✅ matches the fleet's `Commit: ac02da6c` |
| Commit date | Sun Sep 13 01:59:34 2026 +0800 |
| Latest tag in repo | `v7.3.16` → `c404af96` (Thu Sep 24 08:09:21 2026 +0800, "fix(auth): preserve file priority across plugin auth refreshes") |
| Commits `ac02da6c..origin/dev` | 276 |

The fleet is 276 commits / 17 minor releases behind `dev`. Everything below is what the **running** nodes do, not what
`main` does.

---

## 1. `sdk/pluginabi` and `sdk/pluginapi` — what the plugin contract looks like

### 1a. ABI / schema version constants and what the host does on mismatch

Constants (all in `sdk/pluginabi/types.go`):

| Constant | Value | Line |
|---|---|---|
| `ABIVersion uint32` | **1** | `sdk/pluginabi/types.go:7` |
| `SchemaVersion uint32` | **6** | `sdk/pluginabi/types.go:20` |
| `SchemaVersionStreamChunkOmitRequestBody` | 3 | `:23` |
| `SchemaVersionWebSocketResponseObserver` | 4 | `:26` |
| `SchemaVersionStreamChunkOmitHistory` | 5 | `:29` |
| `SchemaVersionRawManagementResponse` | 6 | `:32` |

Host-side behaviour:

- **ABI (native C table)** — the host stamps `pluginHostABIVersion = pluginabi.ABIVersion` (`internal/pluginhost/abi.go:9`)
  into the host API struct it hands to the plugin (`internal/pluginhost/loader_unix.go:142`), calls the plugin's
  `cliproxy_plugin_init` (`loader_unix.go:149`), then **hard-rejects any plugin whose returned table does not equal exactly
  1**: `if uint32(client.api.abi_version) != pluginHostABIVersion { … "plugin ABI version %d is not supported" }`
  (`loader_unix.go:154-157`; Windows equivalent `loader_windows.go:107`). Exact-equality, not ≥.
- **Schema (JSON RPC contract)** — the host sends its own `SchemaVersion: pluginabi.SchemaVersion` (=6) in the
  `plugin.register` request (`internal/pluginhost/rpc_client.go:68-71`). The plugin replies with the schema version it
  speaks. The host **rejects only if the plugin's version is newer than the host's**:
  `if resp.SchemaVersion > pluginabi.SchemaVersion { return … "plugin schema version %d is not supported" }`
  (`rpc_client.go:75-77`). A plugin declaring 0 is treated as 1 (`rpc_client.go:80-83`). So a plugin built against
  schema ≤6 loads on a 7.2.159 host; one built against schema 7+ (if a newer SDK ever bumps it) will not load on the
  fleet's nodes until they are upgraded.

Practical consequence for ws-0002: **whatever plugin binary we ship must declare `abi_version == 1` and
`schema_version <= 6`.** Check the community plugin's `go.mod` SDK pin and its registration payload (Task 1's job) against
these numbers.

### 1b. Capability interfaces and the auth-storage contract

`Capabilities` struct (`sdk/pluginapi/types.go:72-127`) lists every optional integration point. The ones a provider
plugin needs:

```go
// sdk/pluginapi/types.go:269-276
type AuthProvider interface {
    Identifier() string
    ParseAuth(context.Context, AuthParseRequest) (AuthParseResponse, error)
    StartLogin(context.Context, AuthLoginStartRequest) (AuthLoginStartResponse, error)
    PollLogin(context.Context, AuthLoginPollRequest) (AuthLoginPollResponse, error)
    RefreshAuth(context.Context, AuthRefreshRequest) (AuthRefreshResponse, error)
}

// sdk/pluginapi/types.go:389-393
type ModelProvider interface {
    StaticModels(context.Context, StaticModelRequest) (ModelResponse, error)
    ModelsForAuth(context.Context, AuthModelRequest) (ModelResponse, error)
}

// sdk/pluginapi/types.go:370-373  (older/simpler alternative to ModelProvider)
type ModelRegistrar interface {
    RegisterModels(context.Context, ModelRegistrationRequest) (ModelRegistrationResponse, error)
}

// sdk/pluginapi/types.go:589-596
type ProviderExecutor interface {
    Identifier() string
    Execute(context.Context, ExecutorRequest) (ExecutorResponse, error)
    ExecuteStream(context.Context, ExecutorRequest) (ExecutorStreamResponse, error)
    CountTokens(context.Context, ExecutorRequest) (ExecutorResponse, error)
    HttpRequest(context.Context, ExecutorHTTPRequest) (ExecutorHTTPResponse, error)
}

// sdk/pluginapi/types.go:1470-1480
type QuotaProvider interface {
    Identifier() string
    DescribeQuota(context.Context, QuotaDescribeRequest) (QuotaDescribeResponse, error)
    FetchQuota(context.Context, QuotaFetchRequest) (QuotaFetchResponse, error)
    ResetQuota(context.Context, QuotaResetRequest) (QuotaResetResponse, error)
}
```

Other capabilities present at this commit (`types.go:72-127`): `FrontendAuthProvider`, `Scheduler`, `ModelRouter`,
`RequestTranslator/Normalizer`, `ResponseTranslator/Normalizer` (before/after), `RequestInterceptor`,
`RequestLifecyclePlugin`, `ResponseInterceptor`, `StreamChunkInterceptor`, `WebSocketResponseObserver`,
`ThinkingApplier`, `UsagePlugin`, `CommandLinePlugin`, `ManagementAPI`. Executors must also declare
`ExecutorInputFormats` / `ExecutorOutputFormats` (`types.go:93-96`) and optionally `ExecutorModelScope`
(`both|static|oauth`, `types.go:129-139`).

**Auth-storage contract offered to a plugin.** There is no "storage interface" the plugin implements; instead the plugin
hands the host an `AuthData` record and the host owns persistence:

```go
// sdk/pluginapi/types.go:219-243
type AuthData struct {
    Provider string; ID string; FileName string; Label string; Prefix string; ProxyURL string
    Disabled bool
    StorageJSON []byte            // provider-owned persisted auth data  (types.go:235-236)
    Metadata map[string]any       // mutable host-managed auth metadata  (types.go:237-238)
    Attributes map[string]string  // immutable routing/provider attrs    (types.go:239-240)
    NextRefreshAfter time.Time
}
```

- Host converts `AuthData` → core `coreauth.Auth` in `pluginAuthDataToCoreAuth` (`internal/pluginhost/auth_provider.go:543-595`).
  The `Storage` field becomes a `*pluginTokenStorage{provider, rawJSON: StorageJSON, meta: Metadata}` (`:587`), and
  `metadata["type"] = provider` is forced (`:552-554`).
- **Where it is persisted (standalone node):** `pluginTokenStorage.SaveTokenToFile(path)` (`auth_provider.go:443-458`)
  merges `StorageJSON` + `Metadata` + `"type": provider` (`mergedStorageJSON`, `:483-509`) and atomically writes it as
  a JSON file (`atomicWriteFile`, `:511-541`). The path is a file **in `auth-dir`** — the file name comes from
  `AuthData.FileName`, and the auth ID defaults to the path relative to auth-dir (`authIDForPath`, `:90-106`). The
  file-backed store is the default (`cmd/server/main.go:610` registers `NewFileTokenStore()`); Postgres/object/git
  stores exist too (`internal/store/*.go`) and all still call `auth.Storage.SaveTokenToFile` (`postgresstore.go:252`,
  `objectstore.go:201`, `gitstore.go:456`).
- The host also exposes callbacks the plugin can call: `host.auth.list`, `host.auth.get`, `host.auth.get_runtime`,
  `host.auth.save` (`sdk/pluginabi/types.go:106-109`). `host.auth.save` writes `filepath.Join(authDir, Base(name))` with
  mode 0600 and upserts the record (`internal/pluginhost/auth_callbacks.go:284-306`).
- Every plugin call that needs the token receives the storage back: `ExecutorRequest.StorageJSON` (`types.go:925-926`),
  `AuthRefreshRequest.StorageJSON` (`:350-351`), `AuthModelRequest.StorageJSON` (`:410-411`), `QuotaFetchRequest.StorageJSON`
  (`:1506-1507`). On the host side that is `storageJSONFromAuth` (`internal/pluginhost/adapters_executors.go:1047-1062`):
  it prefers `auth.Storage.RawJSON()`; **if the Auth has no Storage object it falls back to `json.Marshal(auth.Metadata)`**
  (`:1054-1061`).

**How a Home-managed node persists (or rather doesn't):**

- When `home.enabled` is true the node **does not load a local auth store at all**: `if s.coreManager != nil && !homeEnabled { s.coreManager.Load(ctx) … }`
  (`sdk/cliproxy/service_lifecycle.go:77-94`), does not ensure auth-dir (`:67-71`), does not load token/api-key providers
  (`:96-110`), and cooldown persistence is disabled (`service_auth.go:434`).
- Credentials arrive per request from Home over RESP: `pickHomeDispatchSelection` → `RPopAuthWithSessionHierarchy`
  (`sdk/cliproxy/auth/conductor_home.go:1013-1018`) → `internal/home/client.go:1364-1424` (`rPopAuth`). The response
  `homeAuthDispatchResponse` (`conductor_home.go:346-356`) contains a full `Auth` JSON. `Auth.Storage` is `json:"-"`
  (`sdk/cliproxy/auth/types.go:64`), `Auth.Metadata` is `json:"metadata"` (`:80`), `Auth.Attributes` is `json:"attributes"`
  (`:78`). So a dispatched plugin auth has **no Storage object**, and `storageJSONFromAuth` gives the plugin
  **`json.Marshal(auth.Metadata)`** as its `StorageJSON`. ⇒ **The plugin's token must live in `Metadata` (what Home stores
  as the auth JSON), not only in a separate `StorageJSON` blob**, or the executor will get an empty/incorrect storage on a
  Home node. (Since `mergedStorageJSON` writes StorageJSON+Metadata into one flat JSON file, and Home imports that same
  file shape, in practice the merged file IS what Home holds — but the round trip goes through `Metadata`.)
- Refresh on a Home node never runs the plugin's `RefreshAuth`: `helps.RefreshAuthViaHome` short-circuits when
  `cfg.Home.Enabled` and asks Home via `GetRefreshAuth(authIndex, accessTokenSHA256)`
  (`internal/runtime/executor/helps/home_refresh.go:93-127`; `internal/home/client.go:1434-1464`). Note:
  `pluginRefreshCompatExecutor` (`internal/pluginhost/plugin_refresh_compat_executor.go:15-40`) delegates `Refresh` to
  `Host.RefreshAuth` — that is the *non-Home* path; on a Home node, **Home** must know how to refresh the credential
  (Task 2 territory: does Home's plugin-auth support at upstream/dev call the plugin's refresh, and does the Home build the
  Ark runs have it?).
- Home ↔ node plugin sync: the node asks Home for the plugin list via `GetPluginSync` (`internal/home/client.go:1594`),
  installs/deletes artefacts (`sdk/cliproxy/home_plugins.go:47-91`, `internal/homeplugins/sync.go:199,370`), and reports
  status via `RPushPluginStatus` (`client.go:1561`). The sync key is a hash of `plugins.enabled`, `plugins.dir`,
  `plugins.auth-revision` and every `plugins.configs.<id>` subtree (`home_plugins.go:254-281`). If Home returns
  `ErrPluginSyncUnsupported` the node falls back to its own store sync (`home_plugins.go:80-84`).

### 1c. How plugins are loaded

- **In-process native dynamic library via cgo `dlopen`**, not Go's `plugin` package and not out-of-process gRPC.
  `loader_unix.go:1` is `//go:build cgo && (linux || darwin || freebsd)`; it `dlopen(path, RTLD_NOW|RTLD_LOCAL)`
  (`:43-45`) and looks up a single C symbol `cliproxy_plugin_init` (`:120-126`). The plugin exports a C function table
  `{abi_version, call, free_buffer, shutdown}` (`:31-36`) and every method is a JSON request/response over
  `call(method, request, len, *response)` (`:165-200`). The method names are the `Method*` constants in
  `sdk/pluginabi/types.go:35-110` (`plugin.register`, `auth.parse`, `executor.execute_stream`, `host.http.do`, …).
- Because the plugin is a C-ABI shared object, **it does not need the same Go toolchain or module versions as the host**
  (unlike Go's `plugin` package). It does need the host to be a **cgo build**: with `!cgo` the loader is a stub that
  returns "standard dynamic library plugin loading requires cgo on this platform" (`internal/pluginhost/loader_unsupported.go:1-15`).
  Upstream's Dockerfile builds with `CGO_ENABLED=1` (`Dockerfile:17`), as does our own fleet Dockerfile (FLEET.md). The
  `*_no-plugin.tar.gz` release archives are the `CGO_ENABLED=0` ones (`.github/workflows/release.yaml:425-431`). ⇒ Confirm
  the minis' `cli-proxy-api` binary is a cgo build (the `docker` image is; a `no-plugin` tarball is not).
- **Where CPA looks:** `candidateDirs(root, GOOS, GOARCH)` = `[<dir>/<goos>/<goarch>, <dir>]`
  (`internal/pluginhost/platform.go:308-313`); files must end in `.so` (linux) / `.dylib` / `.dll`
  (`platform.go:103-112`, `:144`); file name `<id>.so` or `<id>-v<version>.so` (`platform.go:62-96`). Default dir is
  `plugins` relative to CWD (`internal/config/plugin_path.go:10`, tilde-expanded `:14-33`).
- **Config keys** (`internal/config/config_types.go:24-47`):

  ```go
  type PluginsConfig struct {
      Enabled      bool                            `yaml:"enabled"`        // default false (Go zero value; config.example.yaml:90 also "false")
      Dir          string                          `yaml:"dir"`            // default "plugins"
      StoreSources []string                        `yaml:"store-sources"`
      StoreAuth    []sdkpluginstore.AuthConfig     `yaml:"store-auth"`
      AuthRevision int64                           `yaml:"auth-revision"`  // "changes when Home-managed plugin credentials change"
      Configs      map[string]PluginInstanceConfig `yaml:"configs"`
  }
  type PluginInstanceConfig struct {
      Enabled  *bool `yaml:"enabled"`   // nil → false (config_types.go:41,56-57)
      Priority int   `yaml:"priority"`
      Raw      yaml.Node                // full plugin YAML subtree preserved
  }
  ```

  Top-level key `plugins:` (`internal/config/config.go:31-32`). So a plugin needs **both** `plugins.enabled: true` **and**
  `plugins.configs.<id>.enabled: true` (`config.example.yaml:88`). On a Home-managed node this config comes from Home
  (`forceHomeRuntimeConfig`, `service_lifecycle.go:55`), so Home's config for the node has to carry the `plugins:` block.

### 1d. Model registration and — critically — how a credential is chosen for a model name

**Model fields a plugin can register** — `pluginapi.ModelInfo` (`sdk/pluginapi/types.go:141-181`): `ID`, `Object`,
`Created`, `OwnedBy`, `Type`, `DisplayName`, `Name`, `Version`, `Description`, `InputTokenLimit`, `OutputTokenLimit`,
`SupportedGenerationMethods`, `ContextLength`, `MaxCompletionTokens`, `SupportedParameters`,
`SupportedInputModalities`, `SupportedOutputModalities`, `Thinking *ThinkingSupport`, `UserDefined`. There is **no
alias field** on the plugin's ModelInfo.

**How they land in the registry:**

1. Static models: `Host.RegisterModels` calls `StaticModels` (or `RegisterModels` for a `ModelRegistrar`)
   (`internal/pluginhost/adapters.go:228-258`); the **provider key is whatever the plugin returns in
   `ModelResponse.Provider`**, lower-cased (`adapters.go:265`), and is remembered as `modelProviders[pluginID]`
   (`adapters.go:290`, committed at `adapters_interceptors.go:392`). If the plugin has no executor the models are
   registered under client id `plugin:<id>:<provider>` (`adapters.go:291-298`).
2. Plugins **with** an executor: `Host.RegisterExecutors` (`adapters_executors.go:34-129`) registers the executor
   under that provider key (`:116`, `:173` → `manager.RegisterExecutor`) and registers the models under client id
   `plugin:<id>:<provider>:executor` (`:131-133`, `:188`). Guard rails: a plugin **cannot** claim a provider key that a
   native executor already owns (`providerHasNativeExecutor`, `:61-64`, `:111-113`, `:272-278`) and **cannot claim a model
   ID that a native provider already serves** (`modelHasNativeExecutor`, `:79-81`, `:280-290`).
3. Per-credential models: when an auth is (re)loaded, `registerModelsForAuthWithCache` first tries
   `tryRegisterPluginModelsForAuth` (`sdk/cliproxy/service_models.go:57`) which calls the plugin's `ModelsForAuth`
   (`service_executors.go:490-562`), applies exclusions, **applies `oauth-model-alias` for the provider channel**
   (`:555`), applies the per-auth `Prefix` (`:557` → `applyModelPrefixes`, `service_models.go:578-623`), and registers
   with **client id = the auth ID** and provider = the plugin's provider (`registerResolvedModelsForAuth`,
   `service_executors.go:420-447` → `GlobalModelRegistry().RegisterClient(a.ID, providerKey, models)`).

The registry itself (`internal/registry/model_registry.go`) is **keyed by model ID**: `models map[string]*ModelRegistration`
(`:149`), and each registration tracks `Providers map[string]int` (`:134`) and `InfoByProvider` (`:126`).
`RegisterClient` (`:304-…`) adds one count per (model, provider). `GetModelProviders(modelID)` (`:1428-…`) returns every
provider that has ≥1 client for that model, sorted by count desc then name.

**Credential selection for an incoming model name — the important part:**

- The request handler resolves model → providers with `util.GetProviderName(modelName)` which is literally
  `registry.GetGlobalRegistry().GetModelProviders(modelName)` (`internal/util/provider.go:46-74`). So a model id maps to
  **all** providers that registered it.
- The manager then goes to `pickNextMixed(ctx, providers, model, …)` (`sdk/cliproxy/auth/conductor_selection.go:2013-2098`).
  Local (non-Home) path: it keeps every provider that has an executor (`:2027-2040`) and asks the scheduler
  `pickMixed(eligibleProviders, model, …)` (`:2072`).
- `authScheduler.pickMixedWithStrategy` (`sdk/cliproxy/auth/scheduler.go:350-…`): for **more than one provider** it takes
  each provider's per-model shard (`:402-408`), finds the **best (highest) auth priority across all providers**
  (`:412-419`), then:
  - fill-first: first provider in list order that has a ready auth at that priority (`:425-437`);
  - round-robin / weighted: **flattens the ready auths from every provider's shard at that priority into one list and
    round-robins across them** (`:439-470`: `entries = append(entries, bucket.all.flat...)` for every shard, then
    `pickSmoothWeightedScheduled(entries, …)`), with the cursor keyed by `providers-joined:model` (`:439`).

  So: **yes — a model id registered by a plugin and the same id registered by a native provider end up in ONE pool and
  are round-robined between them**, separated only by auth `priority` (the per-credential priority attribute). The pool
  key is effectively (model id) across all providers; provider is only a grouping inside the shard list.

- **Home path (what the fleet actually runs):** `pickNextMixed` short-circuits to `pickNextViaHome` when
  `HomeEnabled()` (`conductor_selection.go:2015-2017`), which sends only `requestedModel` to Home
  (`conductor_home.go:1018`) and Home picks the credential. The node then looks up the executor by
  `executorKeyFromAuth(&auth)` = lower-cased `auth.Provider` (or `provider_key`/`compat_name` attrs)
  (`conductor_home.go:1153-1175`; `conductor_execution.go:1750-1772`), falling back to `openai-compatibility` if the auth
  carries a `base_url` attribute (`:1169-1171`), else `executor_not_found` (`:1172-1175`). ⇒ On the fleet **the pooling
  decision is made by Home, not by this code.** The node only needs: (1) the plugin loaded so an executor with
  `Identifier()=="<provider>"` exists, and (2) Home to dispatch an auth whose `provider` equals that key. Whether Home
  pools `copilot` credentials for `claude-sonnet-4-5` together with Anthropic OAuth ones is a **Home** question (Task 2
  / ws-0001's fleet Home code), but note Home's dispatch response carries `model`, `provider`, `force_mapping`,
  `original_alias` (`conductor_home.go:346-356`) so Home already has a per-provider upstream-model rewrite channel
  (`auth.Attributes["home_upstream_model"]`, `:1128-1140`).

### 1e. Model aliases at this commit

- **Config key:** `oauth-model-alias: { <channel>: [ {name, alias, fork, display-name, force-mapping} ] }`
  (`internal/config/config.go:158-164`; struct `internal/config/config_types.go:284-293`; example
  `config.example.yaml:735-743`). Comment says supported channels are `vertex, aistudio, antigravity, claude, codex, kimi, xai`
  (`config.go:160`), **but the code's channel resolver returns the provider key unchanged for any unknown provider**
  (`OAuthModelAliasChannel`, `sdk/cliproxy/auth/oauth_model_alias.go:476-496`, `default: return provider`), and only
  `gemini` and API-key auths are excluded (`:479-484`). Plugin OAuth providers therefore DO get `oauth-model-alias`
  applied (also confirmed by the host passing `OAuthModelAlias` to plugins in `HostConfigSummary`, `sdk/pluginapi/types.go:205-206`,
  `internal/pluginhost/auth_provider.go:29`; and `OAuthRequestScopedErrors` comment explicitly lists "OAuth plugin provider keys",
  `config.go:167`). PR #5661's edit to that comment (adds `github-copilot`) is cosmetic.
- **Semantics:** an alias is **per channel (provider), not per (provider, model) pair in the sense of steering** — it maps
  `name` (upstream model) → `alias` (client-facing id) for auths **of that channel only**. Listing: `applyOAuthModelAliasEntries`
  (`sdk/cliproxy/service_models.go:962-1040+`) replaces the model id with the alias (or keeps both when `fork: true`,
  `:1011-1023`). Routing: the manager compiles a reverse table `channel → alias(lower) → upstream` (`oauth_model_alias.go:26-78`)
  and rewrites the upstream model at execution time (`OAuthModelAliasResult`, `:31-36`; `force-mapping` additionally
  rewrites the model name in responses).
- **Does an aliased name pool with the un-aliased name?** The alias is registered as a *model id* in the same registry
  (`registerResolvedModelsForAuth`, `service_executors.go:446`), so **pooling is by the client-facing id**: if channel A
  aliases `gpt-5` → `copilot/gpt-5` and channel B does not, then `copilot/gpt-5` pools only A's auths and `gpt-5` pools only
  B's (plus A's if `fork: true`). Conversely if two channels both expose (or alias to) the same id, they pool together —
  exactly what `config.example.yaml:709-711` warns about ("overlapping client-visible names can become ambiguous across
  providers. For strict backend pinning, use …"). The recommended CPA-side isolation tool is the per-auth `prefix`
  (`applyModelPrefixes`, `service_models.go:578-623`): with `force-model-prefix: true` the un-prefixed id is **not**
  registered (`:610-612`), only `<prefix>/<id>` (`:613-620`), and the registry stores `MetadataModelID = baseID` so the
  upstream name is recovered (`:615-617`). `AuthData.Prefix` is settable by the plugin (`sdk/pluginapi/types.go:229-230`) and
  `HostConfigSummary.ForceModelPrefix` is passed to it (`:203-204`).
- Again: on the fleet, **Home** does the model→credential mapping and already carries an `original_alias`/`force_mapping`
  channel in the dispatch response, so the CPA-side alias machinery matters mainly for `/v1/models` listing consistency
  and for the `home_upstream_model` rewrite the node applies (`conductor_home.go:1128-1133`).

### 1f. Usage accounting for a plugin executor and the push to Home

- The plugin executor adapter wraps every call in `helps.NewExecutorUsageReporter(ctx, a, modelName, auth)`
  (`internal/pluginhost/adapters_executors.go:663-670` non-stream, `:721-728` stream). The reporter's **provider is
  `executor.Identifier()`** (`internal/runtime/executor/helps/usage_helpers.go:59-63`) — i.e. the plugin's provider key —
  and `executorType` is the Go type name `executorAdapter` (`ExecutorTypeName`, `:178-187`). Tokens are parsed from the
  plugin's response payload per declared output format (`ParsePluginExecutorResponseUsage`, `adapters_executors.go:704`)
  and published (`:705-706`); failures via `PublishFailure` (`:677-684`).
- Record fields (`pluginapi.UsageRecord`/core record): `Provider`, `ExecutorType`, `Model`, `Alias`, `APIKey`, `SessionID`,
  `AuthID`, `AuthIndex`, `AuthType`, `Source`, `RequestedAt`, `Latency`, `TTFT`, `Failed`, `Detail{Input,Output,Reasoning,
  Cached,CacheRead,CacheCreation,Total}` (`sdk/pluginapi/types.go:1395-1468`; usage_helpers.go:96-104, 448).
- Push to Home: usage records are turned into a `queuedUsageDetail` JSON (`internal/redisqueue/plugin.go:106-146`) —
  `provider` defaults to `"unknown"` only if empty (`:42-45`), **no allowlist** — and the Home drain goroutine pops them
  and `LPushUsage`es to Home (`sdk/cliproxy/service_home.go:349-367`; `internal/home/client.go:1466`). Home-side ingestion
  is out of scope here (Task 2).
- **No provider-name allowlist that would drop `copilot`** was found in `sdk/cliproxy/usage`, `internal/redisqueue`, or
  `sdk/cliproxy/service_home.go`. The one provider-sensitive switch is **token-accounting semantics**
  (`sdk/cliproxy/usage/accounting.go:340-364`): it string-matches `provider+" "+executorType` for `claude/anthropic`
  (independent), `gemini/aistudio/antigravity/vertex` (separate-reasoning), `openai/codex/xai/grok/kimi/qwen/deepseek/openrouter`
  (subset). A provider key like `github-copilot` with executor type `executoradapter` matches **none** → `Unknown`
  semantics → the breakdown may be marked unclassified/inconsistent (`:270-278`, `:366-380`) even though raw
  input/output counts are still recorded. Cosmetic in Home's token breakdown, not a drop. (If the plugin's provider key
  contains "openai" or "copilot-openai" it would be classified as subset; worth a look at what key the community plugin
  uses.)

### 1g. Quota / disable path and what a plugin must return

- The manager's `MarkResult` (`sdk/cliproxy/auth/conductor_cooldown.go:741-…`) maps the failure's HTTP status to a
  cooldown on the (auth, model) state: `401/402/403` → 30 min (`:852-858`), `404` → 12 h (`:859-865`), **`429` → quota
  exceeded**: uses `result.RetryAfter` if present (floored at `minQuotaCooldownFloor`) else exponential
  `quotaCooldownAfterFailure` (`:866-889`), sets `state.Quota = {Exceeded:true, Reason:"quota", NextRecoverAt, BackoffLevel}`
  (`:884-889`), and if `result.CredentialScope` propagates the cooldown to every other model on that credential
  (`:890-905`). Aggregation to the auth level is `updateAggregatedAvailability` (`:1290-1356`). The status code is
  read via `statusCodeFromResult`/`statusCodeFromError`, i.e. **any error implementing `StatusCode() int`**
  (`:1422-1434`). The registry mirrors this with `SetModelQuotaExceeded` (`internal/registry/model_registry.go:770`).
- **What a plugin executor must return:** an RPC error envelope `{ok:false, error:{code, message, retryable, http_status}}`
  (`sdk/pluginabi/types.go:113-124`). The host decodes it into `rpcError{Code, message, statusCode: envelope.Error.HTTPStatus}`
  (`internal/pluginhost/rpc_client.go:358-373`), and `rpcError` implements `StatusCode()` (`rpc_client.go:55-57`). So
  **returning `http_status: 429` from `executor.execute`/`executor.execute_stream` is exactly what triggers CPA's quota
  cooldown.** `http_status: 402` (which is what GitHub actually sends when the premium-request allowance is exhausted —
  see PR #5661's `CopilotQuotaError`, §3) would instead be a 30-minute 401/402/403 cooldown, not the quota path; the
  plugin must translate 402 → 429 itself if it wants the quota semantics. Nothing in the plugin ABI lets a plugin set
  `CredentialScope` or `RetryAfter` directly (there is no field for it in `pluginabi.Error`, `types.go:119-124`); those
  are derived host-side from the error (`Result` struct, `sdk/cliproxy/auth/conductor.go:46-69`).
- **On a Home node**, `MarkResult` still runs locally (it is called from the execution loop, `conductor_execution.go:526,621,…`)
  but the node has no persistent auth store, and the authoritative "exhausted" state lives in Home (usage/error pushes,
  `error_events.go:58` → `redisqueue.EnqueueError`). The node also surfaces the plugin's `QuotaProvider` to the management
  UI (`internal/pluginhost/quota_provider.go`) — that is display only, not scheduling.

---

## 2. PR #5661 — native GitHub Copilot provider (NaveDanan)

URL: https://github.com/router-for-me/CLIProxyAPI/pull/5661  ·  head `NaveDanan/CLIProxyAPI:feat/github-copilot-provider`
(`68095e67`) · base `dev` · **+1423 / −16 in 29 files** · 4 commits.

**State (as of 2026-09-24):** `OPEN`, not draft, **`mergeable: CONFLICTING` / `mergeStateStatus: DIRTY`**,
`reviewDecision: REVIEW_REQUIRED`, no labels. Created 2026-09-09T07:36Z (auto-retargeted from `main` to `dev`), last
updated 2026-09-09T22:15Z — **no activity for 15 days**. Reviews: only the `chatgpt-codex-connector` bot ("Didn't find any
major issues") and the author's own two comments; **no maintainer review or comment**. Merge-base with `dev` is `7fac6b15`
(2026-09-09); `dev` has moved **329 commits** since. `git merge-tree` (read-only) against current `origin/dev` conflicts in
9 files: `cmd/server/main.go`, `config.example.yaml`, `internal/cmd/auth_manager.go`, `internal/config/config.go`,
`internal/runtime/executor/openai_compat_executor.go`, `internal/tui/oauth_tab.go`, `sdk/cliproxy/service_auth.go`,
`sdk/cliproxy/service_executors.go`, `sdk/cliproxy/service_models.go`. Against **`ac02da6c` (v7.2.159) it merges cleanly**
(merge-base is an ancestor of ac02da6c, 53 commits back).

**Design summary (from the diff):**

- **Auth = GitHub device flow.** `internal/auth/copilot/copilot.go`: OAuth app client id `Iv1.b507a08c87ecfe98` (the VS Code
  Copilot Chat client id), `POST github.com/login/device/code` with scope `read:user`, polls
  `login/oauth/access_token` with `grant_type=urn:ietf:params:oauth:grant-type:device_code`, handles `authorization_pending /
  slow_down / access_denied / expired_token`. It then calls `GET api.github.com/user` (with `X-GitHub-Api-Version: 2022-11-28`
  — the fix in commit 91add5ce) for a stable `github_id`/`login`, and exchanges the GitHub token for a short-lived Copilot
  token at `GET api.github.com/copilot_internal/v2/token` (cached in-process per (api, proxy, token) with singleflight;
  refreshed 1 min before expiry; **never written to the credential file**). Headers impersonate VS Code:
  `User-Agent: GitHubCopilotChat/0.40.0`, `Editor-Version: vscode/1.104.0`, `Copilot-Integration-Id: vscode-chat`,
  `X-GitHub-Api-Version: 2025-04-01`. The stored file is `github-copilot-<github_id>.json` with
  `{type:"github-copilot", auth_kind:"oauth", access_token, github_login, github_id, email, expired}` (`sdk/auth/copilot.go`).
  Login entry points: CLI flag `--github-copilot-login` (`internal/cmd/copilot_login.go`, `cmd/server/main.go`), the TUI
  OAuth tab, and a management-API card (`internal/api/handlers/management/copilot.go`) that shows the device code.
- **Provider key `github-copilot`; model ids NOT prefixed.** Models are discovered per account from
  `GET <copilot-api>/models` (`internal/auth/copilot/models.go`), keeping only `capabilities.type == "chat"` models with a
  usable endpoint, and registered with **`ID = m.ID` verbatim** (e.g. `gpt-5`, `claude-sonnet-4.5`, `gemini-2.5-pro` as
  GitHub names them), `OwnedBy = vendor`, `Type = "github-copilot"`, context/input/output limits, thinking support, plus
  a new registry field `UpstreamEndpoint` (`/chat/completions` | `/responses` | `/v1/messages`) used to pick the wire
  protocol. It plugs into the normal per-auth model registration switch (`sdk/cliproxy/service_models.go` `case
  "github-copilot"`), so **aliases, exclusions and prefixes apply exactly as for other OAuth channels** — and therefore,
  absent a prefix, **any Copilot model whose GitHub id collides with a native id would pool with that native provider**
  (§1d). The doc explicitly acknowledges this and points at prefixes/aliases.
- **Executor** (`internal/runtime/executor/copilot_executor.go`): a thin `CopilotExecutor{Identifier()="github-copilot"}` that,
  per request, exchanges for the Copilot token, clones the auth and injects `base_url`, `api_key` (the Copilot token),
  `header:*` attributes (`X-Request-Id`, `Openai-Intent: conversation-panel`, the VS Code headers), then **delegates** to
  the existing `OpenAICompatExecutor` (for `/chat/completions` or `/responses`, via a new `upstreamFormat` field) or to a
  `ClaudeExecutor` re-labelled with `provider: "github-copilot"` (for `/v1/messages`). `Refresh` goes through
  `helps.RefreshAuthViaHome` first (so it is Home-aware in the same way as native providers), else re-exchanges the token
  and refreshes quota. `responses/compact` and image endpoints return 501.
- **Selector integration:** it is a normal native provider — added to `baselineExecutorAuths` and `registerExecutorForAuth`
  (`sdk/cliproxy/service_executors.go`), authenticator registered in `newDefaultAuthManager` (`service_auth.go`), refresh
  lead 1 min (`sdk/auth/refresh_registry.go`). Adds `GetModelForClient(clientID, modelID)` to the registry so the executor
  can read *its own* client's model metadata rather than another provider's definition of the same id (a direct
  acknowledgement that same-id-different-provider collisions exist).
- **Quota handling:** `helps.CopilotQuotaError` (`internal/runtime/executor/helps/copilot.go`) wraps an upstream **402
  Payment Required** into an error that reports `StatusCode()==429` and `IsCredentialScoped()==true`, so exhausted
  premium-request allowance goes through the standard 429 quota cooldown with credential scope (§1g). A management route
  `GET /v0/management/github-copilot-quota?auth_index=…` returns GitHub's raw `copilot_internal/user` snapshot for the UI.
- **Tests:** 249 lines auth tests, 175 executor tests, plus registry/TUI/management tests.

**Maintainer position — issue #4317** (https://github.com/router-for-me/CLIProxyAPI/issues/4317, closed 2026-07-15 by
`luispater`, ~3 hours after filing, "not planned"): *"GitHub Copilot sits behind Microsoft/GitHub's own gateway rather
than being a first-party model provider. That kind of middle-layer / resale access does not meet our criteria for
first-class upstream provider integration, so we do not plan to add Copilot as a top-level provider."* Nothing in PR #5661's
thread contradicts this — no maintainer has engaged with it, and the plugin framework (with its official plugin store,
§1c) is upstream's designated answer for non-first-party providers.

**Honest maintenance-cost comparison — vendoring #5661 into a CPA fork vs. the plugin:**

| | Vendor #5661 into our CPA fork | Community plugin (`.so`) |
|---|---|---|
| Upstream acceptance | Effectively nil (#4317 explicit "not planned"; PR untouched by maintainers 15 days; CONFLICTING) | Plugin framework is the sanctioned path; no upstream change needed |
| Surface we own | 29 files, +1423 lines touching **core** files that churn every release: `openai_compat_executor.go`, `service_models.go`, `service_executors.go`, `service_auth.go`, `config.go`, `main.go`, `model_registry.go` (9 of them already conflict vs `dev` after 15 days / 329 commits) | Zero CPA source changes. A separate repo with its own release cadence |
| Rebase burden | Every CPA upgrade (upstream ships ~17 minor tags in 11 days between 7.2.159 and 7.3.16) = re-resolve conflicts in hot files, re-run their tests, re-build **both** the node image and (per FLEET.md) all Ark/mini deploys. We would also be forking CPA for the first time — today only Home is forked | Re-verify `abi_version==1`, `schema_version<=6` on CPA upgrades; rebuild the `.so` only when the plugin's SDK pin or GitHub's internal endpoints change |
| Home coupling | Home must recognise provider `github-copilot`, its auth file shape, refresh (Home refresh path, §1b), and quota — Home-side work either way | Same Home-side work, plus Home's plugin-sync/plugin-auth plumbing (Task 2) must exist in the Home build the Ark runs |
| GitHub internal-API drift | We patch Go in a fork and redeploy CPA | Plugin author (or we) patch the plugin; hot-swappable via `plugins/` dir + `plugin.reconfigure` |
| Debuggability | In-process, native logging, `go test` | In-process too (cgo dlopen), but a panic **fuses** the plugin (`fusePlugin`, `adapters_usage_translation.go:96-107`) rather than crashing the node; RPC boundary adds JSON marshalling overhead per chunk |
| Quota semantics | 402→429 credential-scoped mapping already done | Plugin must emit `http_status: 429` itself (§1g); no `CredentialScope` control from the ABI |

Bottom line: vendoring #5661 is technically clean **against 7.2.159 today** (merge-tree is conflict-free) but it turns CPA
into a permanently-forked component with a 9-file conflict tax that started accruing within two weeks, against an explicit
upstream "not planned". The plugin route keeps CPA pristine and moves the maintenance to a bounded artefact — its real
costs are (a) making sure the fleet's nodes are cgo builds with `plugins.enabled`, (b) Home-side support for a plugin
provider's auth/refresh/dispatch, and (c) the model-id collision question, which is **Home's** to answer on the fleet
(§1d) and which #5661 does not solve either (it relies on the same prefix/alias tools).

---

## 3. Things I looked for and did not find

- No out-of-process / gRPC plugin transport at this commit — only cgo `dlopen` (§1c). Not found.
- No provider allowlist in usage push (§1f). Not found.
- No `alias` field on `pluginapi.ModelInfo`; aliasing is host-config only (§1e).
- No way for a plugin to set `RetryAfter` or `CredentialScope` on an error via `pluginabi.Error` (§1g). Not found.
- No maintainer comment on PR #5661 at all. Not found.
- Whether the minis' installed binary is a cgo build: **not verifiable from this laptop** (needs
  `readelf -l` / `ldd` on a mini, or knowing whether it came from the Docker image vs a `*_no-plugin.tar.gz`). Flagging
  as an open check for the lead.

---

## 4. Follow-ups (all at `ac02da6c` / v7.2.159)

### 4.1 A plugin that declares only `["openai-response","claude"]` — what happens to OpenAI-chat / Gemini clients?

**Short answer: the host translates, it does not reject or pass through.** For this plugin every client endpoint the
fleet uses works, because the built-in translator registry has request+stream+non-stream pairs from every client format
into `claude`. The negotiation is entirely host-side in `internal/pluginhost/adapters_executors.go`; the plugin only ever
sees a `claude`-format (or `openai-response`-format) payload.

**Step 1 — what format does a request arrive in?** Each API handler passes a `handlerType` string that becomes
`opts.SourceFormat` (and, absent an override, `opts.ResponseFormat`): `executeWithAuthManager(ctx, handlerType, …)` →
`executeWithAuthManagerFormats(ctx, entryProtocol=handlerType, exitProtocol=handlerType, …)`
(`sdk/api/handlers/handlers_execution.go:41-45`) → `SourceFormat: sdktranslator.FromString(entryProtocol)`,
`ResponseFormat: sdktranslator.FromString(responseProtocol)` (`:282-283`). Handler literals found (grep over
`sdk/api/handlers`): `"openai"` (chat completions, 56 call sites), `"openai-response"` (Responses API, 4),
`"claude"` (3), `"gemini"` (1), `"interactions"` (1), `"antigravity"` (1), `"openai-image"` (2). The format constants are
`openai`, `openai-response`, `claude`, `gemini`, `codex`, `antigravity`, `interactions` (`sdk/translator/formats.go:4-12`).

**Step 2 — the plugin's declared formats are normalised.** `ExecutorInputFormats/OutputFormats` go through
`normalizeExecutorFormats` → `normalizeExecutorFormatName` (`internal/pluginhost/adapters.go:64-98`): `"openai-response"`
hits the `default` branch → `FromString("openai-response")` = `FormatOpenAIResponse`; `"claude"` → `FormatClaude`
(`"anthropic"` is also accepted as an alias, `:93-94`). Declared **order is preserved** (`:68-82`) — it matters below.

**Step 3 — input format selection** (`selectExecutorInputFormat`, `adapters_executors.go:462-475`), run inside
`prepareExecutorCall` (`:399-428`) on every `Execute`/`ExecuteStream` (`:687`, `:745`):

```go
if executorFormatContains(a.inputFormats, requested) { return requested }          // exact match → pass through
for _, format := range a.inputFormats {                                           // declared order
    if requested == "" || sdktranslator.HasRequestTransformer(requested, format) { return format }
}
return "", fmt.Errorf("plugin executor %s does not support input format %q", …)   // → request fails
```

`HasRequestTransformer(from, to)` is a lookup in `defaultRegistry.requests[from][to]`
(`sdk/translator/registry.go:113-123`, `:268-270`), populated by `translator.Register(from, to, request, response)`
(`registry.go:30-45`) from the `init()` files under `internal/translator/<provider>/<client>/…`. The registered
request pairs **into `claude`** at this commit: `OpenAI→Claude` (`internal/translator/claude/openai/chat-completions/init.go:10-18`),
`OpenaiResponse→Claude` (`claude/openai/responses/init.go:10-18`), `Gemini→Claude` (`claude/gemini/init.go:10-19`),
`Interactions→Claude` (`claude/interactions/init.go:10`). Pairs **into `openai-response`**: only `Interactions→OpenaiResponse`
(`openai/interactions/responses/init.go:19`). There is **no `OpenAI→OpenaiResponse` and no `Gemini→OpenaiResponse`
request translator** (full list in the command output of this research; 30 pairs). If the translation is needed the host
runs it itself: `nativeReq.Payload = sdktranslator.TranslateRequest(inputRequested, inputFormat, req.Model, req.Payload, opts.Stream)`
(`adapters_executors.go:413-415`) and sets `nativeOpts.SourceFormat = inputFormat` (`:417`).

**Step 4 — output format selection** (`selectExecutorOutputFormat`, `:477-493`): exact match on the client's
`ResponseFormat` wins; else if the chosen input format is also a declared output and a response translator exists from it
back to the client format (`executorResponseTranslationAvailable(inputFormat, requested)`, `:495-503` →
`HasResponseTransformer(requested, inputFormat)`, i.e. `responses[client][provider]`, `registry.go:126-136`), use that;
else scan declared outputs. The response is then translated host-side: non-stream via `sdktranslator.TranslateNonStream(outputFormat→requestedFormat)`
(`:544`), stream via `TranslateStream` per chunk (`:602`), with the guard that an *unchanged* frame when a native stream
translator exists is treated as a failed translation and dropped (`executorStreamTranslationFellBack`, `:614-625`). The
`claude`-target `init.go`s all register both `Stream` and `NonStream` response transforms (e.g.
`claude/openai/chat-completions/init.go:14-17`, `claude/gemini/init.go:14-18`, `claude/openai/responses/init.go:14-17`).

**Resulting matrix for `inputs = outputs = ["openai-response","claude"]`:**

| Client endpoint (handlerType) | Input chosen | Output chosen | Works? |
|---|---|---|---|
| `POST /v1/messages` (`claude`) | `claude` (exact, `:466-467`) | `claude` (exact, `:481-482`) | ✅ pure pass-through |
| `POST /v1/responses` (`openai-response`) | `openai-response` (exact) | `openai-response` (exact) | ✅ pass-through (+ `EnsureResponsesUsageDetails`, `:525-527`) |
| `POST /v1/chat/completions` (`openai`) | loop: `openai-response`? `HasRequestTransformer(openai, openai-response)`=**no**; `claude`? **yes** → `claude` | `openai` not declared; `claude` is declared and `HasResponseTransformer(openai, claude)`=yes → `claude` (`:484-486`) | ✅ host translates OpenAI⇄Claude both ways |
| Gemini (`gemini`, `/v1beta/models/...:generateContent`) | `openai-response`? no; `claude`? `Gemini→Claude` **yes** → `claude` | `claude` + `HasResponseTransformer(gemini, claude)`=yes | ✅ host translates Gemini⇄Claude |
| Gemini Interactions (`interactions`) | `openai-response`? `Interactions→OpenaiResponse` **yes** (first in declared order!) → `openai-response` | `openai-response`? not the client's format; `HasResponseTransformer(interactions, openai-response)` — registered with the pair → `openai-response` | ✅ but goes via the Responses path, not Claude, purely because of declaration order |
| `openai-image`, `antigravity` | no translator into either declared format | — | ❌ `does not support input format` error (`:474`) — expected, not a fleet use case |

Two practical notes: (1) because the search is in **declared order**, `["openai-response","claude"]` sends Interactions
clients down the Responses path while `["claude","openai-response"]` would send them to Claude — harmless either way but
worth knowing when comparing behaviour with the plugin's own docs; (2) `CountTokens` uses the same `prepareExecutorCall`
(not separately verified per-endpoint here). On a Home node nothing changes: the executor is still resolved locally by
provider key (§1d) and `prepareExecutorCall` runs identically; Home does not do format negotiation.

### 4.2 macOS arm64 (darwin/arm64) support for plugin loading

**Loader build tag — darwin is in.** `internal/pluginhost/loader_unix.go:1` is `//go:build cgo && (linux || darwin || freebsd)`.
The `-ldl` link flags are only added for linux/freebsd (`loader_unix.go:6-7`; macOS has `dlopen` in libSystem, no flag
needed). The stub loader that refuses everything is `//go:build !cgo && !windows` (`loader_unsupported.go:1`). So on a
Mac the only way to get "plugin loading requires cgo" is a `CGO_ENABLED=0` binary.

**Extension — `.dylib`.** `pluginExtension(goos)` returns `".dylib"` for `"darwin"` (`internal/pluginhost/platform.go:103-106`),
and discovery filters on `strings.HasSuffix(strings.ToLower(entry.Name()), extension)` with
`extension := pluginExtension(runtime.GOOS)` (`platform.go:127`, `:144`). `.so` files in the directory are **ignored** on
a Mac. Valid file names are `<id>.dylib` or `<id>-v<semver>.dylib` (`pluginFileFromPath`, `:62-96`).

**Search path — `<dir>/darwin/arm64` first, then `<dir>`.** `candidateDirs(root, runtime.GOOS, runtime.GOARCH)` returns
`[filepath.Join(root, goos, goarch), root]` (`platform.go:308-313`, called at `:126`). With the default `plugins.dir`
(`"plugins"`, `internal/config/plugin_path.go:10`) that is `plugins/darwin/arm64/<id>.dylib` or `plugins/<id>.dylib`,
the former winning on id collision (dirs are scanned in that order and the first-seen id is kept unless a desired version
says otherwise, `:149-160`). The Home-driven sync uses the same layout (`internal/homeplugins/sync.go:568` `pluginCandidateDirs`,
`:639` `pluginExtension`), and the node tells Home its `GOOS/GOARCH` in the sync request (`sdk/cliproxy/home_plugins.go:71-77`)
so Home must have a `darwin/arm64` artefact for the plugin in its store.

**Release workflow — the darwin archives ARE cgo builds.** `.github/workflows/release.yaml`, job `build ${{ matrix.target }}`
(`:95-125`):

```yaml
matrix:
  include:
    - target: darwin-amd64   runner: macos-15-intel  goos: darwin  goarch: amd64  asset_arch: amd64    archive_format: tar.gz
    - target: darwin-arm64   runner: macos-15        goos: darwin  goarch: arm64  asset_arch: aarch64  archive_format: tar.gz
    - target: windows-amd64  runner: windows-latest  goos: windows goarch: amd64  asset_arch: amd64    archive_format: zip
    - target: windows-arm64  runner: windows-11-arm  goos: windows goarch: arm64  asset_arch: aarch64  archive_format: zip
```

and its build step is `CGO_ENABLED=1 GOOS="$GOOS" GOARCH="$GOARCH" go build … -o "$archive_dir/$binary_name" ./cmd/server/`
(`:176-178`), producing `CLIProxyAPI_<ver>_darwin_aarch64.tar.gz` (`:172`). The **only** `CGO_ENABLED=0` artefacts are the
Linux `*_no-plugin.tar.gz` (`:425-431`, with a `readelf` check that it is static) and the FreeBSD `*_no-plugin` build
(`:573-579`); the Linux cgo archives are at `:305`. The release notes template says exactly this (`:45`).
⇒ If the minis run the official `darwin_aarch64` tarball (or a `CGO_ENABLED=1` build of the same tree), plugin loading
is supported. What is still **not verifiable from this laptop**: which artefact the minis actually run. A quick on-box
check would be `otool -L <path>/cli-proxy-api` (a cgo Mach-O links `/usr/lib/libSystem.B.dylib` and, for cgo, shows
`libresolv`/`CoreFoundation` etc.) or simply `plugins.enabled: true` + a `.dylib` in place and watching for the
"requires cgo" error string from `loader_unsupported.go:10`.

One more darwin-specific point: because `loader_unix.go` uses `dlopen(RTLD_NOW|RTLD_LOCAL)` (`:43-45`), a `.dylib` that
is unsigned/quarantined will be refused by macOS Gatekeeper on Apple silicon unless the quarantine attribute is cleared
or the dylib is ad-hoc signed — that is an OS behaviour, not visible in CPA code (not found in the repo). Flagging for the
deploy runbook rather than as a CPA fact.

### 4.3 `modelHasNativeExecutor` — does the guard drop a plugin's per-auth model that collides with a native id?

**Precise scope of the guard: it applies ONLY to the plugin's static models at executor-registration time, not to
`ModelsForAuth`.**

- `Host.RegisterExecutors` (`internal/pluginhost/adapters_executors.go:34-129`) iterates `registration.models` where
  `registration := h.modelRegistration(record.id)` (`:60`, `:103`) — and `modelRegistrations[pluginID]` is populated
  **only** by `Host.RegisterModels` from `StaticModels`/`RegisterModels` responses (`adapters.go:248-289`,
  `nextModelRegistrations[record.id] = …` at `:283-289`, committed at `adapters_interceptors.go:393`). `ModelsForAuth`
  results never enter `modelRegistrations`.
- Within that static set the guard is **per model id, silent, and non-fatal**: `if h.modelHasNativeExecutor(manager, modelRegistry, modelID) { continue }`
  (`:79-81`) skips just that id (no log line at this site), where `modelHasNativeExecutor` returns true if **any** provider
  currently registered in the global registry for that model id has a non-plugin executor
  (`:280-290` → `modelRegistry.GetModelProviders(modelID)` then `providerHasNativeExecutor`, `:272-278`). The plugin is
  **not** refused; only if *every* one of its static models is claimed does the plugin's executor get skipped entirely
  (`if len(registration.models) > 0 && len(selectedModels[record.id]) == 0 { continue }`, `:104-106`), and even then a
  plugin with **zero** static models (pure `ModelsForAuth` provider) still has its executor registered (`:65-67` only
  `continue`s when `len(registration.models)==0` inside the *claim* loop; the second loop's `:104` condition is false
  for it, so it proceeds to `:115-116`). A separate, stronger guard: a plugin can never take over a **provider key** that
  a native executor owns (`:61-64`, `:111-113`).
- **Per-auth path (`ModelsForAuth`) has no native-collision guard at all.** `Host.ModelsForAuth` (`adapters.go:304-374`)
  matches the plugin by provider key (`:308-340`), calls the plugin (`:341`), converts models (`:353-362`) and returns
  them; `tryRegisterPluginModelsForAuth` (`sdk/cliproxy/service_executors.go:490-562`) applies exclusions/aliases/prefix
  (`:554-557`) and registers **all** of them under `clientID = auth.ID`, `provider = plugin's key`
  (`registerResolvedModelsForAuth`, `:420-447` → `GlobalModelRegistry().RegisterClient(a.ID, providerKey, models)`, `:446`).
  `RegisterClient` (`internal/registry/model_registry.go:304-…`) just increments `Providers[provider]` for each id —
  nothing checks whether another provider already serves it. So a Copilot plugin auth returning `claude-sonnet-4-5-20250929`
  on the same node as a `claude` OAuth auth yields `models["claude-sonnet-4-5-20250929"].Providers = {claude: n, <copilot-key>: 1}`,
  and `GetModelProviders` returns both (`:1428-…`) ⇒ **one mixed pool, round-robined across providers** (§1d,
  `scheduler.go:439-470`). Nothing is dropped and nothing is refused. The only thing that *would* drop it is the
  `ExecutorModelScope` gate (`executorScopeAllowsOAuthModels`, `adapters.go:317-319`) — a plugin declaring
  `executor_model_scope: "static"` gets no per-auth models at all, which is the opposite problem.
  (Corollary: the guard at `:79-81` protects native providers from the plugin's *static* catalogue only; PR #5661's added
  `GetModelForClient` exists precisely because per-auth collisions are otherwise unguarded.)

**Does any of this matter on a Home-managed node? No — the node registers no per-auth models when `home.enabled`.**

- Auth loading is skipped: `if s.coreManager != nil && !homeEnabled { s.coreManager.Load(ctx) … registerAvailableExecutors(auths: s.coreManager.List()) … }`
  (`sdk/cliproxy/service_lifecycle.go:77-94`); token/api-key providers are skipped too (`:96-110`). So `coreManager.List()`
  is empty, and every path that would call the plugin's `ModelsForAuth` is driven by that list:
  `syncPluginModelRuntime` → `registerAvailableExecutors{auths: s.coreManager.List()}` + `refreshPluginModelRegistrations`
  → `registerModelsForAuthBatch(ctx, s.coreManager.List())` (`sdk/cliproxy/service_plugins.go:129-162`; early return on
  `len(auths)==0`, `:165-167`); the per-auth hook `completeModelRegistrationForAuth` (`service_auth.go:360-382`) is fed by
  the watcher/auth-update queue, which is only wired in the non-Home branch (`service_lifecycle.go:96-110`, watcher
  wrapper `sdk/cliproxy/service_lifecycle.go:193`).
- Home-dispatched auths never enter `coreManager.auths`: `pickHomeDispatchSelection` builds the `Auth` from the RESP
  payload (`conductor_home.go:1101-1113`) and only caches it per session in `m.homeRuntimeAuths` (`:874-895`,
  field `sdk/cliproxy/auth/conductor.go:150` — "retains legacy session auth lookups for non-execution callers"), never
  in `m.auths` (`conductor.go:144`). `tryRegisterPluginModelsForAuth`'s `coreManager.Update` (`service_executors.go:528`)
  is therefore never reached for a Home auth.
- What a Home node **does** still do for plugins: `syncPluginModelRuntime` runs on Home config commits/plugin syncs
  (`service_lifecycle.go:129`, `:204`; `service_config.go:201`) and calls `pluginHost.RegisterModels` (static models →
  registry, `service_plugins.go:136`) and `registerAvailableExecutors{includeBaseline: homeEnabled, includePlugins: true}`
  (`:143-148`) → `registerPluginExecutors` → `Host.RegisterExecutors` (`service_executors.go:196-197`, `service_plugins.go:44-49`).
  So the plugin's **executor is registered under its provider key** (needed for `executorKeyFromAuth` lookup at
  `conductor_home.go:1168`), and its **static** catalogue (if any) lands in the local registry subject to the
  `modelHasNativeExecutor` guard — but with `includeBaseline` the native executors (`claude`, `codex`, … `baselineExecutorAuths`,
  `service_executors.go:201-213`) are registered on a Home node too, so a plugin static model id that a native provider
  *also* has in the local registry would be skipped. With no native auths loaded, native providers have no registry
  entries unless a native static catalogue is registered elsewhere (not traced further; static-model plugins are not the
  Copilot case).
- The model list a client sees on a Home node comes **entirely from Home**: `/v1/models` (and the Codex `client_version`
  variant) branch on `s.cfg.Home.Enabled` to `handleHomeModels` / `handleHomeCodexClientModels`
  (`internal/api/server_routes.go:575-588`), which call `loadHomeModelEntries` → `home.Current().GetModels(ctx, headers, query)`
  (`:831-856`; RESP `GetModels` at `internal/home/client.go:983`). The local registry is not consulted for listing, and
  selection is `pickNextViaHome` (§1d). ⇒ **On the fleet, model-id pooling between a Copilot plugin credential and a
  native Claude credential is decided 100% by Home's dispatcher and Home's model catalogue, and the CPA-side guards in
  this section are irrelevant to that decision.** The only CPA-side requirement is the executor-key match.
