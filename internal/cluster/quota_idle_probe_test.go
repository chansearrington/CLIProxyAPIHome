package cluster

import (
	"context"
	"testing"
	"time"

	coreauth "github.com/router-for-me/CLIProxyAPIHome/internal/cliproxy/auth"
)

// seedQuotaExhaustedReading saves a successful exhausted reading observed at
// observedAt whose weekly window resets at resetAt.
func seedQuotaExhaustedReading(t *testing.T, repo *Repository, id string, observedAt time.Time, resetAt time.Time) {
	t.Helper()
	expiresAt := observedAt.Add(30 * time.Minute)
	period := float64(1)
	remaining := float64(0)
	if _, errSeed := repo.UpsertQuotaSnapshot(context.Background(), QuotaSnapshotWrite{
		CredentialID: id, QuotaStatus: "exhausted", CollectionStatus: "success", Source: "active_probe",
		ObservedAt: &observedAt, ExpiresAt: &expiresAt, NextProbeAt: &expiresAt, LastAttemptAt: &observedAt, LastSuccessAt: &observedAt,
		ParserVersion: codexQuotaSnapshotVersion, CollectorVersion: codexQuotaSnapshotVersion, ReplaceWindows: true,
		Windows: []QuotaWindow{{
			ID: "codex-weekly", Scope: "account", Mode: "rolling", Status: "exhausted", Unit: "percentage",
			RemainingRatio: &remaining, ResetAt: &resetAt, PeriodUnit: "week", PeriodValue: &period,
			Source: "active_probe", ObservedAt: observedAt,
		}},
	}); errSeed != nil {
		t.Fatalf("UpsertQuotaSnapshot(%s) error = %v", id, errSeed)
	}
}

func TestClaimEligibleQuotaProbeRechecksIdleUnblockedCredentialAfterInterval(t *testing.T) {
	ctx := context.Background()
	repo, closeRepo := newBillingTestRepository(t, ctx)
	defer closeRepo()
	lastAttemptAt := time.Date(2026, 10, 24, 3, 0, 0, 0, time.UTC)
	seedQuotaSnapshotAuth(t, repo, "standby-idle", "codex", "Standby", map[string]any{"type": "codex"})
	seedQuotaLastAttempt(t, repo, "standby-idle", lastAttemptAt)

	// Freshness and next_probe_at already passed 30 minutes after the attempt,
	// so only the idle cadence keeps the credential from being claimed here.
	beforeInterval := lastAttemptAt.Add(quotaIdleProbeInterval - time.Second)
	claimed, errClaim := repo.ClaimEligibleQuotaProbe(ctx, "standby-idle", "home-a", beforeInterval, time.Minute)
	if errClaim != nil || claimed {
		t.Fatalf("claim before idle interval = %v, %v, want false, nil", claimed, errClaim)
	}

	atInterval := lastAttemptAt.Add(quotaIdleProbeInterval)
	claimed, errClaim = repo.ClaimEligibleQuotaProbe(ctx, "standby-idle", "home-a", atInterval, time.Minute)
	if errClaim != nil || !claimed {
		t.Fatalf("claim at idle interval = %v, %v, want true, nil", claimed, errClaim)
	}
}

