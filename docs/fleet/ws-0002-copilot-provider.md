# ws-0002 — GitHub Copilot as a provider across the fleet

Workstream brief for the `fleet` fork of CLIProxyAPIHome. Created 2026-09-24.

- Branch: `ws-0002/copilot-provider` (based on `fleet` @ `9d8bf4e`, same base as ws-0001)
- Worktree: `.claude/worktrees/ws-0002-copilot-provider`
- herdr space: `CLIProxyAPIHome | ws-0002 | Copilot provider` (agent alias `cpahome-ws-0002`)
- Runs in parallel with ws-0001 (upstream sync). Rebases onto the synced `fleet` once ws-0001
  lands, before anything ships. Nothing here goes to upstream Home unless it is a clean fix.

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
9. Rebased onto the post-ws-0001 `fleet`; full suite green; rollout to the remaining nodes and any
   Home image only on a decided `ship` card. Never stop a mini's `cpa-home-node`; restart in place.

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
9. Rebase onto synced `fleet` (after ws-0001 closes), full tests, close-out here, ship card.

## Stop points

Before: installing anything on any node or the Ark; logging in any account; any `git push`;
any Home image build. Ask in plain text in this pane; no AskUserQuestion dialogs.

## Close-out

(filled in at the end)
