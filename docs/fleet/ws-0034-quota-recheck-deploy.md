# ws-0034: deploy the quota re-check fixes (c632a78) to Home

Decision 339 (2026-10-25, Chanse): "There is no quiet hour. install it now and make sure it works for all accounts
not just codex." Deployed by the Token Management lead.

## What shipped

- PR #18 (`9b3faca`): a healthy quota check lifts a stale routing block (`LiftQuotaCooldownObservedAt`); a
  credential blocked for quota is re-checked every hour (`quotaBlockedProbeInterval`).
- PR #19 (`b347bd1`): idle credentials are re-checked every 6 h and right after a saved window reset
  (`quotaIdleRecheckDue`).
- Home container log keep raised from 3 x 10 MB (about 9 h) to 10 x 50 MB (about 4 days), so operator actions
  stay traceable (a cooldown sweep on 2026-10-24 could not be attributed because the log had rotated).

Why: on 2026-10-24/25 OpenAI renewed all four Codex weeks days early, but Home kept two of them blocked until their
old reset times (Oct 30/31) because a blocked credential gets no traffic and was never re-read. Two full accounts
sat unused for about 12 h until a manual one-account reset.

## Build (Ark, all four fleet locks held)

- Source: `git archive c632a78` plus the 64 panel assets (3,922,518 bytes) from ws-0031's image source, which
  matched the running panel. Tree diff against ws-0031's image source = exactly `git diff 2800d59 c632a78`.
- Image `cpa-home:1.1.1-claude-fleet-c632a78`, ID
  `sha256:fe690fc3094b17741ba6e7c96c693006cf99be39f9cff5accdbc15a1f60a4863`.
- New binary contains `LiftQuotaCooldownObservedAt` and `quotaIdleRecheckDue`; the old one contains neither;
  control symbol `quotaCooldownAfterFailure` is in both.
- `go test -count=1` passed for `internal/quota`, `internal/cliproxy/auth`, `internal/cluster`,
  `internal/cluster/management`. `go vet` on those packages stops on a pre-existing warning in an unchanged file
  (`internal/cluster/refresh.go:172`, unreachable code, last changed 2026-08-29); not fixed here.

## Swap

- DB backup `data/backups/home-pre-c632a78-20261025T142120Z.db`, 13,722,746,880 bytes, mode 0600,
  integrity `ok`, SHA256 `ac3c13d4ed558eda7dc07ee46afa2e279c9846e194623062079d308ac021e9e2`.
- Compose backup `docker-compose.yml.pre-c632a78-20261025T143211Z.bak`; only the image tag and the two log options
  changed.
- Home's fleet policy was not paused: no fleet check lease was held and none was due until about 14:49Z.
- `docker compose up -d home` at 14:32:13.8Z. Home did not answer from 14:32:14.1Z to 14:32:46.8Z (33 s,
  probed every 0.2 s), then answered on the new image. State running, 0 restarts, no panic/fatal lines,
  LogConfig `max-file:10 max-size:50m`.
- Requests 14:29-14:45Z: Codex 384 ok before, 7 ok during, 435 ok after; Claude 24 ok before, 32 ok after;
  no failed request recorded (a request that failed on a node while Home was down may not be recorded).
  All five node memberships reconnected with plugin `reported_ok`; the panel loads.
- Rollback target kept: `cpa-home:1.1.1-claude-fleet-2800d59`
  (`sha256:3f421772d8ddf859a9e29c415540c2bb97cd7de2d639688598309b6715c57122`).

## Result on both families

- Claude: within 6 min Home re-read all 10 Claude credentials (readings 23-136 h old became fresh) and lifted
  23 stale model blocks on Claude credentials with room (`lifted stale quota cooldown after healthy quota check`,
  all `provider=claude`, from 14:32:49Z). Credentials that are really exhausted stay blocked until their real reset.
- Codex: all four credentials read fresh. None was blocked at deploy time (two had been reset by hand that morning),
  so the Codex block lift is covered by tests, not yet observed live.

Private evidence: `/mnt/user/appdata/cpa-home-build/ws0034-c632a78/` on the Ark (build, test, backup, swap and
probe logs, `deployment.json`).