func TestClaimEligibleQuotaProbeRechecksOnceAfterSavedReset(t *testing.T) {
	ctx := context.Background()
	repo, closeRepo := newBillingTestRepository(t, ctx)
	defer closeRepo()
	observedAt := time.Date(2026, 10, 24, 3, 0, 0, 0, time.UTC)
	resetAt := observedAt.Add(2 * time.Hour)
	seedQuotaSnapshotAuth(t, repo, "weekly-rollover", "codex", "Weekly Rollover", map[string]any{"type": "codex"})
	seedQuotaExhaustedReading(t, repo, "weekly-rollover", observedAt, resetAt)

	claimed, errClaim := repo.ClaimEligibleQuotaProbe(ctx, "weekly-rollover", "home-a", resetAt.Add(-time.Minute), time.Minute)
	if errClaim != nil || claimed {
		t.Fatalf("claim before saved reset = %v, %v, want false, nil", claimed, errClaim)
	}
	claimed, errClaim = repo.ClaimEligibleQuotaProbe(ctx, "weekly-rollover", "home-a", resetAt.Add(quotaResetProbeGrace-time.Second), time.Minute)
	if errClaim != nil || claimed {
		t.Fatalf("claim inside reset grace = %v, %v, want false, nil", claimed, errClaim)
	}

	claimAt := resetAt.Add(quotaResetProbeGrace)
	claimed, errClaim = repo.ClaimEligibleQuotaProbe(ctx, "weekly-rollover", "home-a", claimAt, time.Minute)
	if errClaim != nil || !claimed {
		t.Fatalf("claim after saved reset = %v, %v, want true, nil", claimed, errClaim)
	}

	// A failed check keeps the old window and its passed reset time, but the
	// attempt consumes that reset: once the short failure backoff ends, the
	// credential waits for the idle cadence instead of retrying every backoff.
	failedAt := claimAt.Add(time.Second)
	if errFail := repo.FailQuotaProbeAt(ctx, "weekly-rollover", "home-a", QuotaCollectionError{
		Code: "UPSTREAM_RATE_LIMITED", Message: "rate limited", Retryable: true, OccurredAt: &failedAt,
	}, failedAt.Add(5*time.Minute), failedAt); errFail != nil {
		t.Fatalf("FailQuotaProbeAt() error = %v", errFail)
	}
	claimed, errClaim = repo.ClaimEligibleQuotaProbe(ctx, "weekly-rollover", "home-b", failedAt.Add(10*time.Minute), time.Minute)
	if errClaim != nil || claimed {
		t.Fatalf("repeat claim for consumed reset = %v, %v, want false, nil", claimed, errClaim)
	}
	claimed, errClaim = repo.ClaimEligibleQuotaProbe(ctx, "weekly-rollover", "home-b", failedAt.Add(quotaIdleProbeInterval-time.Second), time.Minute)
	if errClaim != nil || claimed {
		t.Fatalf("claim before idle interval after reset check = %v, %v, want false, nil", claimed, errClaim)
	}
	claimed, errClaim = repo.ClaimEligibleQuotaProbe(ctx, "weekly-rollover", "home-b", failedAt.Add(quotaIdleProbeInterval), time.Minute)
	if errClaim != nil || !claimed {
		t.Fatalf("claim at idle interval after reset check = %v, %v, want true, nil", claimed, errClaim)
	}
}

func TestClaimEligibleQuotaProbeRechecksBlockedCredentialAfterSavedReset(t *testing.T) {
	ctx := context.Background()
	repo, closeRepo := newBillingTestRepository(t, ctx)
	defer closeRepo()
	observedAt := time.Date(2026, 10, 24, 3, 0, 0, 0, time.UTC)
	resetAt := observedAt.Add(10 * time.Minute)
	// The routing block outlives the provider reset, as a stale 429 timer does.
	seedQuotaBlockedAuth(t, repo, "blocked-rollover", observedAt.Add(48*time.Hour))
	seedQuotaExhaustedReading(t, repo, "blocked-rollover", observedAt, resetAt)

	// The hourly blocked cadence is not due yet; the passed reset is.
	claimAt := observedAt.Add(31 * time.Minute)
	claimed, errClaim := repo.ClaimEligibleQuotaProbe(ctx, "blocked-rollover", "home-a", claimAt, time.Minute)
	if errClaim != nil || !claimed {
		t.Fatalf("blocked claim after saved reset = %v, %v, want true, nil", claimed, errClaim)
	}
}

func TestClaimEligibleQuotaProbeNeverRechecksDisabledIdleCredential(t *testing.T) {
	ctx := context.Background()
	repo, closeRepo := newBillingTestRepository(t, ctx)
	defer closeRepo()
	observedAt := time.Date(2026, 10, 20, 3, 0, 0, 0, time.UTC)
	createdAt := observedAt.Add(-time.Hour)
	auth := &coreauth.Auth{
		ID: "disabled-idle", Index: "disabled-idle", Provider: "codex", Label: "Disabled", Status: coreauth.StatusDisabled, Disabled: true,
		Metadata: map[string]any{"type": "codex"}, CreatedAt: createdAt, UpdatedAt: createdAt,
	}
	if _, errUpsert := repo.UpsertAuth(ctx, auth, "test"); errUpsert != nil {
		t.Fatalf("UpsertAuth() error = %v", errUpsert)
	}
	seedQuotaExhaustedReading(t, repo, "disabled-idle", observedAt, observedAt.Add(time.Hour))

	// Both the idle interval and the saved reset have long passed.
	claimed, errClaim := repo.ClaimEligibleQuotaProbe(ctx, "disabled-idle", "home-a", observedAt.Add(96*time.Hour), time.Minute)
	if errClaim != nil || claimed {
		t.Fatalf("disabled idle claim = %v, %v, want false, nil", claimed, errClaim)
	}
}

