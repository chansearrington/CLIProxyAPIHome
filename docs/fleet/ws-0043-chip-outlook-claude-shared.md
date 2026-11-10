# Chip Outlook Claude shared account

Status: complete — shared at highest Claude priority 13; all six original keys pass Sonnet, Opus passes, all seven requests attributed to the new account; maintenance restored and all four locks released.

The user urgently requested `chip.bullard@outlook.com` be shared and prioritized in CPA Home.
Retained original credential UUID `3a836628-581b-4ed0-a3a9-e36eb010711a` and assigned only that account priority **13**
with shared channel-group-1 binding **31**. All existing priorities, account settings,
client key identities/scopes, model bindings and inference configuration match the saved baseline.

Saved only the affected configuration and bindings before mutation:
`/mnt/user/appdata/cpa-home/data/backups/home-config-20261109T231353Z.json`
(13,256 bytes, mode 0600). The directory retains the two newest targeted configuration
backups. No usage/history database copy, image upgrade, node restart or native CLI login change.

Home maintenance had no active task. Its complete policy was saved and paused; all four canonical
fleet locks were acquired and checked before mutation. Live Sonnet requests through the MacBook
node succeeded for all six original keys; key 6 also passed Opus. Every verification request
returned HTTP 200 with the expected response and exact new-account Home attribution.

| Key/model | Home usage ID |
| --- | --- |
| 1 / claude-sonnet-5-5 | 3029111 |
| 2 / claude-sonnet-5-5 | 3029114 |
| 3 / claude-sonnet-5-5 | 3029115 |
| 4 / claude-sonnet-5-5 | 3029113 |
| 5 / claude-sonnet-5-5 | 3029117 |
| 6 / claude-sonnet-5-5 | 3029118 |
| 6 / claude-opus-5-5 | 3029119 |

Queued a normal provider quota collection. Restored the exact saved maintenance policy and
released all four locks. This urgent request was scoped to CPA Home; CodexBar was not modified.

Scoped rollback removes only binding 31 and restores the new account’s prior unset priority;
retain the credential UUID, authorization and history. Never restore a whole database for this change.

Private operation evidence: `/mnt/user/appdata/cpa-home-build/ws0043-chip-outlook/` on Ark and
`/private/tmp/cpa-chip-outlook-20261109/` locally.
