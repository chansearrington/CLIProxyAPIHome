# GitHub Copilot API model catalogue — as of 2026-09-24

Research for ws-0002 (Copilot as a CPA/Home provider). Web research only; nothing was logged into, so no live `/models` call was made. Every claim carries a source; anything not directly confirmed is marked **UNVERIFIED**.

Headline facts a Home/CPA integrator needs:

- **"Premium requests" are dead for everyone except legacy annual Pro/Pro+ subscribers.** Since 2026-06-01 every plan bills in **GitHub AI Credits** (1 credit = $0.01), charged per token at a published per-model rate. Per-model *multipliers* now only exist on the legacy annual plans (Sources S9, S10, S11, S12).
- The catalogue is ~32 models across Anthropic, OpenAI (served as vendor "Azure OpenAI"), Google, xAI, Moonshot AI and Microsoft. Two retirement waves are in flight: six ids died 2026-09-01, four more die **2026-10-02** (S5, S6).
- Copilot serves Claude on **two** paths: OpenAI-style `/chat/completions` (thinking silently stripped, `reasoning_effort` rejected) and a native Anthropic-style **`/v1/messages`** (thinking, `cache_control`, streaming thinking blocks all work). GPT models additionally get `/responses`. The `/models` entry says which via `supported_endpoints` (S15, S16, S17).
- Third-party context caps are real: `max_prompt_tokens` = 128k on the Claude ids as of Feb 2026 even where native is 200k–1M; 1M-context variants are documented as VS Code + Copilot CLI only (S14, S18, S1).

---

## 1. Current model ids in the `/models` response

**Where the exact strings come from.** No GitHub page prints the API ids; GitHub's docs use display names. The ids below are the [models.dev `github-copilot` provider](https://models.dev/api.json) (S3), whose entries are generated against `https://api.githubcopilot.com/models` (the `gemini-3.8-flash.toml` file cites that URL directly) and whose last three commits were 2026-09-23 (`claude-opus-5.5`, `gpt-6-sol`, `gpt-6-luna`). Naming convention matches the verbatim Dec-2025 dump in S13 (`gpt-5-mini`, `gpt-5`, `claude-sonnet-4.5`, …): lowercase, vendor family, dotted version, hyphen-joined suffix. Display names are cross-checked against GitHub Docs "Supported AI models" (S1, fetched 2026-09-24, page carries no date).

Plan columns come from GitHub Docs "Plans for GitHub Copilot" (S8). Free and Student get **Auto model selection only** (no named model). "Deprecation" = GitHub changelog dates (S5, S6).

