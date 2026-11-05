package cluster

import (
	"reflect"
	"testing"
	"time"

	coreauth "github.com/router-for-me/CLIProxyAPIHome/internal/cliproxy/auth"
)

func TestOAuthReconnectPreservesIdentityRoutingAndHistory(t *testing.T) {
	for _, scenario := range []struct{ provider, oldPlan, newPlan string }{
		{"claude", "free", "promax"}, {"codex", "free", "promax"},
		{"codex", "promax", "free"}, {"codex", "promax", ""},
	} {
		t.Run(scenario.provider+"-"+scenario.oldPlan+"-"+scenario.newPlan, func(t *testing.T) {
			provider := scenario.provider
			repo, ctx := newOAuthSessionTestRepository(t)
			id := "11111111-1111-4111-8111-111111111111"
			original := &coreauth.Auth{ID: id, Index: id, Provider: provider, FileName: "original.json", Label: "Original",
				Prefix: "keep", ProxyURL: "http://proxy.test", Disabled: true, Status: coreauth.StatusDisabled,
				StatusMessage: "unauthorized", LastError: &coreauth.Error{Code: "unauthorized", HTTPStatus: 401},
				LastRefreshError: &coreauth.Error{Code: "authentication_error", HTTPStatus: 401},
				CreatedAt:        time.Now().UTC().Add(-time.Hour),
				Attributes:       map[string]string{"source": "oauth:original", refreshLeaseAttribute: "stale-lease", "note": "keep", "plan_type": scenario.oldPlan},
				Metadata: map[string]any{"uuid": id, "type": provider, "email": "owner@example.com", "account_id": "workspace",
					"access_token": "old", "refresh_token": "old", "priority": 10, "filename": "original.json"}}
			if _, errUpsert := repo.UpsertAuth(ctx, original, "create"); errUpsert != nil {
				t.Fatal(errUpsert)
			}
			before, _, errGet := repo.GetAuth(ctx, id)
			if errGet != nil {
				t.Fatal(errGet)
			}
			db, errDB := repo.database()
			if errDB != nil {
				t.Fatal(errDB)
			}
			binding := ChannelGroupDetailRecord{ChannelGroupID: 7, AuthID: id}
			if errCreate := db.Create(&binding).Error; errCreate != nil {
				t.Fatal(errCreate)
			}
			usage := UsageRecord{AuthIndex: id, Provider: provider, PayloadJSON: JSONB(`{}`)}
			if errCreate := db.Create(&usage).Error; errCreate != nil {
				t.Fatal(errCreate)
			}
			session, errSession := NewOAuthSessionRecord(provider, "reconnect", map[string]any{
				"reconnect_credential_id": id, "reconnect_email": "owner@example.com", "reconnect_account_id": "workspace"}, time.Now())
			if errSession != nil {
				t.Fatal(errSession)
			}
			if errStore := repo.UpsertOAuthSession(ctx, session); errStore != nil {
				t.Fatal(errStore)
			}
			fresh := &coreauth.Auth{ID: "22222222-2222-4222-8222-222222222222", Provider: provider, Attributes: map[string]string{"plan_type": scenario.newPlan}, Metadata: map[string]any{
				"type": provider, "email": "owner@example.com", "account_id": "workspace", "access_token": "fresh-access",
				"refresh_token": "fresh-refresh", "id_token": "fresh-id", "last_refresh": time.Now().Format(time.RFC3339),
				"expired": time.Now().Add(time.Hour).Format(time.RFC3339), "priority": 999}}
			if errComplete := repo.CompleteOAuthSessionWithAuths(ctx, session.State, []*coreauth.Auth{fresh}); errComplete != nil {
				t.Fatal(errComplete)
			}
			after, _, errAfter := repo.GetAuth(ctx, id)
			if errAfter != nil {
				t.Fatal(errAfter)
			}
			if after.ID != id || after.FileName != before.FileName || after.Label != before.Label || after.Prefix != before.Prefix ||
				after.ProxyURL != before.ProxyURL || !after.CreatedAt.Equal(before.CreatedAt) ||
				!reflect.DeepEqual(after.Metadata["priority"], before.Metadata["priority"]) {
				t.Fatal("reconnect changed credential identity or routing")
			}
			if after.Metadata["access_token"] != "fresh-access" || after.Disabled || after.LastRefreshError != nil || after.LastError != nil || after.Unavailable {
				t.Fatal("reconnect did not restore authorization")
			}
			if after.Attributes[refreshLeaseAttribute] != "" {
				t.Fatal("reconnect retained stale refresh lease")
			}
			wantPlan := scenario.oldPlan
			if provider == "codex" {
				wantPlan = scenario.newPlan
			}
			if after.Attributes["plan_type"] != wantPlan || after.Attributes["note"] != "keep" {
				t.Fatal("reconnect failed to update the account plan while preserving routing attributes")
			}
			var savedBinding ChannelGroupDetailRecord
			if errBinding := db.First(&savedBinding, binding.ID).Error; errBinding != nil || savedBinding.AuthID != id {
				t.Fatal("binding changed")
			}
			var savedUsage UsageRecord
			if errUsage := db.First(&savedUsage, usage.ID).Error; errUsage != nil || savedUsage.AuthIndex != id {
				t.Fatal("history changed")
			}
			auths, errList := repo.ListAuths(ctx)
			if errList != nil || len(auths) != 1 {
				t.Fatal("reconnect created a duplicate")
			}
			if errReplay := repo.CompleteOAuthSessionWithAuths(ctx, session.State, []*coreauth.Auth{fresh}); errReplay == nil {
				t.Fatal("completed session replay succeeded")
			}
		})
	}
}

