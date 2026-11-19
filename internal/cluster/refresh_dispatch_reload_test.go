package cluster

import (
	"context"
	"testing"
	"time"

	coreauth "github.com/router-for-me/CLIProxyAPIHome/internal/cliproxy/auth"
)

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
