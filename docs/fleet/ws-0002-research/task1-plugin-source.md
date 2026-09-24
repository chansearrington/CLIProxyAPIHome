# Task 1 — Community CPA Copilot plugin: source read-through

Scope: read-only research of https://github.com/arthur-sommer-etc/cliproxyapi-copilot-plugin (MIT).
Clone lives at `/tmp/ws-0002-research/copilot-plugin`; all `file:line` references below are relative to that clone.
For cross-checks against the CPA host I also shallow-cloned CPA at the plugin's pinned tag
(`/tmp/ws-0002-research/cpa-sdk/CLIProxyAPI`, tag `v7.2.118`, commit `29bdd3c1492c383b89d0be76b353a2891aa04c54`);
those references are marked **[CPA]**.

Nothing was installed, run, built, or logged into.

---

## 1. Repository facts

| Item | Value | Evidence |
|---|---|---|
| HEAD commit | `7f16b6011e93266f6d317166ef7cb6f0262f511f` | `git rev-parse HEAD` |
| Last commit | 2026-08-07 09:32:49 -0500 — "docs: route Sonnet alias to claude-opus-5" | `git log -1` |
| Total commits | 11 (all between 2026-08-05 and 2026-08-07) | `git rev-list --count HEAD`; `git log` |
| Tags (creator date) | `v0.3.0` 2026-08-05, `v0.3.1` 2026-08-05, `v0.3.2` 2026-08-05, `v0.3.3` 2026-08-05 | `git tag -l --format='%(refname:short) %(creatordate:short)'` |
| Latest tag | `v0.3.3` (= current HEAD minus two docs-only commits) | `git describe --tags --abbrev=0` |
| GitHub releases | v0.3.0 … v0.3.3, all 2026-08-05; v0.3.3 marked Latest | `gh release list` |
| Repo created / last push | 2026-08-05T14:05:10Z / 2026-09-08T16:33:58Z | `gh repo view` |
| Stars / forks / archived | 2 / 8 / no | `gh repo view` |
| Open issues | **none** (`gh issue list --state all` returns `[]`) | `gh issue list` |
| Open PRs | #1 "fix: preserve OAuth model prefixes" (2026-08-24, author Goodwu); #2 "构建: 支持 Linux 与 macOS 双架构发布" (Linux+macOS multi-arch builds, 2026-08-24, author Goodwu). Both still OPEN, unmerged. | `gh pr list --state all` |

Commit history (`git log --format='%h %ad %s' --date=short`):
```
7f16b60 2026-08-07 docs: route Sonnet alias to claude-opus-5
53c81c4 2026-08-07 docs: replace setup scripts with documented commands
3a8246f 2026-08-05 Avoid model collisions with native providers
3f75a8c 2026-08-05 Support Claude token counting for Copilot
7805136 2026-08-05 Preserve whitespace in streamed text deltas
d17f983 2026-08-05 Add MIT license
71378a5 2026-08-05 Automate plugin packages and releases
7edb064 2026-08-05 Document existing deployment installation
52344fe 2026-08-05 Use the default CLIProxyAPI port
46d9610 2026-08-05 Rename project identifiers to CLIProxyAPI
61c423f 2026-08-05 Add GitHub Copilot provider for CLIProxyAPI
```
Practical read: a three-day burst by one author in early August 2026, untouched since (apart from two unmerged community PRs). Total Go source is 4,183 lines across 24 files (`wc -l`).

## 2. `go.mod`

`go.mod:1-10`:
```
module github.com/arthur-sommer-etc/cliproxyapi-copilot-plugin
go 1.26.0
require (
    github.com/router-for-me/CLIProxyAPI/v7 v7.2.118
    github.com/tidwall/gjson v1.18.0
    github.com/tiktoken-go/tokenizer v0.8.1
    gopkg.in/yaml.v3 v3.0.1
)
```
- CPA pin: **`github.com/router-for-me/CLIProxyAPI/v7 v7.2.118`** (`go.mod:6`).
- Go version: **1.26.0** (`go.mod:3`).
- `replace` directives: **not found** (none in `go.mod`).
- Indirect deps include logrus, go-redis, x/net, protobuf (`go.mod:12-26`) — pulled in transitively via the CPA module.
- The plugin imports three CPA SDK packages: `sdk/pluginabi` (`cmd/cliproxyapi-copilot/main.go:68`), `sdk/pluginapi` (`cmd/cliproxyapi-copilot/dispatch.go:11`), and `sdk/translator` + `sdk/translator/builtin` (`internal/translate/translate.go:9-10`).

## 3a. ABI / schema version, capabilities, provider key

