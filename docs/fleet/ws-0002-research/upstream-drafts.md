# ws-0002 — upstream submissions (drafts, not yet posted)

Both post publicly under Chanse's GitHub account on repos he does not own, so each waits for his
yes. Evidence for both is in `docs/fleet/ws-0002-copilot-provider.md` (Task 6 records).

## 1. PR to router-for-me/CLIProxyAPIHome `dev` — branch `fix/plugin-auth-identity` (`c02cb14`)

**Title:** fix(plugin-oauth): give plugin-created auths a UUID identity before saving

**Body:**

Every login made through a plugin `AuthProvider` fails to save on Home. The plugin returns
`AuthData` with its own identifier (for example `copilot-octocat.json`) and no index, and
`AuthToRecord` requires `ID == Index`, a UUID:

```
cluster plugin oauth: save auth failed provider=copilot error=auth index is required
```

The panel shows "Failed to save authentication tokens" and nothing is stored. Native OAuth logins
do not hit this because `EnsureOAuthPayloadUUID` gives them a UUID; the plugin path has no
equivalent. Reproduced on 1.0.73 with a GitHub Copilot provider plugin; the same save path
(`CompleteOAuthSessionWithAuths` → `UpsertAuth` → `AuthToRecord`) is unchanged on `dev`.

**Change:** `cluster.EnsurePluginAuthIdentity` derives a deterministic UUID from the provider and
the plugin identifier (same account again → same record; different account or provider → its
own), keeps the plugin identifier as `FileName`, and leaves an existing UUID alone. The plugin
login path calls it for each auth before `CompleteOAuthSessionWithAuths`.

**Tests:** 4 new tests in `internal/cluster/plugin_auth_identity_test.go` (UUID assigned and
storable via `AuthToRecord`, stable across re-login, distinct per account and per provider,
existing UUID kept, `FileName` fallback, empty auth untouched). Full suite on `dev` + this
change: see the Ark run recorded below.

## 2. Issue on router-for-me/CLIProxyAPI — Home-managed node loops forever when Home adds a plugin

**Title:** Home-managed node loops "installed but not loaded" forever when Home adds a new plugin at runtime

**Body:**

On a node attached to CLIProxyAPI Home, adding a plugin through Home's config (a `store`
manifest under `plugins.configs.<id>`) while the node is running puts the node into an endless
retry loop:

```
[warn] [service_home.go:680] failed to stage home config; retrying error=load home plugins: home plugins: plugin <id> installed but not loaded
```

several times a second, forever. The node keeps serving on its previous config but can no longer
apply any config from Home. Seen on 7.3.16 (`c404af96`) on five macOS arm64 nodes; the code is the
same on v8.0.4.

Cause: in `stageHomeOverlayWithClient` (`sdk/cliproxy/service_home.go`), the plugin is downloaded
and installed, then `homeplugins.MarkLoadResults(&report, s.pluginHost)` checks
`pluginHost.PluginRegistered(id)` **before** the staged config is applied
(`applyConfigUpdateWithAuthSynthesis` runs later, when the work is committed). The plugin host only
loads plugins when a config is applied, so a newly added plugin can never be registered at that
point and staging always fails. `runHomeConfigWorkerWithSupervisor` then retries the same payload
and never dequeues a newer one, so removing the plugin from Home's config does not recover the
node either.

The startup path is ordered correctly (`cmd/server/main.go`: `pluginHost.ApplyConfig` then
`MarkLoadResults`), so restarting the node recovers it and the plugin loads. Workaround used: a
restart in place of every node after installing a plugin through Home.

Suggested fix: on the hot path, check load results after the config (and so the plugin host) has
been applied, or have the sync step ask the plugin host to load newly installed plugins before
`MarkLoadResults`.