func TestClaimEligibleQuotaProbeIdleRecheckHonorsFailureBackoff(t *testing.T) {
	ctx := context.Background()
	repo, closeRepo := newBillingTestRepository(t, ctx)
	defer closeRepo()
	lastAttemptAt := time.Date(2026, 10, 24, 3, 0, 0, 0, time.UTC)
	seedQuotaSnapshotAuth(t, repo, "idle-backoff", "codex", "Idle Backoff", map[string]any{"type": "codex"})
	seedQuotaLastAttempt(t, repo, "idle-backoff", lastAttemptAt)

	failedAt := lastAttemptAt.Add(quotaIdleProbeInterval)
	claimed, errClaim := repo.ClaimEligibleQuotaProbe(ctx, "idle-backoff", "home-a", failedAt, time.Minute)
	if errClaim != nil || !claimed {
		t.Fatalf("idle claim = %v, %v, want true, nil", claimed, errClaim)
	}
	// A retry schedule longer than the idle interval still wins.
	retryAt := failedAt.Add(quotaIdleProbeInterval + 2*time.Hour)
	if errFail := repo.FailQuotaProbeAt(ctx, "idle-backoff", "home-a", QuotaCollectionError{
		Code: "UPSTREAM_UNAVAILABLE", Message: "failed", Retryable: true, OccurredAt: &failedAt,
	}, retryAt, failedAt); errFail != nil {
		t.Fatalf("FailQuotaProbeAt() error = %v", errFail)
	}

	claimed, errClaim = repo.ClaimEligibleQuotaProbe(ctx, "idle-backoff", "home-b", failedAt.Add(quotaIdleProbeInterval), time.Minute)
	if errClaim != nil || claimed {
		t.Fatalf("idle claim during failure backoff = %v, %v, want false, nil", claimed, errClaim)
	}
	claimed, errClaim = repo.ClaimEligibleQuotaProbe(ctx, "idle-backoff", "home-b", retryAt, time.Minute)
	if errClaim != nil || !claimed {
		t.Fatalf("idle claim after failure backoff = %v, %v, want true, nil", claimed, errClaim)
	}
}

func TestClaimEligibleQuotaProbeIdleRecheckUsesPassiveObservationWithoutAttempt(t *testing.T) {
	ctx := context.Background()
	repo, closeRepo := newBillingTestRepository(t, ctx)
	defer closeRepo()
	observedAt := time.Date(2026, 10, 24, 3, 0, 0, 0, time.UTC)
	seedQuotaSnapshotAuth(t, repo, "passive-only", "codex", "Passive Only", map[string]any{"type": "codex"})
	period := float64(5)
	remaining := 0.5
	// A response-header reading carries no collection attempt time.
	if _, errSeed := repo.UpsertQuotaSnapshot(ctx, QuotaSnapshotWrite{
		CredentialID: "passive-only", QuotaStatus: "healthy", CollectionStatus: "success", Source: "response_header",
		ObservedAt: &observedAt, ParserVersion: codexQuotaSnapshotVersion, CollectorVersion: codexQuotaSnapshotVersion, ReplaceWindows: true,
		Windows: []QuotaWindow{{ID: "codex-5-hour", Scope: "account", Mode: "rolling", Status: "healthy", Unit: "percentage", RemainingRatio: &remaining, PeriodUnit: "hour", PeriodValue: &period, Source: "response_header", ObservedAt: observedAt}},
	}); errSeed != nil {
		t.Fatalf("UpsertQuotaSnapshot() error = %v", errSeed)
	}

	claimed, errClaim := repo.ClaimEligibleQuotaProbe(ctx, "passive-only", "home-a", observedAt.Add(quotaIdleProbeInterval-time.Second), time.Minute)
	if errClaim != nil || claimed {
		t.Fatalf("claim before idle interval from observation = %v, %v, want false, nil", claimed, errClaim)
	}
	claimed, errClaim = repo.ClaimEligibleQuotaProbe(ctx, "passive-only", "home-a", observedAt.Add(quotaIdleProbeInterval), time.Minute)
	if errClaim != nil || !claimed {
		t.Fatalf("claim at idle interval from observation = %v, %v, want true, nil", claimed, errClaim)
	}
}