**ABI version 1, registration schema version 2** — both taken from the SDK constants, not hard-coded:
- `cmd/cliproxyapi-copilot/main.go:75` — `cliproxy_plugin_init` rejects the host unless `uint32(host.abi_version) == pluginabi.ABIVersion`; `main.go:79` sets `plugin.abi_version = C.uint32_t(pluginabi.ABIVersion)`.
- `cmd/cliproxyapi-copilot/dispatch.go:163` — registration payload uses `SchemaVersion: pluginabi.SchemaVersion`.
- **[CPA]** `sdk/pluginabi/types.go:7` `ABIVersion uint32 = 1`; `types.go:10` `SchemaVersion uint32 = 2`.
- README confirms: "implements ABI version 1 and registration schema 2" (`README.md:34`).

**Capabilities registered** (`dispatch.go:181-188`):
```go
Capabilities: registrationCapability{
    ModelProvider:         true,
    AuthProvider:          true,
    Executor:              true,
    ExecutorModelScope:    pluginapi.ExecutorModelScopeOAuth,
    ExecutorInputFormats:  []string{"openai-response", "claude"},
    ExecutorOutputFormats: []string{"openai-response", "claude"},
},
```
So: AuthProvider + ModelProvider + ProviderExecutor. **Not registered**: FrontendAuth, Scheduler, ModelRouter, request/response translators/normalizers/interceptors, Thinking, Usage, CommandLine, Management (the `dispatch()` switch at `dispatch.go:76-158` returns `unknown_method` / HTTP 501 for anything else, `dispatch.go:152-157`). `ExecutorModelScopeOAuth` means it only serves auth-bound (OAuth-discovered) models, never static ones — and indeed `StaticModels()` returns an empty list (`internal/provider/models.go:74-76`).

**Provider key: `"copilot"`.**
- `internal/provider/service.go:10` — `const providerID = "copilot"`.
- `dispatch.go:98-99` — both `auth.identifier` and `executor.identifier` answer `identifierResponse{Identifier: "copilot"}`.
- Every `AuthData` carries `Provider: providerID` (`internal/provider/storage.go:75`); `ModelResponse.Provider` is `providerID` (`models.go:87`).

**Plugin ID (distinct from provider key): `cliproxyapi-copilot`**, derived by the host from the `.so` filename (`Makefile:4` builds `cliproxyapi-copilot.so`; `docs/install-existing-deployment.md:22-24`: "CLIProxyAPI derives the plugin ID `cliproxyapi-copilot` from it"). **[CPA]** `internal/pluginhost/platform.go:47-58` (`pluginIDFromPath`) strips the extension; an optional `-vX.Y.Z` suffix is parsed as version (`platform.go:82-90`). The config key under `plugins.configs` must match this ID (`config/config.yaml:36`).

**Metadata** (`dispatch.go:164-179`): Name "GitHub Copilot subscription provider", `Version` from `pluginVersion` var (`dispatch.go:15`, default `"0.3.3"`, overridden by `-ldflags "-X main.pluginVersion=$(VERSION)"` at `Makefile:6`), Author "self-owned", plus nine declared `ConfigFields` (see 3d).

## 3b. Auth: device-code login, storage, identity, Copilot token

**Flow** (all HTTP goes through host callbacks, never a direct `net/http` client — see `internal/transport/transport.go:33-40` and `cmd/cliproxyapi-copilot/host.go:55-71`):
1. `auth.login.start` → `Service.StartLogin` (`internal/provider/oauth.go:87-170`): POST `{github_base_url}/login/device/code` with `client_id` (+ `scope`) (`oauth.go:92-105`). Stores an in-memory `deviceSession` keyed by a random 24-byte hex `state` (`oauth.go:132-147`). Returns `AuthLoginStartResponse{Provider:"copilot", URL: verification_uri_complete (or verification_uri?user_code=…), State, ExpiresAt, Metadata{user_code, interval}}` (`oauth.go:160-169`). Device-code lifetime clamped to `oauth_timeout_seconds` (`oauth.go:123-128`); poll interval floored at 5 s (`oauth.go:129-131`).
2. `auth.login.poll` → `Service.PollLogin` (`oauth.go:172-265`): POST `{github_base_url}/login/oauth/access_token` with `grant_type=urn:ietf:params:oauth:grant-type:device_code` (`oauth.go:20`, `196-211`). Handles `authorization_pending`, `slow_down` (+5 s), and terminal errors via `classifyDeviceToken` (`oauth.go:283-322`). Server-side rate limiting: a poll before `NextPoll` or while one is in flight returns `pending` without hitting GitHub (`oauth.go:186-189`).
3. On success it GETs `{github_api_url}/user` with `Authorization: Bearer <access_token>` to learn `login` and numeric `id` (`fetchGitHubUser`, `oauth.go:324-351`); a missing login is a hard error (`oauth.go:347-349`).
4. Builds the credential and returns it as `AuthLoginPollResponse{Status: success, Auth: <AuthData>}` (`oauth.go:236-264`).