| Copilot id (`/models` `id`) | Display name | Vendor | Pro | Pro+ | Max | Business / Enterprise | Notes |
|---|---|---|---|---|---|---|---|
| `claude-haiku-4.5` | Claude Haiku 4.5 | Anthropic | ✓ | ✓ | ✓ | ✓ | |
| `claude-sonnet-4.6` | Claude Sonnet 4.6 | Anthropic | ✓ | ✓ | ✗ | ✓ (policy) | **Deprecated 2026-09-01** everywhere except individual *annual* plans (S5). models.dev still lists it. |
| `claude-sonnet-5` | Claude Sonnet 5 | Anthropic | ✓ | ✓ | ✓ | ✓ | Replacement for Sonnet 4.5/4.6 |
| `claude-opus-4.7` | Claude Opus 4.7 | Anthropic | ✗ | ✓ | ✓ | ✓ | **Retires 2026-10-02** → Opus 5 (S6) |
| `claude-opus-4.8` | Claude Opus 4.8 | Anthropic | ✗ | ✓ | ✓ | ✓ | |
| *(id UNVERIFIED — likely `claude-opus-4.8-fast` by analogy with LiteLLM's `claude-opus-4.6-fast`)* | Claude Opus 4.8 (fast mode) (preview) | Anthropic | ✗ | ✓ | ✓ | ✓ | Preview; not in models.dev; priced 2× Opus 4.8 (S12) |
| `claude-opus-5` | Claude Opus 5 | Anthropic | ✗ | ✓ | ✓ | ✓ | |
| `claude-opus-5.5` | Claude Opus 5.5 | Anthropic | ✗ | ✓ | ✓ | ✓ | Added 2026-09-22 (S3 commit; changelog cited in TOML) |
| `claude-fable-5` | Claude Fable 5 | Anthropic | ✗ | ✓ | ✓ | ✓ **admin must enable**; retention terms (S1) | |
| `claude-fable-5.1` | Claude Fable 5.1 | Anthropic | ✗ | ✓ | ✓ | ✓ admin must enable | |
| `gpt-5-mini` | GPT-5 mini | Azure OpenAI | ✓ | ✓ | ✓ | ✓ | LTS-style cheap model; `is_chat_fallback` candidate |
| `gpt-5.3-codex` | GPT-5.3-Codex | Azure OpenAI | ✓ | ✓ | ✓ | ✓ | GitHub's "LTS fallback" model (S1) |
| `gpt-5.4` | GPT-5.4 | Azure OpenAI | ✓ | ✓ | ✓ | ✓ | |
| `gpt-5.4-mini` | GPT-5.4 mini | Azure OpenAI | ✓ | ✓ | ✓ | ✓ | |
| `gpt-5.4-nano` | GPT-5.4 nano | Azure OpenAI | ✗ | ✓ | ✓ | ✓ | |
| `gpt-5.5` | GPT-5.5 | Azure OpenAI | ✗ | ✓ | ✓ | ✓ | |
| `gpt-5.6-luna` | GPT-5.6 Luna | Azure OpenAI | ✓ | ✓ | ✓ | ✓ | |
| `gpt-5.6-terra` | GPT-5.6 Terra | Azure OpenAI | ✓ | ✓ | ✓ | ✓ | |
| `gpt-5.6-sol` | GPT-5.6 Sol | Azure OpenAI | ✗ | ✓ | ✓ | ✓ | |
| `gpt-6-astra` | GPT-6 Astra | Azure OpenAI | ✗ | ✓ | ✓ | ✓ | Added 2026-09-04 |
| `gpt-6-luna` | GPT-6 Luna | Azure OpenAI | ✓ | ✓ | ✓ | ✓ | Added 2026-09-22; OpenAI model card "coming soon" |
| `gpt-6-sol` | GPT-6 Sol | Azure OpenAI | ✗ | ✓ | ✓ | ✓ | Added 2026-09-22 |
| `gemini-3.5-flash` | Gemini 3.5 Flash | Google | ✓ | ✓ | ✓ | ✓ | **Retires 2026-10-02** → 3.8 Flash |
| `gemini-3.6-flash` | Gemini 3.6 Flash | Google | ✓ | ✓ | ✓ | ✓ | **Retires 2026-10-02** → 3.8 Flash |
| `gemini-3.7-flash` | Gemini 3.7 Flash | Google | ✓ | ✓ | ✓ | ✓ | |
| `gemini-3.8-flash` | Gemini 3.8 Flash | Google | ✓ | ✓ | ✓ | ✓ | Added 2026-09-02, gradual rollout |
| `grok-4.5` | Grok 4.5 | xAI | ✓ | ✓ | ✓ | ✓ | |
| `grok-4.6` | Grok 4.6 | xAI | ✓ | ✓ | ✓ | ✓ | |
| `grok-4.7` | Grok 4.7 | xAI | ✓ | ✓ | ✓ | ✓ | Added 2026-09-21 |
| `kimi-k2.7-code` | Kimi K2.7 Code | Moonshot AI | ✓ | ✓ | ✓ | ✓ (open-weight → **disabled by default** under org policy, S24) | **Retires 2026-10-02** → K3 |
| `kimi-k3` | Kimi K3 | Moonshot AI | ✓ | ✓ | ✓ | ✓ (disabled by default) | Individual plans may get a *fine-tuned* variant, not Business/Enterprise (S2) |
| `mai-code-1.1-flash` | MAI-Code-1.1-Flash | Microsoft | ✓ | ✓ | ✓ | ✓ | "Continuously updated checkpoints" (S2) |
| `mai-code-1-flash-picker` | MAI-Code-1-Flash | Microsoft | — | — | — | — | Retired 2026-09-10 (S6 coverage); still in models.dev |

Ids retired 2026-09-01 that older tooling still lists (do **not** map these): `claude-opus-4.5`, `claude-opus-4.6`, `claude-sonnet-4.5`, `gemini-3.1-pro` (`gemini-3.1-pro-preview` in some dumps), `raptor-mini`; earlier (2026-02-17) wave removed the GPT-4.x/4o/5.x-pre-5.3 ids that LiteLLM's Dec-2025 table still carries (`gpt-4.1`, `gpt-4o`, `gpt-5`, `gpt-5.1`, `gpt-5.2`, `claude-sonnet-4`, `claude-opus-41`, …) (S5, S7, S13).

**1M-context variant ids.** As of 2026-06-05 the API exposed suffixed ids `claude-opus-4.6-1m`, `claude-sonnet-4.6-1m` (GA) and `claude-opus-4.7-1m-internal` (first-party only) (S18). Whether `claude-sonnet-5`, `claude-opus-5`, `claude-opus-5.5`, `claude-fable-5.x` and the GPT ids have `-1m` siblings today, or expose 1M on the base id, is **UNVERIFIED** — GitHub Docs (S1) say the 1M window "is limited to VS Code and Copilot CLI", and models.dev's 1,000,000 figures for those ids are copied from GitHub Docs, not from an API dump.

Also on GitHub's comparison page but **not** in the API list from any source: "Qwen2.5" (Alibaba) — probably a completions-only or stale entry; **UNVERIFIED** (S2).

---

## 2. Copilot id → native vendor id

Sources: models.dev `base_model` fields (S3), Anthropic model-id docs (S19), OpenAI model pages (S20), Google Gemini docs (S21), xAI docs (S22). Confidence: **firm** = vendor docs confirm the native id and the Copilot display name is unambiguous; **likely** = same name, vendor id inferred from naming convention; **UNVERIFIED** = no native equivalent found.

| Copilot id | Native vendor id | Native API | Confidence / caveat |
|---|---|---|---|
| `claude-haiku-4.5` | `claude-haiku-4-5-20251001` (alias `claude-haiku-4-5`) | Anthropic | firm |
| `claude-sonnet-4.6` | `claude-sonnet-4-6` | Anthropic | firm (dateless id is canonical from 4.6 on, S19) |
| `claude-sonnet-5` | `claude-sonnet-5` | Anthropic | firm |
| `claude-opus-4.7` | `claude-opus-4-7` | Anthropic | firm |
| `claude-opus-4.8` | `claude-opus-4-8` | Anthropic | firm |
| Opus 4.8 fast mode | `claude-opus-4-8` + Anthropic "fast mode" (speed option, 2× price) | Anthropic | **UNVERIFIED** how Copilot exposes it; native fast mode is a request-level option, not a separate id |
| `claude-opus-5` | `claude-opus-5` | Anthropic | firm |
| `claude-opus-5.5` | `claude-opus-5-5` | Anthropic | firm |
| `claude-fable-5` | `claude-fable-5` | Anthropic | firm |
| `claude-fable-5.1` | `claude-fable-5-1` | Anthropic | firm |
| `gpt-5-mini` | `gpt-5-mini` | OpenAI (Copilot vendor string: "Azure OpenAI") | firm |
| `gpt-5.3-codex` | `gpt-5.3-codex` | OpenAI | firm |
| `gpt-5.4` / `gpt-5.4-mini` / `gpt-5.4-nano` | same | OpenAI | firm |
| `gpt-5.5` | `gpt-5.5` | OpenAI | firm |
| `gpt-5.6-luna` / `-terra` / `-sol` | `gpt-5.6-luna` / `gpt-5.6-terra` / `gpt-5.6-sol` (`gpt-5.6` alias → Sol) | OpenAI | firm (S20) |
| `gpt-6-astra` / `gpt-6-luna` / `gpt-6-sol` | `gpt-6-astra` / `gpt-6-luna` / `gpt-6-sol` | OpenAI | firm; note OpenAI docs list GPT-6 Luna/Sol cards as "coming soon" while Copilot already lists them |
| `gemini-3.5-flash` … `gemini-3.8-flash` | `gemini-3.5-flash`, `gemini-3.6-flash`, `gemini-3.7-flash`, `gemini-3.8-flash` | Google Gemini API | firm for 3.8 (S21); likely for 3.5–3.7 |
| `grok-4.5` / `grok-4.6` / `grok-4.7` | `grok-4.5` / `grok-4.6` / `grok-4.7` | xAI | firm for 4.7 (S22); likely for 4.5/4.6 |
| `kimi-k2.7-code` | `kimi-k2.7-code` | Moonshot | likely; Copilot may serve a **fine-tuned** variant on individual plans (S2) → not weight-identical |
| `kimi-k3` | `kimi-k3` | Moonshot | likely; same fine-tune caveat |
| `mai-code-1.1-flash` | — | Microsoft MAI, no public native API found | **UNVERIFIED** |

Important nuance: GPT ids are served through **Azure OpenAI**, not OpenAI directly (the `vendor` field says so, S13). Parameter support (e.g. `reasoning.effort` values, Responses-API features) can lag OpenAI's own endpoint; treat OpenAI-doc feature claims as upper bounds.

---

## 3. Billing: AI credits (current) and premium-request multipliers (legacy only)

### 3a. What every plan gets today (usage-based billing, live since 2026-06-01)

Source: GitHub Docs "Usage-based billing for individuals" (S9), "…for organizations and enterprises" (S10), "Plans for GitHub Copilot" (S8), changelog 2026-06-01 (S11). None of these pages shows a last-updated date; all fetched 2026-09-24.

| Plan | Price | Included AI credits / month | Model access | When exhausted |
|---|---|---|---|---|
| Free | $0 | "an allowance" — **amount not published**; 2,000 completions/month | Auto model selection only | Upgrade or wait for reset (S8, S9) |
| Student | $0 (verified) | not published; unlimited completions | Auto only; no third-party agents | same |
| Pro | $10 | 1,000 base + 500 flex = **1,500** ($15) | "a selection of models" (see §1 column) | Upgrade (pay difference), or set a USD budget for *additional usage* ($10 = 1,000 credits, billed month-end) — GitHub "may cap" additional usage by usage pattern / billing history / verification; or wait. Mobile-store subscribers **cannot** buy extra (hard stop). |
| Pro+ | $39 | 3,900 + 3,100 = **7,000** | all models | same |
| Max | $100 | 10,000 + 10,000 = **20,000** | all except Sonnet 4.6; "priority access" | same, higher caps |
| Business | $19 / seat | **1,900 per user**, **pooled** at billing entity (100 seats → 190,000) | premium models, subject to org model policy | Governed by "AI credits paid usage" policy — **overage on by default**; admin can disable → **hard stop** until the 1st. User-level budgets can block an individual sooner ($0 budget = immediate block). No auto-fallback to a cheaper model. |
| Enterprise | $39 / seat | **3,900 per user**, pooled | premium models, priority | same |

Common rules: 1 credit = $0.01; cost = model rate × tokens (input, output, cached, and cache-write where the model has it); resets 00:00:00 UTC on the 1st, **no rollover**; "base" credits never change, "flex" may be adjusted; Auto model selection = **10% discount** on paid plans; completions and next-edit-suggestions are free; Copilot code review also burns Actions minutes. Existing Business/Enterprise customers had a promotional higher allotment 2026-06-01 → 2026-09-01 (S11 coverage; exact figures **UNVERIFIED**). Business/Enterprise seats become **pre-paid per seat from 2026-10-01**; revoking a seat gives no prorated refund (S23).

Discrepancy to note: the April 2026 announcement said Pro would include "$10 of credits"; the live docs say 1,500 credits ($15). The docs are authoritative today.

### 3b. Per-model token rates (what a credit buys), per 1M tokens, USD

Source: GitHub Docs "Models and pricing for GitHub Copilot" (S12), fetched 2026-09-24. Long-context tier applies to the whole request once input exceeds the threshold.

| Copilot model | Input | Cached read | Cache write | Output | Long-context tier |
|---|---|---|---|---|---|
| Claude Haiku 4.5 | 1.00 | 0.10 | 1.25 | 5.00 | — |
| Claude Sonnet 4.6 | 3.00 | 0.30 | 3.75 | 15.00 | — |
| Claude Sonnet 5 | 2.00 | 0.20 | 2.50 | 10.00 | — |
| Claude Opus 4.7 / 4.8 / 5 | 5.00 | 0.50 | 6.25 | 25.00 | — |
| Claude Opus 5.5 | 4.00 | 0.20 | 5.00 | 20.00 | — |
| Claude Opus 4.8 (fast mode) preview | 10.00 | 1.00 | 12.50 | 50.00 | — |
| Claude Fable 5 | 10.00 | 1.00 | 12.50 | 50.00 | — |
| Claude Fable 5.1 | 10.00 | 0.25 | 12.50 | 50.00 | — |
| GPT-5 mini | 0.25 | 0.025 | — | 2.00 | — |
| GPT-5.3-Codex | 1.75 | 0.175 | — | 14.00 | — |
| GPT-5.4 | 2.50 | 0.25 | — | 15.00 | >272K: 5.00 / 0.50 / 22.50 |
| GPT-5.4 mini | 0.75 | 0.075 | — | 4.50 | — |
| GPT-5.4 nano | 0.20 | 0.02 | — | 1.25 | — |
| GPT-5.5 | 5.00 | 0.50 | — | 30.00 | >272K: 10 / 1 / 45 |
| GPT-5.6 Luna | 0.20 | 0.02 | 0.25 | 1.20 | >200K: 0.40 / 0.04 / 0.50 / 1.80 |
| GPT-5.6 Terra | 2.00 | 0.20 | 2.50 | 12.00 | >272K: 4 / 0.4 / 5 / 18 |
| GPT-5.6 Sol | 4.00 | 0.40 | 5.00 | 20.00 | >272K: 8 / 0.8 / 10 / 30 |
| GPT-6 Astra | 10.00 | 1.00 | 12.50 | 50.00 | >272K: 20 / 2 / 25 / 75 |
| GPT-6 Luna | 0.10 | 0.01 | 0.125 | 0.50 | >272K: 0.2 / 0.02 / 0.25 / 0.75 |
| GPT-6 Sol | 2.00 | 0.20 | 2.50 | 10.00 | >272K: 4 / 0.4 / 5 / 15 |
| Gemini 3.5 Flash | 1.50 | 0.15 | — | 9.00 | — |
| Gemini 3.6 / 3.7 / 3.8 Flash | 0.75 | 0.075 | — | 3.75 | promo through 2026-12-31 |
| MAI-Code-1.1-Flash | 0.20 | 0.02 | — | 1.20 | — |
| Grok 4.5 / 4.6 / 4.7 | 2.00 | 0.50 | — | 6.00 | >200K: 4 / 1 / 12 |
| Kimi K2.7 Code | 0.95 | 0.19 | — | 4.00 | — |
| Kimi K3 | 3.00 | 0.30 | — | 15.00 | — |

These equal the vendors' own list prices (e.g. Anthropic Opus 5.5 $4/$20, Sonnet 5 $2/$10, Fable 5.1 $10/$50; OpenAI GPT-5.6 Sol promo $4/$20; xAI Grok 4.7 $2/$6) — Copilot adds no per-token markup; the value is the included allowance (S12, S19, S20, S22).

### 3c. Legacy premium-request multipliers (annual Pro/Pro+ only)

Source: GitHub Docs "Model multipliers for annual plans on request-based billing (legacy)" (S7) and "Requests in GitHub Copilot (legacy)" (S4). Applies **only** to Pro/Pro+ subscribers still on an annual plan after 2026-06-01; those users "will not receive access to new models and features". Allowance: Pro **300**/month, Pro+ **1,500**/month, extra requests $0.04 each (budget-gated; unavailable to iOS/Android-store subscribers = hard stop). Free/Business/Enterprise are not on this scheme at all any more.

| Model | Multiplier | | Model | Multiplier |
|---|---|---|---|---|
| Claude Haiku 4.5 | 0.33 | | GPT-5.1 / 5.1-Codex / 5.1-Codex-Max | 3 |
| Claude Sonnet 4.6 | 9 | | GPT-5.1-Codex-Mini | 0.33 |
| Claude Opus 4.7 / 4.8 | 27 | | GPT-5.3-Codex / GPT-5.4 / GPT-5.4 mini | 6 |
| Gemini 3 Pro | 6 | | GPT-5.5 | 57 |
| Gemini 3.5 Flash | 14 | | GPT-5 mini / GPT-4o / GPT-4o mini | 0.33 |
| MAI-Code-1.1-Flash | 0.25 | | Copilot code review | 13 per review |

Auto selection = 10% off the multiplier. CLI: 1 request per prompt × multiplier (tool calls free); cloud agent: 1 per session + 1 per steering comment.

### 3d. Billing fields in `/models`

The Dec-2025 dump (S13) carries `"billing": {"is_premium": true, "multiplier": 1, "restricted_to": ["pro","pro_plus","max","business","enterprise"]}` per model. Whether `multiplier` is still populated post-2026-06-01 or has been replaced by token-rate fields is **UNVERIFIED** (no post-June dump found). `restricted_to` is the machine-readable plan gate and is worth reading at runtime.

---

## 4. Shape of a `/models` entry

Verbatim object from `GET https://api.enterprise.githubcopilot.com/models`, quoted in LiteLLM PR #17858 (opened 2025-12-12, merged 2025-12-16) (S13). The model (`gpt-5`) has since been retired, but the schema is what every third-party client parses today.

```json
{
  "billing": {
    "is_premium": true,
    "multiplier": 1,
    "restricted_to": ["pro", "pro_plus", "max", "business", "enterprise"]
  },
  "capabilities": {
    "family": "gpt-5",
    "limits": {
      "max_context_window_tokens": 400000,
      "max_output_tokens": 128000,
      "max_prompt_tokens": 128000,
      "vision": {
        "max_prompt_image_size": 3145728,
        "max_prompt_images": 1,
        "supported_media_types": ["image/jpeg", "image/png", "image/webp", "image/gif"]
      }
    },
    "object": "model_capabilities",
    "supports": {
      "parallel_tool_calls": true,
      "streaming": true,
      "structured_outputs": true,
      "tool_calls": true,
      "vision": true
    },
    "tokenizer": "o200k_base",
    "type": "chat"
  },
  "id": "gpt-5",
  "is_chat_default": false,
  "is_chat_fallback": false,
  "model_picker_category": "versatile",
  "model_picker_enabled": true,
  "name": "GPT-5",
  "object": "model",
  "policy": {
    "state": "enabled",
    "terms": "Enable access to the latest GPT-5 model from OpenAI. [Learn more about how GitHub Copilot serves GPT-5](https://gh.io/copilot-openai)."
  },
  "preview": false,
  "supported_endpoints": ["/chat/completions", "/responses"],
  "vendor": "Azure OpenAI",
  "version": "gpt-5"
}
```

Fields confirmed by the same dump for a non-premium model: `"billing": {"is_premium": false, "multiplier": 0}` and no `restricted_to` (that was `gpt-5-mini`, `model_picker_category: "lightweight"`).

Additional fields seen in 2026 sources:

| Field | Seen where | Meaning |
|---|---|---|
| `capabilities.limits.max_non_streaming_output_tokens` (e.g. 16000 on `claude-opus-4.6`) | models.dev #858, Feb 2026 (S14) | Output cap when `stream:false` |
| `capabilities.supports.reasoning_effort` (array, e.g. `["low","medium","high","xhigh"]`) | models.dev #2021, Jun 2026 (S18) | Accepted `reasoning_effort` values; absent = model rejects the parameter |
| `supported_endpoints` containing `"/v1/messages"` for Claude ids | hermes-agent PR, LiteLLM #28053 (S16, S15) | Native Anthropic path available |
| `custom_model: {key_name, owner_name, owner_type, provider}` | copilot-cli #4953 (S13 search) | Enterprise BYOK custom models; id like `myorg/OpenRouter/deepseek/...` |
| `policy.state` values `enabled` / `disabled`; a `preview` model may be listed yet reject requests | LobeHub skill notes (S13 search) | Org/user policy gate |

Claude limits as reported by the endpoint in Feb 2026 (S14) — note the gap versus native:

| id | max_context_window_tokens | max_prompt_tokens | max_output_tokens | max_non_streaming_output_tokens |
|---|---|---|---|---|
| `claude-opus-4.6` | 144,000 | 128,000 | 64,000 | 16,000 |
| `claude-sonnet-4.5` | 144,000 | 128,000 | 32,000 | — |
| `claude-opus-4.5` | 160,000 | 128,000 | 32,000 | — |
| `claude-sonnet-4` | 216,000 | 128,000 | 16,000 | — |
| `claude-haiku-4.5` | 144,000 | 128,000 | 32,000 | — |

Current values for the Sonnet 5 / Opus 5.x / Fable ids: **UNVERIFIED** (no public dump after June 2026). The community discussion in S13 confirms the same pattern for GPT: `max_context_window_tokens` 400K but `max_prompt_tokens` 128K, so "the actual usable capacity is limited by max_prompt".

How third parties call it (S13, S14, S18):

```
curl -s https://api.githubcopilot.com/models \
  -H "Authorization: Bearer $COPILOT_TOKEN" \
  -H "Copilot-Integration-Id: vscode-chat" \
  -H "Editor-Version: vscode/1.96.0" \
  -H "Editor-Plugin-Version: copilot-chat/0.35.0" \
  -H "User-Agent: GitHubCopilotChat/0.35.0" \
  -H "X-GitHub-Api-Version: 2025-10-01"
```

`$COPILOT_TOKEN` is not the GitHub OAuth token: the `ghu_` token from the VS Code GitHub-App device flow (client id `Iv1.b507a08c87ecfe98`) is exchanged at `GET api.github.com/copilot_internal/v2/token` (`Authorization: token ghu_…`) for a short-lived (~30 min) HMAC-signed bearer; that response also carries `endpoints.api` (the correct host — `api.individual.githubcopilot.com`, `api.business.githubcopilot.com`, `api.enterprise.githubcopilot.com`) and the SKU. A `gho_` token from a plain OAuth App gets 404 from the exchange (S25, S26).

---

## 5. Behavioural differences: Copilot vs the native vendor API

| Dimension | Via Copilot | Native | Source |
|---|---|---|---|
| **Wire protocol** | All models: OpenAI-compatible `POST /chat/completions`. GPT ids also `/responses`. Claude ids also **`POST /v1/messages`** (+ `/v1/messages/count_tokens`), header `anthropic-version: 2023-06-01`, auth is `Authorization: Bearer` **not** `x-api-key`. Which paths a model has = `supported_endpoints`. Gemini/Grok/Kimi: chat/completions only (no native Gemini `generateContent` path found). | Vendor-native | S15, S16, S17, S13 |
| **Context window** | Third-party clients: `max_prompt_tokens` = **128K** on every Claude id as of Feb 2026 (context 144–216K incl. output). GPT: 128K prompt / 400K window. **1M context** exists (`-1m` ids) but GitHub Docs say it is limited to VS Code and Copilot CLI; `claude-opus-4.7-1m-internal` is first-party-only by name. Whether the Sonnet 5 / Opus 5.x / Fable ids give 1M to a third-party bearer: **UNVERIFIED**. | Claude 200K, or 1M on Sonnet 5 / Opus 5 / Fable; GPT-5.4+ ~1M; Gemini 3.x 1M; Grok 500K | S14, S18, S1, S19 |
| **Max output** | Claude 32K–64K (older ids 16K); `max_non_streaming_output_tokens` 16K on Claude — long answers need `stream:true`. GPT 128K. | Claude 64K–128K; GPT 128K | S14, S13 |
| **Tool calling** | Yes: `supports.tool_calls`, `parallel_tool_calls`, `structured_outputs` in the catalogue. OpenAI `tools`/`tool_choice` format on chat/completions; Anthropic `tools`/`tool_use` blocks on `/v1/messages`. | same | S13 |
| **Extended thinking / reasoning effort** | On `/chat/completions`: `reasoning_effort` → HTTP 400 `{"code":"invalid_reasoning_effort","message":"model claude-haiku-4.5 does not support reasoning effort"}` for models without `supports.reasoning_effort`; Anthropic-style `thinking` param is **silently ignored** and thinking blocks are stripped from the reply. On `/v1/messages`: `thinking: {type:"enabled", budget_tokens:N}` works, response has `{"type":"thinking"}` blocks and SSE `content_block_start/delta` thinking events per Anthropic spec. GitHub Docs: configurable reasoning levels exist for Claude Sonnet 4.6+, Opus 4.7+, Fable, GPT-5.3-Codex+, Kimi K3, "in VS Code, CLI and cloud agent". | Full | S15, S16, S1 |
| **Prompt caching** | `/chat/completions`: Copilot **auto-caches** and reports hits in `usage.prompt_tokens_details.cached_tokens`; no cache-write count; VS Code sends a proprietary `copilot_cache_control: {type:"ephemeral"}` message-level field. `/v1/messages`: **no caching unless** you send explicit `cache_control` markers; then `cache_creation_input_tokens` / `cache_read_input_tokens` are returned. Cache lifetime observed ≈ 5 min (VS Code cost-spike issue). Billed at the cached rates in §3b. | Anthropic explicit `cache_control` (5 min / 1 h); OpenAI automatic | S27, S28 |
| **Streaming** | `supports.streaming: true` on all chat models; required for outputs > `max_non_streaming_output_tokens`. | same | S13, S14 |
| **Vision** | `supports.vision` + `limits.vision`: **max 1 image**, 3,145,728 bytes, jpeg/png/webp/gif (Dec 2025 values). GitHub Docs call out vision only for GPT-5 mini, Sonnet 4.6, MAI-Code-1.1-Flash. | Anthropic: many images; Gemini: many + video/audio | S13, S2 |
| **Embeddings** | `text-embedding-3-small` etc. existed in the 2025 catalogue; current presence **UNVERIFIED**. | — | S13 (LiteLLM table) |
| **Rate limits** | No numbers published. Observed 429 bodies: `user_global_rate_limited:pro`, `…:pro_plus` with "exceeded your 5 hour session limits", and `user_weekly_rate_limited` ("switch to auto model to continue"). Docs describe global/weekly service limits plus per-model-family capacity limits by plan; advice is wait, use Auto, or change model. Limits are per user. | Vendor RPM/TPM tiers | S29, S30 |
| **Model availability gating** | `policy.state` per model. Individuals accept `policy.terms` in their Copilot settings. Business/Enterprise: **global model policy GA 2026-08-26** — unconfigured GA models inherit the org default (on by default); open-weight models (Kimi) and retention-requiring models (Fable 5/5.1) are **off by default** and need an explicit admin enable; enterprise settings override org. Each retirement wave tells admins to enable the replacement. | none | S24, S5, S6, S1 |
| **Model identity** | GPT via Azure OpenAI; Kimi may be a GitHub fine-tune on individual plans; MAI-Code "continuously updated checkpoints" — not a pinned snapshot. Claude ids are dateless (e.g. `claude-sonnet-5`), which natively are pinned releases. | — | S13, S2, S19 |
| **Data handling** | Fable 5/5.1: Anthropic retains prompts/outputs for safety classifiers by default; ZDR exemption on request through end-2026, then EFS required; ZDR users may not expose "the model endpoints or outputs" externally. Kimi K3 has extra GitHub safeguards. | Vendor ZDR terms | S1, S2 |
| **Client identity** | Server keeps a **per-client-ID model allowlist**: the VS Code GitHub-App id gets the full set; OpenCode's own OAuth-App id got a different set (Apr 2026 issue). Requests without matching `Copilot-Integration-Id` / `Editor-Version` / `Editor-Plugin-Version` / `User-Agent` are rejected (`400 model not supported` or `403`). | n/a | S25, S26 |
| **Legacy annual plans** | Never receive new models. | — | S7 |

### 5a. GitHub's stance on third-party clients

- **Officially supported third-party route #1 — OpenCode** (changelog 2026-01-16, S31): Pro/Pro+/Business/Enterprise users "can now authenticate into OpenCode using their Copilot credentials — no additional AI license needed", via `/connect` + GitHub device flow. It says nothing about other clients, proxies, terms or billing. A community question about whether OpenCode gets the same IP indemnity as first-party clients (2026-01-26) has **no GitHub answer** (S32).
- **Officially supported route #2 — Copilot SDK** (GA 2026-06-02, S33): embeds the Copilot agent runtime (bundles the Copilot CLI) with GitHub OAuth / GitHub-App / env-token auth, in Node, Python, Go, .NET, Rust, Java. This is the sanctioned programmatic path; it is an *agent* runtime, not a raw completions endpoint.
- **Reverse-engineered proxies** (copilot-api, copilot2api, copilot-proxy-api, CLIProxyAPI community plugin, OpenClaw native provider) all present the VS Code client id and headers. Their own READMEs warn: "reverse-engineered … not supported by GitHub, may break unexpectedly", and quote a "GitHub Security Notice" that heavy automated/bulk requests "may trigger GitHub's abuse-detection systems … could result in temporary suspension of your Copilot access", citing the AUP ban on "excessive automated bulk activity" (S34, S35).
- **No 2026 GitHub announcement blocking unofficial clients was found.** Nearest signals: (a) OpenClaw's Aug-2026 regression where every request returned `403 unauthorized: not authorized to use this Copilot feature` (cause not attributed by anyone, S36); (b) pi's report of the token-exchange host being blocked in a corporate network (network, not GitHub); (c) GitHub paused Copilot sign-ups 2026-04-20 → 06-17 (individual) / 09-03 (Business/Enterprise) citing agentic load, and added "stronger account vetting" and per-seat prepayment for orgs (S37, S23). **UNVERIFIED** whether GitHub treats unofficial clients as a terms breach per se.
- **Terms**: the Copilot Product Specific Terms were deprecated 2026-03-05; volume-licence customers are under the "GitHub Generative AI Services Terms" (March 2026), individuals under the GitHub ToS. Neither document contains a reverse-engineering, "authorized clients only" or rate-circumvention clause; use is "subject to the Acceptable Use Policies, the AI Code of Conduct and the Required Mitigations", and the ToS lets GitHub suspend accounts for API abuse (S38, S39, S40). The AUP's "excessive automated bulk activity" language is the operative restriction.

---

## 6. Multiple Copilot subscriptions and per-account tokens

**One GitHub account cannot hold two Copilot subscriptions.**
- Pro / Pro+ / Max are tiers of one personal plan; "upgrading" replaces it (you pay the difference, usage carries over) (S9).
- If a user with an active Pro/Pro+/Max plan is assigned a Business or Enterprise seat, "their personal plan is automatically canceled, and a prorated refund … is issued"; they then run under the org's policies (S41, S42). A community report confirms this happened and that the user lost the ability to manage their own model settings (S43).
- Copilot Free is only for individuals who do not have Copilot through an org (S8).
- Several org seats inside one enterprise = one billed seat; Enterprise wins over Business (S41).

**The Copilot token is per GitHub user account.** The `ghu_` OAuth token identifies the user; `copilot_internal/v2/token` returns that user's SKU, plan-specific `endpoints.api` host and a bearer that carries the entitlement. Rate-limit error codes are `user_…` scoped (S25, S29). So "several Copilot credentials" = several GitHub accounts, each with its own paid plan (or its own seat in an org), each authenticated through its own device flow, each with its own credit pool and its own rate-limit bucket.

**ToS constraints on holding several accounts** (GitHub Terms of Service §B.3, S40): "One person or legal entity may maintain no more than one **free** Account"; "Your login may only be used by one person"; "You must be a human to create an Account"; one free *machine* account allowed on top. The ToS sets **no numeric cap on paid accounts** and does not say a person may not own several paid personal accounts. **UNVERIFIED**: whether GitHub's 2026 Copilot sign-up vetting ("stronger account vetting", additional-credit caps by "verification status", S23, S9) would refuse or later flag several paid Copilot personal accounts owned by one person. The cleaner alternative is one **organization on Copilot Business** with N seats: N × 1,900 credits pooled, one billing entity, seats pre-paid from 2026-10-01 (S10, S23) — each seat still authenticates as its own user account.

---

## Sources

Dates are the source's own publication/last-change date where it shows one; "no date shown" means the page carries none and was fetched 2026-09-24.

| # | Source | Date |
|---|---|---|
| S1 | GitHub Docs — Supported AI models in Copilot: https://docs.github.com/en/copilot/reference/ai-models/supported-models | no date shown; lists Opus 5.5 / Grok 4.7 so ≥ 2026-09-22 |
| S2 | GitHub Docs — AI model comparison: https://docs.github.com/en/copilot/reference/ai-models/model-comparison | no date shown |
| S3 | models.dev `github-copilot` provider (API dump https://models.dev/api.json; TOMLs at https://github.com/anomalyco/models.dev/tree/dev/providers/github-copilot/models) | last commits 2026-09-23 (`claude-opus-5.5`, `gpt-6-sol`, `gpt-6-luna`), 2026-09-21 (`grok-4.7`), 2026-09-04 (`gpt-6-astra`) |
| S4 | GitHub Docs — Requests in GitHub Copilot (legacy): https://docs.github.com/en/copilot/concepts/billing/copilot-requests | no date shown; mentions 2026-06-01 cutover |
| S5 | GitHub Changelog — Selected GitHub Copilot models deprecated: https://github.blog/changelog/2026-08-31-selected-github-copilot-models-deprecated/ | 2026-08-31 (effective 09-01) |
| S6 | GitHub Changelog — Upcoming deprecation of selected GitHub Copilot models: https://github.blog/changelog/2026-09-03-upcoming-deprecation-of-selected-github-copilot-models/ | 2026-09-03 (effective 10-02) |
| S7 | GitHub Docs — Model multipliers for annual plans (legacy): https://docs.github.com/en/copilot/reference/copilot-billing/request-based-billing-legacy/model-multipliers-for-annual-plans | no date shown |
| S8 | GitHub Docs — Plans for GitHub Copilot: https://docs.github.com/en/copilot/get-started/plans | no date shown |
| S9 | GitHub Docs — Usage-based billing for individuals: https://docs.github.com/copilot/concepts/billing/usage-based-billing-for-individuals | no date shown |
| S10 | GitHub Docs — Usage-based billing for organizations and enterprises: https://docs.github.com/en/copilot/concepts/billing/usage-based-billing-for-organizations-and-enterprises | no date shown |
| S11 | GitHub Changelog — Updates to GitHub Copilot billing and plans: https://github.blog/changelog/2026-06-01-updates-to-github-copilot-billing-and-plans/ ; GitHub Blog — Copilot is moving to usage-based billing: https://github.blog/news-insights/company-news/github-copilot-is-moving-to-usage-based-billing/ | 2026-06-01; 2026-04-27 |
| S12 | GitHub Docs — Models and pricing for GitHub Copilot: https://docs.github.com/en/copilot/reference/copilot-billing/models-and-pricing | no date shown; includes Opus 5.5 |
| S13 | LiteLLM PR #17858 (verbatim `/models` dump from api.enterprise.githubcopilot.com): https://github.com/BerriAI/litellm/pull/17858 ; LiteLLM `model_prices_and_context_window.json` (`github_copilot/*` entries) ; GitHub community discussion #186340 (context_window vs max_prompt): https://github.com/orgs/community/discussions/186340 ; copilot-cli #4953 (custom_model fields): https://github.com/github/copilot-cli/issues/4953 | PR 2025-12-12/16; others 2026 |
| S14 | models.dev issue #858 — Copilot Claude limits: https://github.com/anomalyco/models.dev/issues/858 | 2026-02-11 |
| S15 | LiteLLM issue #28053 — route `/v1/messages` to Copilot native endpoint (thinking comparison): https://github.com/BerriAI/litellm/issues/28053 | 2026-05-16 |
| S16 | hermes-agent PR #57625 (supported_endpoints, bearer auth on /v1/messages): https://github.com/NousResearch/hermes-agent/pull/57625 | 2026 |
| S17 | messense/copilot-api-proxy README (native /v1/messages + count_tokens passthrough): https://github.com/messense/copilot-api-proxy ; whtsky/copilot2api: https://github.com/whtsky/copilot2api | fetched 2026-09-24 |
| S18 | models.dev issue #2021 — `-1m` Claude variants: https://github.com/anomalyco/models.dev/issues/2021 ; GitHub community discussion #198034 (1M context + reasoning levels, 2026-06-04 changelog): https://github.com/orgs/community/discussions/198034 | 2026-06-05; 2026-06-04 |
| S19 | Anthropic — Models overview: https://platform.claude.com/docs/en/models/overview ; Model IDs and versioning: https://platform.claude.com/docs/en/about-claude/models/model-ids-and-versions ; Opus 5.5 system prompts (lists current ids): https://platform.claude.com/docs/en/release-notes/system-prompts/claude-opus-5-5 | Opus 5.5 ≈ 2026-09-22 |
| S20 | OpenAI — GPT-5.6 Sol / Terra / GPT-6 Astra model pages: https://developers.openai.com/api/docs/models/gpt-5.6-sol , …/gpt-5.6-terra , …/gpt-6-astra ; latest-model guidance: https://developers.openai.com/api/docs/guides/latest-model | updated within days of 2026-09-24 |
| S21 | Google — What's new in Gemini 3.8 Flash: https://ai.google.dev/gemini-api/docs/latest-model ; model page https://ai.google.dev/gemini-api/docs/models/gemini-3.8-flash | GA 2026-09-02 |
| S22 | xAI — Grok 4.7: https://docs.x.ai/developers/grok-4-7 ; release notes: https://docs.x.ai/developers/release-notes | 2026-09-21 |
| S23 | GitHub Changelog — Upcoming changes to Copilot policies and billing: https://github.blog/changelog/2026-08-28-upcoming-changes-to-github-copilot-policies-and-billing/ ; Reopening Copilot Business and Enterprise signups: https://github.blog/changelog/2026-09-03-reopening-copilot-business-and-enterprise-signups/ | 2026-08-28; 2026-09-03 |
| S24 | GitHub Changelog — Global model policy generally available: https://github.blog/changelog/2026-08-26-global-model-policy-generally-available/ | 2026-08-26 |
| S25 | opencode issue #20759 — Business/Enterprise auth, endpoints, headers, per-client-ID allowlist, token exchange: https://github.com/anomalyco/opencode/issues/20759 | 2026-04-02 |
| S26 | openclaw issue #65556 (endpoint, headers, ~30 min token TTL): https://github.com/openclaw/openclaw/issues/65556 ; pi issue #9764 (token-exchange host): https://github.com/earendil-works/pi/issues/9764 | 2026-04; 2026 |
| S27 | openclaw issue #60174 — Claude on Copilot caching: auto-cache on OpenAI path, `cache_control` required on /v1/messages: https://github.com/openclaw/openclaw/issues/60174 | 2026 |
| S28 | microsoft/vscode #312939 (`copilot_cache_control` CAPI extension): https://github.com/microsoft/vscode/issues/312939 ; #321551 (cache expiry ≈5 min): https://github.com/microsoft/vscode/issues/321551 ; voidsteed/copilot-proxy-api README (cached_tokens mapping): https://github.com/voidsteed/copilot-proxy-api | 2026 |
| S29 | GitHub community — Pro+ `user_weekly_rate_limited`: https://github.com/orgs/community/discussions/192485 ; Pro 429 `user_global_rate_limited:pro`: https://github.com/orgs/community/discussions/180092 ; "5 hour session limits": https://github.com/orgs/community/discussions/190759 ; copilot-cli #2742: https://github.com/github/copilot-cli/issues/2742 | 2026-03 → 2026-09 |
| S30 | GitHub Docs — Rate limits for GitHub Copilot: https://docs.github.com/en/copilot/how-tos/troubleshoot/rate-limits-for-github-copilot ; concepts: https://docs.github.com/en/copilot/concepts/rate-limits | no date shown |
| S31 | GitHub Changelog — GitHub Copilot now supports OpenCode: https://github.blog/changelog/2026-01-16-github-copilot-now-supports-opencode/ | 2026-01-16 |
| S32 | GitHub community discussion #185336 — OpenCode legal protections (unanswered): https://github.com/orgs/community/discussions/185336 | 2026-01-26 |
| S33 | GitHub Changelog — Copilot SDK is now generally available: https://github.blog/changelog/2026-06-02-copilot-sdk-is-now-generally-available/ ; repo https://github.com/github/copilot-sdk | 2026-06-02 |
| S34 | ericc-ch/copilot-api README (security notice, account types, rate-limit flags, /v1/messages): https://github.com/ericc-ch/copilot-api | fetched 2026-09-24, no date |
| S35 | voidsteed/copilot-proxy-api README (abuse-control warning): https://github.com/voidsteed/copilot-proxy-api ; CLIProxyAPI README (Copilot not a native provider; community projects only): https://github.com/router-for-me/CLIProxyAPI | fetched 2026-09-24 |
| S36 | openclaw issue #133987 — Copilot models unavailable / 403 in v2026.8.x: https://github.com/openclaw/openclaw/issues/133987 | ≈2026-09-03 |
| S37 | The Register — GitHub suspends Copilot sign-ups: https://www.theregister.com/2026/04/20/microsofts_github_grounds_copilot_account/ ; GitHub Changelog — individual sign-ups reopening: https://github.blog/changelog/2026-06-17-copilot-individual-plan-sign-ups-are-reopening/ | 2026-04-20; 2026-06-17 |
| S38 | GitHub Copilot Product Specific Terms (deprecated 2026-03-05): https://github.com/customer-terms/github-copilot-product-specific-terms | version Oct 2024, deprecation notice 2026-03-05 |
| S39 | GitHub Generative AI Services Terms: https://github.com/customer-terms/github-generative-ai-services-terms | version March 2026 |
| S40 | GitHub Terms of Service §B.3/B.4: https://docs.github.com/en/site-policy/github-terms/github-terms-of-service | no date shown |
| S41 | GitHub Docs — Copilot seat assignment: https://docs.github.com/en/copilot/reference/copilot-billing/seat-assignment | no date shown |
| S42 | GitHub Docs — Billing for individuals: https://docs.github.com/en/copilot/concepts/billing/billing-for-individuals | no date shown |
| S43 | GitHub community discussion #179217 — personal plan auto-cancelled on org seat: https://github.com/orgs/community/discussions/179217 | 2026 |

Not consulted (would need a login): a live `GET /models` with a real Copilot bearer. That single call would close every UNVERIFIED item in §1, §3d and §4 (current ids, `-1m` siblings, current Claude limits, whether `billing.multiplier` still exists).
