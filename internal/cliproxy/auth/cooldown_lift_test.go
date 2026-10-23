package auth

import (
	"context"
	"net/http"
	"reflect"
	"testing"
	"time"
)

func quotaLiftTestAuth(id string, states map[string]*ModelState) *Auth {
	auth := &Auth{ID: id, Index: id, Provider: "codex", Status: StatusError, ModelStates: states}
	updateAggregatedAvailability(auth, time.Now().UTC())
	return auth
}

func TestLiftQuotaCooldownObservedAtLiftsBlockRecordedBeforeCheck(t *testing.T) {
	blockedAt := time.Now().UTC().Add(-time.Hour)
	state := quotaCooldownModelState(blockedAt, 48*time.Hour)
	store := &fakeMutatorStore{persisted: quotaLiftTestAuth("auth-lift-healthy", map[string]*ModelState{"gpt-6.1-sol": state})}
	manager := newHomeNodeManager(t, store, "auth-lift-healthy")

	result, errLift := manager.LiftQuotaCooldownObservedAt(context.Background(), "auth-lift-healthy", blockedAt.Add(time.Minute))
	if errLift != nil {
		t.Fatalf("LiftQuotaCooldownObservedAt() error = %v", errLift)
	}
	if !result.Cleared || !reflect.DeepEqual(result.ClearedModels, []string{"gpt-6.1-sol"}) {
		t.Fatalf("LiftQuotaCooldownObservedAt() result = %#v", result)
	}
	persisted := store.persistedSnapshot()
	if got := persisted.ModelStates["gpt-6.1-sol"]; got == nil || got.Quota.Exceeded || got.Unavailable || got.Status != StatusActive {
		t.Fatalf("persisted state = %#v, want lifted quota block", got)
	}
	local, ok := manager.GetByID("auth-lift-healthy")
	if !ok || local == nil {
		t.Fatal("local auth missing after lift")
	}
	if blocked, _, _ := isAuthBlockedForModel(local, "gpt-6.1-sol", time.Now().UTC()); blocked {
		t.Fatalf("local auth still blocked after lift: %#v", local.ModelStates["gpt-6.1-sol"])
	}
}

func TestLiftQuotaCooldownObservedAtKeepsBlockRecordedAfterCheck(t *testing.T) {
	blockedAt := time.Now().UTC().Add(-time.Minute)
	state := quotaCooldownModelState(blockedAt, 48*time.Hour)
	store := &fakeMutatorStore{persisted: quotaLiftTestAuth("auth-lift-late-block", map[string]*ModelState{"gpt-6.1-sol": state})}
	manager := newHomeNodeManager(t, store, "auth-lift-late-block")

	for _, observedAt := range []time.Time{blockedAt.Add(-time.Second), blockedAt} {
		result, errLift := manager.LiftQuotaCooldownObservedAt(context.Background(), "auth-lift-late-block", observedAt)
		if errLift != nil {
			t.Fatalf("LiftQuotaCooldownObservedAt(%s) error = %v", observedAt, errLift)
		}
		if result.Cleared || len(result.ClearedModels) != 0 {
			t.Fatalf("LiftQuotaCooldownObservedAt(%s) result = %#v, want no lift", observedAt, result)
		}
	}
	if got := store.persistedSnapshot().ModelStates["gpt-6.1-sol"]; got == nil || !got.Quota.Exceeded || !got.Unavailable {
		t.Fatalf("persisted state = %#v, want quota block kept", got)
	}
	if store.mutationCount() != 0 {
		t.Fatalf("mutation count = %d, want 0", store.mutationCount())
	}
}