**Storage: host-owned, not written by the plugin.** The plugin never calls `host.auth.save`/`host.auth.get` and never touches the filesystem (grep for `os.WriteFile`/`MethodHostAuthSave` in the plugin: not found). It returns `pluginapi.AuthData` from `auth.login.poll`, `auth.parse`, and `auth.refresh`, and the host persists it. The host-side interface is CPA's `AuthProvider` contract: `auth.identifier` / `auth.parse` / `auth.login.start` / `auth.login.poll` / `auth.refresh` (**[CPA]** `sdk/pluginabi/types.go:22-26`; `sdk/pluginapi/types.go:263` `type AuthProvider interface`). The README states the same: "returned through CLIProxyAPI's `AuthProvider` storage contract and is persisted only in the isolated auth volume" (`README.md:72-74`); "does not need a separate credential volume" (`docs/install-existing-deployment.md:103-105`).

**Stored credential fields** — the provider-owned JSON put in `AuthData.StorageJSON` (`internal/provider/storage.go:14-26`):
```go
type authStorage struct {
    Type                  string `json:"type"`                    // always "copilot"
    GitHubAccessToken     string `json:"github_access_token"`
    GitHubRefreshToken    string `json:"github_refresh_token,omitempty"`
    TokenType             string `json:"token_type,omitempty"`
    Scope                 string `json:"scope,omitempty"`
    ExpiresAt             int64  `json:"expires_at,omitempty"`
    RefreshTokenExpiresAt int64  `json:"refresh_token_expires_at,omitempty"`
    GitHubLogin           string `json:"github_login"`
    GitHubUserID          int64  `json:"github_user_id,omitempty"`
    OAuthClientID         string `json:"oauth_client_id,omitempty"`
    UpdatedAt             string `json:"updated_at"`
}
```
The wrapping `AuthData` (`storage.go:53-87`) sets `Provider:"copilot"`, `ID`, `FileName`, `Label: storage.GitHubLogin`, `Metadata{type:"copilot", github_login}`, `Attributes{auth_kind:"oauth"}`, and `NextRefreshAfter` (10 min before `expires_at`, or now+24h when GitHub returned no expiry — `storage.go:89-98`). `Prefix` and `ProxyURL` are passed through as empty strings from every caller (`oauth.go:80`, `254`, `360`, `416`).

**Per-credential identity — two GitHub accounts do NOT collide, same account re-login overwrites:**
- File name is derived from the GitHub login: `credentialFileName(login)` → `"copilot-" + <lowercased login, non [a-z0-9-_.] → '-'> + ".json"` (`storage.go:100-118`), e.g. `copilot-chanse.json`. `AuthData.ID` defaults to the file name (`storage.go:58-63`).
- Two different logins → two different files → two distinct auths. The same login logging in twice yields the same file name, so the host will overwrite the earlier credential (this is by-design dedup, not a bug). Empty login falls back to `copilot-account.json` (`storage.go:113-116`), but an empty login is already rejected upstream (`oauth.go:347-349`).
- On `auth.parse` (host re-reading an existing file) the plugin keeps the host's `FileName` as both `id` and `fileName` (`oauth.go:80`); on `auth.refresh` it keeps `req.AuthID` and the host's existing `Metadata`/`Attributes` (`oauth.go:360`, `416`).
- **Caveat surfaced by open PR #1:** `authStorage` has no `prefix` field, so a `prefix` the operator adds to the credential JSON is dropped on parse and the models are registered without the namespace ("CLIProxyAPI therefore registered the discovered models without the configured credential namespace" — PR #1 body). The host itself falls back to the auth record's own prefix in some paths (**[CPA]** `internal/pluginhost/auth_provider.go:376-377`, `adapters_executors.go:729-730`), so behaviour depends on which path the host takes; PR #1 makes it deterministic by reading `prefix` from the JSON (`storage.go` diff, +5 lines). Unmerged as of HEAD.

**GitHub token refresh** (`RefreshAuth`, `oauth.go:353-421`): if `expires_at` is 0 or more than 10 min away it just re-emits the same data (`oauth.go:359-365`). Otherwise POSTs `grant_type=refresh_token` to `/login/oauth/access_token` (`oauth.go:376-391`); no refresh token → `auth_expired` 401 (`oauth.go:366-368`). Note the default client `Iv1.b507a08c87ecfe98` issues classic non-expiring `gho_` tokens with no refresh token (README `README.md:62-64` calls it "the public client identifier used by established Copilot device-flow clients"), so in practice `expires_at==0` and refresh is a no-op re-emit every 24 h.

