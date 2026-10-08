# Home and CPA fleet upgrade

Status: source integration and compatibility audit in progress, 2026-10-08 UTC.

Chanse authorized bringing the available Home and CPA updates into the fork, testing them,
and deploying across the fleet. Targets are upstream Home 1.1.0 (`c098d84`) and CPA 8.0.20,
plus the Home Claude refresh-safety port `643f6c4`. Both release targets were rechecked against
GitHub's latest release endpoints before starting.

This instruction explicitly authorizes deployment; the fleet's locks, backups, rollback,
preservation, and proof requirements still apply. Go tools run only on the Ark inside Docker.
One live change runs at a time. Nodes restart in place, one at a time.

## Required preservation and verification

- Preserve all fleet source behavior, especially Claude reset handling and sequential priority,
  quota storage, Copilot plugin OAuth/discovery/quota, and Microsoft channel isolation.
- Preserve the live partial usage-accounting index, fresh Gmail/HypeSports authorizations,
  all six API keys, credential UUIDs, priorities 3 → 2 → 1, and channel groups.
- Resolve Home V8 and shared configuration layout changes against the existing runtime contract.
- Test formatting, the full Go suite, vet, and CGO compilation on the Ark. Review the integrated
  diff and explicitly check Home/node protocol and plugin compatibility before deployment.
- Mirror and verify management panel assets before building a deployable Home image.
- Verify a fresh full database backup, compose/config backups, and the current image ID before
  cutover. Rehearse database migration and rollback compatibility without allowing a shadow Home
  to refresh or use copied live OAuth credentials.
- After each live change, prove native Claude and Codex requests, all six active API keys,
  Copilot access and fencing, healthy membership/plugin reports, and usage attribution.
- Canary on the MacBook, then a mini, then the remaining nodes individually. Roll back a failed
  proof before investigating further.

## Source and rollout evidence

Pending integration, test, build, and deployment results.