func TestLiftQuotaCooldownObservedAtKeepsNonQuotaErrorBlocks(t *testing.T) {
	now := time.Now().UTC()
	blockedAt := now.Add(-time.Hour)
	quotaState := quotaCooldownModelState(blockedAt, 48*time.Hour)
	forbiddenRetry := now.Add(20 * time.Minute)
	forbidden := &ModelState{
		Status: StatusError, StatusMessage: "forbidden", Unavailable: true, NextRetryAfter: forbiddenRetry,
		LastError: &Error{Message: "forbidden", HTTPStatus: http.StatusForbidden}, UpdatedAt: blockedAt,
	}
	unauthorizedRetry := now.Add(10 * time.Minute)
	unauthorized := &ModelState{
		Status: StatusError, StatusMessage: "unauthorized", Unavailable: true, NextRetryAfter: unauthorizedRetry,
		LastError: &Error{Message: "unauthorized", HTTPStatus: http.StatusUnauthorized}, UpdatedAt: blockedAt,
	}
	unavailableRetry := now.Add(time.Minute)
	unavailable := &ModelState{
		Status: StatusError, StatusMessage: "upstream unavailable", Unavailable: true, NextRetryAfter: unavailableRetry,
		LastError: &Error{Message: "upstream unavailable", HTTPStatus: http.StatusServiceUnavailable}, UpdatedAt: blockedAt,
	}
	store := &fakeMutatorStore{persisted: quotaLiftTestAuth("auth-lift-mixed", map[string]*ModelState{
		"gpt-quota": quotaState,
		"gpt-403":   forbidden,
		"gpt-401":   unauthorized,
		"gpt-503":   unavailable,
	})}
	manager := newHomeNodeManager(t, store, "auth-lift-mixed")

	result, errLift := manager.LiftQuotaCooldownObservedAt(context.Background(), "auth-lift-mixed", now)
	if errLift != nil {
		t.Fatalf("LiftQuotaCooldownObservedAt() error = %v", errLift)
	}
	if !result.Cleared || !reflect.DeepEqual(result.ClearedModels, []string{"gpt-quota"}) {
		t.Fatalf("LiftQuotaCooldownObservedAt() result = %#v", result)
	}
	persisted := store.persistedSnapshot()
	if got := persisted.ModelStates["gpt-quota"]; got == nil || got.Quota.Exceeded || got.Unavailable {
		t.Fatalf("gpt-quota state = %#v, want lifted", got)
	}
	for model, want := range map[string]struct {
		status int
		retry  time.Time
	}{
		"gpt-403": {http.StatusForbidden, forbiddenRetry},
		"gpt-401": {http.StatusUnauthorized, unauthorizedRetry},
		"gpt-503": {http.StatusServiceUnavailable, unavailableRetry},
	} {
		got := persisted.ModelStates[model]
		if got == nil || !got.Unavailable || got.LastError == nil || got.LastError.HTTPStatus != want.status || !got.NextRetryAfter.Equal(want.retry) {
			t.Fatalf("%s state = %#v, want non-quota block kept", model, got)
		}
	}
}

func TestLiftQuotaCooldownObservedAtKeepsNewerLocalQuotaBlock(t *testing.T) {
	now := time.Now().UTC()
	persistedBlockAt := now.Add(-time.Hour)
	store := &fakeMutatorStore{persisted: quotaLiftTestAuth("auth-lift-local-newer", map[string]*ModelState{
		"gpt-6.1-sol": quotaCooldownModelState(persistedBlockAt, 48*time.Hour),
	})}
	manager := newHomeNodeManager(t, store, "auth-lift-local-newer")
	// A newer 429 already applied locally whose persisted snapshot is still queued.
	localBlockAt := now.Add(-time.Minute)
	manager.mu.Lock()
	manager.auths["auth-lift-local-newer"].ModelStates = map[string]*ModelState{
		"gpt-6.1-sol": quotaCooldownModelState(localBlockAt, 48*time.Hour),
	}
	manager.mu.Unlock()

	result, errLift := manager.LiftQuotaCooldownObservedAt(context.Background(), "auth-lift-local-newer", now.Add(-10*time.Minute))
	if errLift != nil {
		t.Fatalf("LiftQuotaCooldownObservedAt() error = %v", errLift)
	}
	if result.Cleared {
		t.Fatalf("LiftQuotaCooldownObservedAt() result = %#v, want no lift", result)
	}
	if got := store.persistedSnapshot().ModelStates["gpt-6.1-sol"]; got == nil || !got.Quota.Exceeded {
		t.Fatalf("persisted state = %#v, want quota block kept", got)
	}
	local, ok := manager.GetByID("auth-lift-local-newer")
	if !ok || local == nil {
		t.Fatal("local auth missing")
	}
	if got := local.ModelStates["gpt-6.1-sol"]; got == nil || !got.Quota.Exceeded {
		t.Fatalf("local state = %#v, want quota block kept", got)
	}
	if blocked, _, _ := isAuthBlockedForModel(local, "gpt-6.1-sol", now); !blocked {
		t.Fatal("local auth no longer blocked, want newer local quota block to keep routing away")
	}
}