**Short-lived Copilot token** (`internal/provider/token.go`):
- Obtained by GET `{github_api_url}/copilot_internal/v2/token` with `Authorization: token <github_access_token>` and `X-GitHub-Api-Version: 2025-04-01` (`token.go:82-93`). Response fields used: `token`, `expires_at`, `refresh_in`, `endpoints.api` (`token.go:17-23`).
- **Kept only in process memory**: `Service.tokenEntries map[string]copilotTokenEntry` (`service.go:22-24`), keyed by `authID` (falls back to a SHA-256 fingerprint of the GitHub token, `token.go:39-43`, `storage.go:120-123`). Cleared on shutdown (`service.go:62-72`). Never written into `StorageJSON`.
- Refreshed when `ExpiresAt` is within `token_expiry_buffer_seconds` (`token.go:48-53`); single-flight per key so concurrent requests share one exchange (`token.go:54-79`). Expiry parsed from `expires_at`, else from the `exp=` segment inside the token string, else `refresh_in`, else now+20 min (`token.go:121-139`).
- The API base URL comes from the token response's `endpoints.api`, falling back to `copilot_api_url` (`token.go:141-157`); HTTPS enforced unless `allow_insecure_base_urls`.
- A 401 from `/models` or an inference call invalidates the cache and re-exchanges once (`models.go:118-132`, `executor.go:120-133`, `151-165`).

## 3c. Model discovery and the exact model-ID format

**Endpoint**: GET `{APIBaseURL}/models` (API base from the Copilot token, default `https://api.githubcopilot.com`) with the full Copilot header set (`internal/provider/models.go:110-114`; headers in 3e). Result cached in memory per auth for `model_cache_ttl_seconds` (`models.go:149-157`), cleared on reconfigure (`service.go:50-52`).

**Upstream shape parsed** (`models.go:18-64`): `data[]` of `{id, vendor, name, version, object, model_picker_enabled, preview, supported_endpoints[], warning_messages[], info_messages[], capabilities{type, tokenizer, family, object, supports{tool_calls, parallel_tool_calls, streaming, vision, adaptive_thinking, reasoning_effort[]}, limits{max_inputs, max_prompt_tokens, max_output_tokens, max_non_streaming_output_tokens, max_context_window_tokens}}}`.

**Normalisation** (`normalizeModels`, `models.go:210-237`): trims, dedups case-insensitively, canonicalises endpoints to `/responses`, `/chat/completions`, `/v1/messages` (`normalizeEndpoints`, `models.go:260-281`), force-adds `/responses` for `gpt-5.6-sol` / `gpt-5.6-terra` (`models.go:228-230`, `specialResponsesModel` `models.go:201-208`), sorts by lowercase ID. Then `filterModels` drops excluded prefixes (`models.go:239-258`). Zero usable models is an error (`models.go:141-143`).

**Model ID exposed to CPA: the RAW Copilot id, unprefixed.** `modelInfos` (`models.go:283-343`):
```go
out = append(out, pluginapi.ModelInfo{
    ID:                         model.ID,                                   // models.go:323  ← raw, e.g. "gpt-4.1", "claude-sonnet-4"
    Object:                     firstNonEmpty(model.Object, "model"),
    OwnedBy:                    owner,                                      // models.go:325 (vendor, else "github-copilot")
    Type:                       firstNonEmpty(model.Capabilities.Type, "chat"),
    DisplayName:                displayName,                                // models.go:327 (upstream name, else id)
    Name:                       model.ID,
    Version:                    model.Version,
    Description:                modelDescription(model),
    InputTokenLimit:            model.Capabilities.Limits.MaxPromptTokens,
    OutputTokenLimit:           model.Capabilities.Limits.MaxOutputTokens,
    SupportedGenerationMethods: append([]string(nil), model.SupportedEndpoints...),
    ContextLength:              contextLength,
    MaxCompletionTokens:        model.Capabilities.Limits.MaxOutputTokens,
    SupportedParameters:        parameters,        // "stream","tools","tool_choice","parallel_tool_calls","reasoning_effort" as supported
    SupportedInputModalities:   inputModalities,   // "TEXT" (+ "IMAGE" if vision)
    SupportedOutputModalities:  []string{"TEXT"},
    Thinking:                   thinking,          // DynamicAllowed=adaptive_thinking, Levels=reasoning_effort
})
```
- `owned_by` = upstream `vendor` (e.g. `Anthropic`, `OpenAI`) or `"github-copilot"` when empty (`models.go:286-289`).
- `display_name` = upstream `name` or the id (`models.go:290-293`).
- `description` = first warning/info message from GitHub, else `"GitHub Copilot subscription model; family <f>; endpoints <…>"` (`models.go:345-359`).
- There is **no plugin-side prefixing** anywhere (`grep -n '"copilot/"\|copilot/` in the plugin: not found). Prefixing is purely a CPA-host feature driven by the credential's `prefix`: **[CPA]** `sdk/cliproxy/service_models.go:611-613` clones each model as `trimmedPrefix + "/" + baseID` and, unless `force-model-prefix` is on, also keeps the bare ID (`service_models.go:608-613`). With `prefix: copilot` on the credential you would see both `gpt-4.1` and `copilot/gpt-4.1` (PR #1's author "verified `/v1/models` exposes 46 `copilot/...` model IDs with a credential configured as `prefix: copilot`" — on CPA 7.2.140, and only with the PR #1 patch applied).

