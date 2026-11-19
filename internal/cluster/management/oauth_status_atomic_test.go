package management

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	coreauth "github.com/router-for-me/CLIProxyAPIHome/internal/cliproxy/auth"
	"github.com/router-for-me/CLIProxyAPIHome/internal/cluster"
	"gorm.io/gorm"
)

func TestPatchAuthFileStatusPreservesInterleavedRefresh(t *testing.T) {
	handler, engine, db, closeRepo := newConcurrencyManagementTestServer(t)
	defer closeRepo()
	ctx := context.Background()
	const id = "22222222-2222-4222-8222-222222222222"
	auth := &coreauth.Auth{
		ID: id, Index: id, Provider: "claude", Status: coreauth.StatusActive,
		Metadata: map[string]any{"type": "claude", "email": "fixture@example.com",
			"access_token": "fake-old-access", "refresh_token": "fake-old-refresh"},
	}
	if _, errUpsert := handler.repo.UpsertAuth(ctx, auth, "fixture"); errUpsert != nil {
		t.Fatal(errUpsert)
	}
	engine.PATCH("/auth-files/status", handler.PatchAuthFileStatus)
	interleaved := false
	// A refresh lands after the handler's lookup and before its write.
	const hook = "test:status-after-selector-read"
	if errRegister := db.Callback().Query().After("gorm:query").Register(hook, func(tx *gorm.DB) {
		if _, ok := tx.Statement.Dest.(*[]cluster.AuthRecord); !ok || interleaved {
			return
		}
		interleaved = true
		if _, _, _, errMutate := handler.repo.MutateAuth(ctx, id, "refresh", func(latest *coreauth.Auth) bool {
			latest.Metadata["access_token"] = "fake-new-access"
			latest.Metadata["refresh_token"] = "fake-new-refresh"
			return true
		}); errMutate != nil {
			t.Fatal(errMutate)
		}
	}); errRegister != nil {
		t.Fatal(errRegister)
	}
	t.Cleanup(func() {
		if errRemove := db.Callback().Query().Remove(hook); errRemove != nil {
			t.Error(errRemove)
		}
	})
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodPatch, "/auth-files/status", strings.NewReader(`{"name":"`+id+`","disabled":true}`)))
	if response.Code != http.StatusOK || !interleaved {
		t.Fatalf("status = %d, interleaved = %t", response.Code, interleaved)
	}
	after, _, errGet := handler.repo.GetAuth(ctx, id)
	if errGet != nil {
		t.Fatal(errGet)
	}
	if !after.Disabled || after.Status != coreauth.StatusDisabled {
		t.Fatalf("disabled = %t, status = %s", after.Disabled, after.Status)
	}
	if after.Metadata["refresh_token"] != "fake-new-refresh" || after.Metadata["access_token"] != "fake-new-access" {
		t.Fatal("status save restored the stale sign-in")
	}
}