func TestOAuthReconnectRejectsWrongAccountCancellationAndDeletedTarget(t *testing.T) {
	for _, scenario := range []string{"wrong-email", "wrong-workspace", "cancelled", "expired-session", "deleted", "changed-identity", "invalid-tokens"} {
		t.Run(scenario, func(t *testing.T) {
			repo, ctx := newOAuthSessionTestRepository(t)
			id := "11111111-1111-4111-8111-111111111111"
			auth := &coreauth.Auth{ID: id, Index: id, Provider: "codex", Metadata: map[string]any{"type": "codex", "email": "owner@example.com", "account_id": "workspace", "access_token": "old"}}
			if _, errUpsert := repo.UpsertAuth(ctx, auth, "create"); errUpsert != nil {
				t.Fatal(errUpsert)
			}
			session, errSession := NewOAuthSessionRecord("codex", "reject", map[string]any{"reconnect_credential_id": id, "reconnect_email": "owner@example.com", "reconnect_account_id": "workspace"}, time.Now())
			if errSession != nil {
				t.Fatal(errSession)
			}
			if scenario == "expired-session" {
				session.ExpiresAt = time.Now().Add(-time.Hour)
			}
			if errStore := repo.UpsertOAuthSession(ctx, session); errStore != nil {
				t.Fatal(errStore)
			}
			fresh := &coreauth.Auth{Provider: "codex", Metadata: map[string]any{"email": "owner@example.com", "account_id": "workspace", "access_token": "fresh", "refresh_token": "fresh", "id_token": "fresh", "expired": time.Now().Add(time.Hour).Format(time.RFC3339)}}
			switch scenario {
			case "wrong-email":
				fresh.Metadata["email"] = "someone-else@example.com"
			case "wrong-workspace":
				fresh.Metadata["account_id"] = "other-workspace"
			case "invalid-tokens":
				fresh.Metadata["refresh_token"] = ""
			case "cancelled":
				if _, errCancel := repo.CancelOAuthSession(ctx, session.State); errCancel != nil {
					t.Fatal(errCancel)
				}
			case "deleted":
				if errDelete := repo.SoftDeleteAuth(ctx, id); errDelete != nil {
					t.Fatal(errDelete)
				}
			case "changed-identity":
				if _, _, _, errMutate := repo.MutateAuth(ctx, id, "update", func(a *coreauth.Auth) bool { a.Metadata["email"] = "changed@example.com"; return true }); errMutate != nil {
					t.Fatal(errMutate)
				}
			}
			if errComplete := repo.CompleteOAuthSessionWithAuths(ctx, session.State, []*coreauth.Auth{fresh}); errComplete == nil {
				t.Fatal("unsafe reconnect succeeded")
			}
			if scenario != "deleted" {
				after, _, errAfter := repo.GetAuth(ctx, id)
				if errAfter != nil || after.Metadata["access_token"] != "old" {
					t.Fatal("rejected reconnect changed tokens")
				}
			}
		})
	}
}