**Consequence for a fleet mixing Copilot with native Anthropic/OpenAI credentials**: a Copilot `claude-sonnet-4` and a native Anthropic `claude-sonnet-4` land in the same CPA model pool under the same ID, so the scheduler would treat them as interchangeable credentials for one model. That is exactly what the plugin's `excluded_model_prefixes` exists to avoid (3d, 3f).

## 3d. Config keys, defaults, and where they come from

Config is read from **the CPA `config.yaml` `plugins.configs.<plugin-id>` block**: the host serialises that block to YAML and passes it as `config_yaml` on `plugin.register` and `plugin.reconfigure` (`cmd/cliproxyapi-copilot/dispatch.go:17-19`, `79-89`); the plugin `yaml.Unmarshal`s it over defaults (`internal/provider/config.go:47-53`). No env vars, no plugin-specific file (grep `os.Getenv`: not found). Example block: `config/config.yaml:32-48` and `docs/install-existing-deployment.md:72-89`.

`internal/provider/config.go:19-31`:
```go
type Config struct {
    Enabled                  bool     `yaml:"enabled"`
    GitHubClientID           string   `yaml:"github_client_id"`
    GitHubScope              string   `yaml:"github_scope"`
    GitHubBaseURL            string   `yaml:"github_base_url"`
    GitHubAPIURL             string   `yaml:"github_api_url"`
    CopilotAPIURL            string   `yaml:"copilot_api_url"`
    AllowInsecureBaseURLs    bool     `yaml:"allow_insecure_base_urls"`
    OAuthTimeoutSeconds      int      `yaml:"oauth_timeout_seconds"`
    ModelCacheTTLSeconds     int      `yaml:"model_cache_ttl_seconds"`
    TokenExpiryBufferSeconds int      `yaml:"token_expiry_buffer_seconds"`
    ExcludedModelPrefixes    []string `yaml:"excluded_model_prefixes"`
}
```
Defaults (`config.go:33-45`) and validation (`config.go:54-82`):

| Key | Default | Validation |
|---|---|---|
| `enabled` | `true` | — (only gates `StartLogin`, `oauth.go:89-91`) |
| `github_client_id` | `Iv1.b507a08c87ecfe98` (`config.go:13`) | required |
| `github_scope` | `read:user` | — |
| `github_base_url` | `https://github.com` | absolute HTTPS, no query/fragment |
| `github_api_url` | `https://api.github.com` | same |
| `copilot_api_url` | `https://api.githubcopilot.com` | same; only a fallback when the token has no `endpoints.api` |
| `allow_insecure_base_urls` | `false` | permits `http://` for the three URLs above |
| `oauth_timeout_seconds` | 900 | 60–1800 |
| `model_cache_ttl_seconds` | 600 | 30–3600 |
| `token_expiry_buffer_seconds` | 300 | 30–900 |
| `excluded_model_prefixes` | `[]` (empty) | lower-cased, trimmed, deduped (`normalizeModelPrefixes`, `config.go:85-100`); matched with case-insensitive `strings.HasPrefix` (`models.go:245-251`) |

`priority: 100` in the sample config (`config/config.yaml:38`) is a host-level plugin field, not read by the plugin (not in `Config`).