func TestClaimEligibleQuotaProbeSavedResetCheckRunsOnceAfterSuccess(t *testing.T) {
	ctx := context.Background()
	repo, closeRepo := newBillingTestRepository(t, ctx)
	defer closeRepo()
	observedAt := time.Date(2026, 10, 24, 3, 0, 0, 0, time.UTC)
	resetAt := observedAt.Add(2 * time.Hour)
	seedQuotaSnapshotAuth(t, repo, "stale-reset", "codex", "Stale Reset", map[string]any{"type": "codex"})
	seedQuotaExhaustedReading(t, repo, "stale-reset", observedAt, resetAt)

	claimAt := resetAt.Add(quotaResetProbeGrace)
	claimed, errClaim := repo.ClaimEligibleQuotaProbe(ctx, "stale-reset", "home-a", claimAt, time.Minute)
	if errClaim != nil || !claimed {
		t.Fatalf("claim after saved reset = %v, %v, want true, nil", claimed, errClaim)
	}
	// The provider answers with the same already-passed reset time. The
	// successful check consumes it, so it does not trigger another check.
	checkedAt := claimAt.Add(time.Second)
	expiresAt := checkedAt.Add(30 * time.Minute)
	period := float64(1)
	remaining := float64(0)
	if _, errComplete := repo.UpsertQuotaSnapshot(ctx, QuotaSnapshotWrite{
		CredentialID: "stale-reset", QuotaStatus: "exhausted", CollectionStatus: "success", Source: "active_probe",
		ObservedAt: &checkedAt, MaxAcceptedObservedAt: &checkedAt, ExpiresAt: &expiresAt, NextProbeAt: &expiresAt, LastAttemptAt: &checkedAt, LastSuccessAt: &checkedAt,
		ParserVersion: codexQuotaSnapshotVersion, CollectorVersion: codexQuotaSnapshotVersion, ExpectedProbeOwner: "home-a", ClearProbeLease: true, ReplaceWindows: true,
		Windows: []QuotaWindow{{
			ID: "codex-weekly", Scope: "account", Mode: "rolling", Status: "exhausted", Unit: "percentage",
			RemainingRatio: &remaining, ResetAt: &resetAt, PeriodUnit: "week", PeriodValue: &period,
			Source: "active_probe", ObservedAt: checkedAt,
		}},
	}); errComplete != nil {
		t.Fatalf("complete probe: %v", errComplete)
	}

	claimed, errClaim = repo.ClaimEligibleQuotaProbe(ctx, "stale-reset", "home-b", expiresAt.Add(time.Minute), time.Minute)
	if errClaim != nil || claimed {
		t.Fatalf("repeat claim after successful reset check = %v, %v, want false, nil", claimed, errClaim)
	}
}

func TestClaimEligibleQuotaProbeRechecksFutureDatedIdleReading(t *testing.T) {
	ctx := context.Background()
	repo, closeRepo := newBillingTestRepository(t, ctx)
	defer closeRepo()
	now := time.Date(2026, 10, 24, 3, 0, 0, 0, time.UTC)
	seedQuotaSnapshotAuth(t, repo, "future-reading", "codex", "Future Reading", map[string]any{"type": "codex"})
	// A reading saved before the clock was stepped back a day.
	seedQuotaLastAttempt(t, repo, "future-reading", now.Add(24*time.Hour))

	claimed, errClaim := repo.ClaimEligibleQuotaProbe(ctx, "future-reading", "home-a", now, time.Minute)
	if errClaim != nil || !claimed {
		t.Fatalf("claim for future-dated reading = %v, %v, want true, nil", claimed, errClaim)
	}
	snapshot, errSnapshot := repo.GetQuotaCredential(ctx, "future-reading", now)
	if errSnapshot != nil {
		t.Fatalf("GetQuotaCredential() error = %v", errSnapshot)
	}
	if snapshot.ObservedAt != nil {
		t.Fatalf("future-dated observation kept after claim: %v", snapshot.ObservedAt)
	}
}
