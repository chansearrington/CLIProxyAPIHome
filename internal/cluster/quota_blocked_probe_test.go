package cluster

import (
	"context"
	"testing"
	"time"

	coreauth "github.com/router-for-me/CLIProxyAPIHome/internal/cliproxy/auth"
)

func seedQuotaBlockedAuth(t *testing.T, repo *Repository, id string, recoverAt time.Time) {
	t.Helper()
	createdAt := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	auth := &coreauth.Auth{
		ID: id, Index: id, Provider: "codex", Label: id, Status: coreauth.StatusError,
		Metadata: map[string]any{"type": "codex"}, CreatedAt: createdAt, UpdatedAt: createdAt,
		ModelStates: map[string]*coreauth.ModelState{
			"gpt-6.1-sol": {
				Status: coreauth.StatusError, Unavailable: true, NextRetryAfter: recoverAt,
				LastError: &coreauth.Error{Message: "usage limit reached", HTTPStatus: 429},
				Quota:     coreauth.QuotaState{Exceeded: true, Scope: "model", Reason: "quota", NextRecoverAt: recoverAt},
				UpdatedAt: createdAt,
			},
		},
	}
	if _, errUpsert := repo.UpsertAuth(context.Background(), auth, "test"); errUpsert != nil {
		t.Fatalf("UpsertAuth(%s) error = %v", id, errUpsert)
	}
}

func seedQuotaLastAttempt(t *testing.T, repo *Repository, id string, attemptAt time.Time) {
	t.Helper()
	expiresAt := attemptAt.Add(30 * time.Minute)
	if _, errSeed := repo.UpsertQuotaSnapshot(context.Background(), QuotaSnapshotWrite{
		CredentialID: id, QuotaStatus: "exhausted", CollectionStatus: "success", Source: "active_probe",
		ObservedAt: &attemptAt, ExpiresAt: &expiresAt, NextProbeAt: &expiresAt, LastAttemptAt: &attemptAt, LastSuccessAt: &attemptAt,
		ParserVersion: codexQuotaSnapshotVersion, CollectorVersion: codexQuotaSnapshotVersion,
	}); errSeed != nil {
		t.Fatalf("UpsertQuotaSnapshot(%s) error = %v", id, errSeed)
	}
}

func TestClaimEligibleQuotaProbeChecksIdleBlockedCredentialWithoutSnapshot(t *testing.T) {
	ctx := context.Background()
	repo, closeRepo := newBillingTestRepository(t, ctx)
	defer closeRepo()
	now := time.Date(2026, 10, 23, 14, 0, 0, 0, time.UTC)
	seedQuotaBlockedAuth(t, repo, "blocked-never-checked", now.Add(48*time.Hour))

	claimed, errClaim := repo.ClaimEligibleQuotaProbe(ctx, "blocked-never-checked", "home-a", now, time.Minute)
	if errClaim != nil || !claimed {
		t.Fatalf("ClaimEligibleQuotaProbe() = %v, %v, want true, nil", claimed, errClaim)
	}
}

func TestClaimEligibleQuotaProbeChecksIdleBlockedCredentialAtBoundedCadence(t *testing.T) {
	ctx := context.Background()
	repo, closeRepo := newBillingTestRepository(t, ctx)
	defer closeRepo()
	lastAttemptAt := time.Date(2026, 10, 23, 13, 0, 0, 0, time.UTC)
	seedQuotaBlockedAuth(t, repo, "blocked-idle", lastAttemptAt.Add(48*time.Hour))
	seedQuotaLastAttempt(t, repo, "blocked-idle", lastAttemptAt)

	// The snapshot is already past its own freshness and retry schedule here,
	// so only the blocked-credential cadence keeps it from being claimed.
	beforeCadence := lastAttemptAt.Add(quotaBlockedProbeInterval - time.Second)
	claimed, errClaim := repo.ClaimEligibleQuotaProbe(ctx, "blocked-idle", "home-a", beforeCadence, time.Minute)
	if errClaim != nil || claimed {
		t.Fatalf("claim before cadence = %v, %v, want false, nil", claimed, errClaim)
	}

	atCadence := lastAttemptAt.Add(quotaBlockedProbeInterval)
	claimed, errClaim = repo.ClaimEligibleQuotaProbe(ctx, "blocked-idle", "home-a", atCadence, time.Minute)
	if errClaim != nil || !claimed {
		t.Fatalf("claim at cadence = %v, %v, want true, nil", claimed, errClaim)
	}
}

func TestClaimEligibleQuotaProbeSkipsIdleCredentialWithoutActiveBlock(t *testing.T) {
	ctx := context.Background()
	repo, closeRepo := newBillingTestRepository(t, ctx)
	defer closeRepo()
	lastAttemptAt := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	// Past the hourly blocked cadence but inside the slower idle cadence, so
	// only an active block could make these credentials due.
	now := lastAttemptAt.Add(2 * quotaBlockedProbeInterval)
	if !now.Before(lastAttemptAt.Add(quotaIdleProbeInterval)) {
		t.Fatalf("test time %v must fall inside the idle cadence", now)
	}

	seedQuotaSnapshotAuth(t, repo, "unblocked-idle", "codex", "Unblocked", map[string]any{"type": "codex"})
	seedQuotaLastAttempt(t, repo, "unblocked-idle", lastAttemptAt)
	claimed, errClaim := repo.ClaimEligibleQuotaProbe(ctx, "unblocked-idle", "home-a", now, time.Minute)
	if errClaim != nil || claimed {
		t.Fatalf("unblocked idle claim = %v, %v, want false, nil", claimed, errClaim)
	}

	// A block whose timer already ran out no longer keeps routing away.
	seedQuotaBlockedAuth(t, repo, "expired-block-idle", now.Add(-time.Minute))
	seedQuotaLastAttempt(t, repo, "expired-block-idle", lastAttemptAt)
	claimed, errClaim = repo.ClaimEligibleQuotaProbe(ctx, "expired-block-idle", "home-a", now, time.Minute)
	if errClaim != nil || claimed {
		t.Fatalf("expired block idle claim = %v, %v, want false, nil", claimed, errClaim)
	}
}
