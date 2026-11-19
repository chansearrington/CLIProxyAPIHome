package cluster

import (
	"context"
	"testing"
	"time"

	coreauth "github.com/router-for-me/CLIProxyAPIHome/internal/cliproxy/auth"
)

func TestLateTruncatedStreamPreservesRefreshFenceAfterReload(t *testing.T) {
	for _, code := range []string{coreauth.ErrorCodeRefreshOutcomeUnknown, "refresh_temporarily_unavailable"} {
		t.Run(code, func(t *testing.T) {
			ctx := context.Background()
			repo := newRefreshTestRepository(t)
			retryAt := time.Now().Add(-time.Hour)
			seed := &coreauth.Auth{ID: "fake-late-stream", Index: "fake-late-stream", Provider: "codex", Status: coreauth.StatusError,
				Unavailable: true, RuntimeRefreshBlocked: true, NextRetryAfter: retryAt,
				StatusMessage: "fake refresh diagnostic", LastError: &coreauth.Error{Code: code, Message: "fake refresh diagnostic"},
				Metadata: map[string]any{"access_token": "fake-access"}}
			if _, errSave := repo.UpsertAuth(ctx, seed, "register"); errSave != nil {
				t.Fatal(errSave)
			}
			stored, _, errGet := repo.GetAuth(ctx, seed.ID)
			if errGet != nil {
				t.Fatal(errGet)
			}
			manager := coreauth.NewManager(nil, nil, nil)
			t.Cleanup(manager.Shutdown)
			if _, errRegister := manager.Register(coreauth.WithSkipPersist(ctx), stored); errRegister != nil {
				t.Fatal(errRegister)
			}
			if errMark := manager.MarkResult(ctx, coreauth.Result{AuthID: seed.ID, Model: "fake-model", Error: &coreauth.Error{Code: coreauth.ErrorCodeStreamTruncated, Message: "fake truncated stream"}}); errMark != nil {
				t.Fatal(errMark)
			}
			updated, ok := manager.GetByID(seed.ID)
			if !ok {
				t.Fatal("updated auth missing")
			}
			// Exercise the full-auth save path explicitly, without asynchronous persistence timing.
			if _, errSave := NewRuntimeAdapter(repo, "192.0.2.10").Save(ctx, updated); errSave != nil {
				t.Fatal(errSave)
			}
			reloaded, _, errReload := repo.GetAuth(ctx, seed.ID)
			if errReload != nil {
				t.Fatal(errReload)
			}
			if reloaded.RuntimeRefreshBlocked || reloaded.LastError == nil || reloaded.LastError.Code != code || reloaded.StatusMessage != seed.StatusMessage || !reloaded.NextRetryAfter.Equal(retryAt) || !coreauth.RefreshBlocksDispatch(reloaded) {
				t.Fatalf("refresh fence/diagnostic lost after save and reload: %+v", reloaded)
			}
			state := reloaded.ModelStates["fake-model"]
			if state == nil || state.LastError == nil || state.LastError.Code != coreauth.ErrorCodeStreamTruncated {
				t.Fatal("per-model truncation diagnostic missing")
			}
			selected, errPick := (&coreauth.FillFirstSelector{}).Pick(ctx, "codex", "fake-model", coreauth.Options{}, []*coreauth.Auth{reloaded})
			if errPick == nil || selected != nil {
				t.Fatal("refresh-blocked credential selected after expired retry deadline")
			}
		})
	}
}

func TestUnknownRefreshOutcomeBlocksDispatchAfterReload(t *testing.T) {
	ctx := context.Background()
	repo := newRefreshTestRepository(t)
	auth := &coreauth.Auth{
		ID: "fake-unknown-refresh", Index: "fake-unknown-refresh", Provider: "codex",
		Status: coreauth.StatusError, Unavailable: true, RuntimeRefreshBlocked: true,
		LastError:        &coreauth.Error{Code: coreauth.ErrorCodeRefreshOutcomeUnknown},
		LastRefreshError: &coreauth.Error{Code: coreauth.ErrorCodeRefreshOutcomeUnknown},
		NextRetryAfter:   time.Now().Add(-time.Hour),
		Metadata:         map[string]any{"access_token": "fake-access", "refresh_token": "fake-refresh"},
	}
	if _, errUpsert := repo.UpsertAuth(ctx, auth, "register"); errUpsert != nil {
		t.Fatal(errUpsert)
	}
	stored, record, errGet := repo.GetAuth(ctx, auth.ID)
	if errGet != nil {
		t.Fatal(errGet)
	}
	if stored.RuntimeRefreshBlocked {
		t.Fatal("transient runtime flag unexpectedly persisted")
	}
	if !coreauth.RefreshBlocksDispatch(stored) {
		t.Error("persisted unknown outcome did not block dispatch")
	}
	indexes, errIndex := repo.ListAuthIndex(ctx)
	if errIndex != nil {
		t.Fatal(errIndex)
	}
	if len(indexes) != 1 || !indexes[0].RefreshBlocked {
		t.Fatal("repository index lost refresh fence")
	}
	item := authIndexFromRecord(record, stored)
	if !item.RefreshBlocked {
		t.Fatal("reloaded record index lost refresh fence")
	}
	projected := authFromIndex(item)
	selector := &coreauth.FillFirstSelector{}
	for _, candidate := range []*coreauth.Auth{stored, projected} {
		selected, errPick := selector.Pick(ctx, "codex", "fake-model", coreauth.Options{}, []*coreauth.Auth{candidate})
		if errPick == nil || selected != nil {
			t.Fatal("unknown outcome was selected after reload")
		}
	}
}