**Not present**: include/allow lists, model aliasing/renaming, per-model overrides, prefix injection, proxy settings (host's `ProxyURL` is passed through empty). The registration's `ConfigFields` list (`dispatch.go:169-179`) omits `enabled` and `allow_insecure_base_urls` even though both are parsed.

## 3e. Executor: formats, streaming, tools, model-family routing, headers, usage

**Client-facing formats accepted**: only `openai-response` (OpenAI Responses) and `claude` (Anthropic Messages) — declared at `dispatch.go:186-187` and enforced by `normalizeRequestFormat` (`internal/provider/executor.go:244-253`): `""|responses|openai-response|openai-responses → "openai-response"`, `claude|anthropic → "claude"`, anything else → `unsupported_format` 422 (`executor.go:37-40`, `75-78`). **OpenAI Chat Completions and Gemini are NOT accepted as input formats** by this plugin (CPA's own front-door translators would have to convert them to one of the two first; whether the host does that for plugin executors is a host question, not answered here).

**Upstream endpoint selection per model** (`endpointForModel` → `selectEndpoint`, `models.go:161-199`): from the model's `supported_endpoints`, prefer `/responses`, then `/chat/completions`, then `/v1/messages` (`models.go:191`). `gpt-5.6-sol` / `gpt-5.6-terra` are hard-wired to `/responses` even if absent from the catalog (`models.go:201-208`, `168-171`, `180-182`). A requested model not in the authenticated catalog → `model_not_found` 404 (`models.go:183`).

**Wire-format handling for a Claude client request** (README `README.md:51-53`; code `internal/translate/translate.go`):
- Model supports `/v1/messages` only (Copilot's Anthropic models advertise this) → request passes through untouched when source == target (`translate.go:40-42`, `61-66`, `80-82`). So **Claude via Copilot is sent as native Anthropic Messages** when that endpoint is offered; if `/responses` or `/chat/completions` is also offered, those win by the preference order above and the request is translated.
- Target `/responses` → custom Claude→Responses bridge in `internal/translate/claude_responses.go:12-…` (system→`instructions`, tools→`function`, tool_choice mapping `claude_responses.go:51-64`, thinking→`reasoning.effort` `65-67`, images, tool_use/tool_result, `output_config`→`text.format`), and Responses→Claude on the way back (`claude_responses.go:185-252`, streaming in `claude_responses_stream.go`).
- Target `/chat/completions` → CPA's built-in translator registry (`builtin.Registry()`, `translate.go:19`), with a two-hop fallback via an intermediate format if no direct transformer exists (`translate.go:97-108`, `123-135`, `147-167`).
- Final request always gets `model` and `stream` overwritten (`setModelAndStream`, `translate.go:208-220`).

**Streaming**: yes, SSE. `executor.execute_stream` opens the upstream stream via `host.http.do_stream`, then a goroutine `pumpStream` reads chunks with `host.http.stream_read`, re-frames them with a `\n\n`/`\r\n\r\n`-aware decoder (`internal/sse/decoder.go:9-51`), translates each frame, and pushes to the client with `host.stream.emit` / `host.stream.close` (`executor.go:71-107`, `187-242`; host bridge `cmd/cliproxyapi-copilot/host.go:73-118`). Non-2xx on stream open is collected and returned as an `upstream_error` (`executor.go:95-101`, `169-185`).

**Tool / function calling**: supported in both directions of the Claude↔Responses bridge (`claude_responses.go:30-64`, `145-170`, `218-232`; stream `claude_responses_stream.go:92-96`, `120-128`, `248-260`) and by the built-in translators for chat completions. Malformed tool arguments return errors, not fallbacks (`README.md:309-311`).

**Token counting** (`executor.count_tokens`): Claude format only; estimated locally with tiktoken `O200kBase` over system+messages+tools+tool_choice text (`internal/provider/token_count.go:22-76`); returns `{"input_tokens": n}`. Never calls Copilot.

**Headers sent to Copilot on every inference and `/models` call** (`copilotHeaders`, `executor.go:292-313`, constants `executor.go:17-23`):
```
Accept: application/json | text/event-stream
Authorization: Bearer <copilot token>
Content-Type: application/json
Copilot-Integration-Id: vscode-chat
Editor-Plugin-Version: copilot-chat/0.35.0
Editor-Version: vscode/1.107.0
OpenAI-Intent: conversation-edits
User-Agent: GitHubCopilotChat/0.35.0
X-Agent-Task-Id: <random 16-byte hex>
X-GitHub-Api-Version: 2025-04-01
X-Initiator: user
X-Interaction-Type: conversation-edits
X-Request-Id: <same random hex>
```
Locked by test `internal/provider/executor_test.go:10-27`. README rationale: "Copilot rejects unrecognized `Copilot-Integration-Id` values … therefore use the recognized VS Code Copilot integration headers (`vscode-chat` / `copilot-chat`)" (`README.md:78-81`). **`X-Initiator` is hard-coded to `user` for every request** — there is no logic to mark follow-up/agent turns as `agent`, so from GitHub's side all traffic looks like user-initiated VS Code Chat. (Whether that changes premium-request accounting is a GitHub-policy question; the plugin has no code for it.) GitHub/device-flow calls use a different UA `CLIProxyAPI-Copilot-Plugin/0.1.0` (`oauth.go:431-433`).

**Restricted generic HTTP** (`executor.http_request`, `executor.go:255-290`): only same scheme+host as the token's API base (`executor.go:268-271`), and it overwrites `Authorization`, `User-Agent`, `X-GitHub-Api-Version` (`executor.go:273-275`).

**Usage / premium-request / quota reporting: none.**
- `grep -rni 'premium|quota|billing|usage.handle|MethodUsage'` over the plugin's Go and Markdown: **no matches**.
- The plugin does not register the Usage capability and returns 501 for `usage.handle` (`dispatch.go:152-157`).
- It does map token counts inside the translated response bodies (Responses `usage.input_tokens/output_tokens/input_tokens_details.cached_tokens` → Claude `usage.input_tokens/output_tokens/cache_read_input_tokens`, `claude_responses.go:236-251`; stream `claude_responses_stream.go:187-199`, `302-309`), so whatever the CPA host normally extracts from a response payload it can still extract. `ExecutorResponse.Metadata` carries only `copilot_endpoint` and `token_expires_at` (`executor.go:64-67`). **[CPA]** `internal/pluginhost/adapters_executors.go` contains zero references to "usage" (`grep -c`), so any usage accounting for plugin executors happens (if at all) in the generic host pipeline after the response comes back — not traced further here.

## 3f. Pooling / collisions with native providers

- The plugin registers models under **raw upstream IDs** with provider key `copilot` (3c). CPA pools credentials per model ID, so a Copilot-discovered `claude-sonnet-4` and a native Anthropic `claude-sonnet-4` become two credentials for **one** model ID; the plugin does nothing to disambiguate them.
- The only collision control is subtractive: `excluded_model_prefixes` (`config.go:30`, `models.go:239-258`). Commit `3a8246f 2026-08-05 "Avoid model collisions with native providers"` added it. README: "Copilot model prefixes can be excluded from discovery to avoid collisions with native providers; the included dual-subscription deployment excludes `claude-*` so native Claude OAuth always owns those model IDs" (`README.md:56-58`). The install doc repeats the rationale: "prevent duplicate Claude model IDs from being scheduled through Copilot" (`docs/install-existing-deployment.md:107-116`); the shipped compose config sets `excluded_model_prefixes: ["claude-"]` (`config/config.yaml:47-48`) while the "existing deployment" snippet ships it empty (`docs/install-existing-deployment.md:88`).
- The additive alternative — namespacing all Copilot models as `copilot/<id>` — is a host feature (credential `prefix`, **[CPA]** `service_models.go:611-613`) that this plugin **breaks** at HEAD because `authStorage` drops `prefix` on parse; open PR #1 fixes it (see 3b). Until PR #1 (or an equivalent local patch) is applied, `prefix: copilot` on the credential cannot be relied on.
- Model-ID matching is case-insensitive on the plugin side (`models.go:175`, `218`).

## 3g. Build and loading

- **Artifact type: a C-shared-library `.so` loaded with `dlopen`, NOT a Go `plugin`-package plugin, not a subprocess, not gRPC.**
  - Plugin side: `package main` with a cgo preamble defining the `cliproxy_host_api` / `cliproxy_plugin_api` structs and exporting `cliproxy_plugin_init`, `cliproxyPluginCall`, `cliproxyPluginFree`, `cliproxyPluginShutdown` (`cmd/cliproxyapi-copilot/main.go:3-60`, `73-119`). Every call in both directions is `(method string, JSON bytes) → JSON envelope` (`main.go:87-106`, `133-175`).
  - Build: `CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -buildvcs=false -trimpath -ldflags "-X main.pluginVersion=$(VERSION)" -buildmode=c-shared -o build/plugins/linux/amd64/cliproxyapi-copilot.so ./cmd/cliproxyapi-copilot` (`Makefile:23`, `build-local` at `Makefile:27`).
  - Host side **[CPA]**: `internal/pluginhost/loader_unix.go:43-44` `dlopen(path, RTLD_NOW | RTLD_LOCAL)`, `:120-125` `dlsym("cliproxy_plugin_init")`; the CPA tree has no import of the Go `"plugin"` package (grep: not found). Search dirs: `<plugins.dir>/<GOOS>/<GOARCH>` then `<plugins.dir>` (`docs/install-existing-deployment.md:28-29`; **[CPA]** `platform.go:126` `candidateDirs(root, runtime.GOOS, runtime.GOARCH)`), extension by OS (`platform.go:103-111`).
- **Version-lock rule**: because it is a C ABI with JSON payloads, the Go `plugin` package's "identical toolchain + identical dependency versions" rule does **not** apply. What must match is (a) `ABIVersion` (checked at `main.go:75`) and (b) the JSON schema (`SchemaVersion` sent at `dispatch.go:163`; host-side compatibility policy lives in **[CPA]** `internal/pluginhost/rpc_schema.go`, not read in depth). Both are `1` / `2` at v7.2.118. The plugin author nevertheless targets exactly one CPA release ("currently targets CLIProxyAPI `v7.2.118`, ABI version 1, on Linux `amd64`", `docs/install-existing-deployment.md:5-6`), and PR #1's author reports loading the HEAD plugin into CPA **7.2.140** successfully — evidence the C ABI tolerates minor host drift.
- **glibc / base image**: production build runs inside `golang:1.26-bookworm` (`Makefile:1`, `README.md:85-87`, `release.yml:39`), explicitly to match the official CPA image's Debian Bookworm runtime; the README warns a `build-local` binary "built on a newer host glibc may not load in the Bookworm container" (`README.md:100-101`). CGO is required (`CGO_ENABLED=1`, `Makefile:23`). CPA's own Dockerfile is also `golang:1.26-bookworm` → `debian:bookworm` with `CGO_ENABLED=1` (**[CPA]** `Dockerfile:1`, `17`, `19`). This matches the fleet's `cpa-home-build` container (`golang:1.26-bookworm`) — the same builder image the Ark already uses.
- **No Dockerfile in the plugin repo** (`find -iname '*dockerfile*'`: none). `docker-compose.yml:5` runs the stock `eceasy/cli-proxy-api:7.2.118` image and bind-mounts `./build/plugins:/CLIProxyAPI/plugins:ro` (`docker-compose.yml:16`), so the plugin never needs a custom CPA image — only a volume mount plus the `plugins:` config block.
- **Release packaging**: `scripts/package-release.sh` zips only `cliproxyapi-copilot.so` into `cliproxyapi-copilot_<ver>_linux_amd64.zip` + `checksums.txt` (`package-release.sh:25-57`); CI builds on every push (`ci.yml:26-42`), releases on `v*` tags (`release.yml:3-6`, `42-54`). Only **linux/amd64** is published at HEAD; PR #2 (open, unmerged) adds linux/arm64 and darwin/{amd64,arm64}.
- Go 1.26 is required for `make test` locally (`ci.yml:20`, `README.md:85-86`) — irrelevant for the fleet, which builds only on the Ark.

## 3h. Open issues, PRs, and README caveats

- **Issues: none, ever** (`gh issue list -R … --state all --limit 30` → `[]`).
- **PR #1** "fix: preserve OAuth model prefixes" (Goodwu, 2026-08-24, open): 2 files, +37/−0. Adds `Prefix` to `authStorage`, trims slashes/whitespace, and uses it in `authData` when the caller passed none. Root cause per author: "`ParseAuth` decoded the credential into `authStorage`, which did not include `prefix` … CLIProxyAPI therefore registered the discovered models without the configured credential namespace." Validated against CPA 7.2.140 on darwin/arm64.
- **PR #2** "构建: 支持 Linux 与 macOS 双架构发布" (Goodwu, 2026-08-24, open): CI/Makefile/docs for four platform packages; not needed for the Ark (linux/amd64).
- **README caveats worth carrying into the scoping report**:
  - "This is an initial MVP. Less common Responses event types, provider-specific reasoning signatures, citations/annotations, audio, computer-use blocks, and all document variants are not exhaustively verified." (`README.md:307-309`)
  - "A Go shared-library plugin is trusted, in-process code. Review and build this repository before mounting its artifact." (`README.md:285-286`)
  - "The Go dependency and image tag are version-pinned, but the Docker tag is not a digest pin." (`README.md:296-298`)
  - Default OAuth client id `Iv1.b507a08c87ecfe98` is "the public client identifier used by established Copilot device-flow clients … not a secret" (`README.md:62-64`) — i.e. the plugin impersonates a known VS Code / Copilot client at both the OAuth layer and the request-header layer (3e). That is the same approach as every third-party Copilot proxy, but it is worth stating plainly for the fleet decision.
  - Docker Hub tag naming mismatch (`v7.2.118` vs `7.2.118`) is a doc-only wrinkle (`README.md:26-30`).
  - Claude Code aliasing examples map `opus`→`gpt-5.6-sol`, `haiku`→`gpt-5.6-terra`, `sonnet`→`claude-opus-5` via the Claude subscription (`docs/claude-code-setup.md:256-293`) — illustrative only, driven by the author's excluded-`claude-` setup.

## Key takeaways for the ws-0002 scoping decision

1. Small (4.2k LOC), single-author, three-day project, dormant since 2026-08-07, zero issues, two unmerged PRs. Treat as a starting point to fork and own, not a maintained dependency.
2. Built on the official CPA C-ABI plugin contract (ABI 1 / schema 2 at v7.2.118), loaded by `dlopen`; no Go-toolchain lockstep requirement, but the builder image (`golang:1.26-bookworm`, CGO on) must match the CPA runtime's glibc — the Ark's existing `cpa-home-build` container already is that image.
3. Auth is host-stored JSON (`copilot-<login>.json`, type `copilot`, contains the GitHub OAuth access token + login/user-id), keyed per GitHub login so multiple accounts work; the short-lived Copilot token is memory-only and self-refreshing.
4. Models come through with **raw Copilot IDs** (`gpt-4.1`, `claude-sonnet-4`, …), `owned_by` = vendor. Nothing prefixes them. To keep Copilot's Claude/OpenAI models from being pooled with native credentials you must either (a) drop them with `excluded_model_prefixes`, or (b) use a credential `prefix` — which requires applying open PR #1 first.
5. Executor accepts only Responses and Claude Messages as input; streams via SSE; supports tools; sends fixed VS Code Chat headers with `X-Initiator: user` on every call; reports no usage/premium-request data to the host.
