package cluster

import (
	"errors"
	"testing"
	"time"

	coreauth "github.com/router-for-me/CLIProxyAPIHome/internal/cliproxy/auth"
)

func TestEnsurePluginAuthIdentityAssignsStableUUID(t *testing.T) {
	first := &coreauth.Auth{ID: "copilot-octo-user.json", Provider: "copilot"}
	EnsurePluginAuthIdentity(first)
	if !isValidUUID(first.ID) {
		t.Fatalf("ID = %q, want a UUID", first.ID)
	}
	if first.Index != first.ID {
		t.Fatalf("Index = %q, want it to equal ID %q", first.Index, first.ID)
	}
	if first.FileName != "copilot-octo-user.json" {
		t.Fatalf("FileName = %q, want the plugin identifier kept", first.FileName)
	}
	if _, err := AuthToRecord(first); err != nil {
		t.Fatalf("AuthToRecord() error = %v, want the auth to be storable", err)
	}

	again := &coreauth.Auth{ID: "copilot-octo-user.json", Provider: "Copilot"}
	EnsurePluginAuthIdentity(again)
	if again.ID != first.ID {
		t.Fatalf("re-login ID = %q, want the same UUID %q", again.ID, first.ID)
	}

	other := &coreauth.Auth{ID: "copilot-second-user.json", Provider: "copilot"}
	EnsurePluginAuthIdentity(other)
	if other.ID == first.ID {
		t.Fatal("a different account produced the same UUID")
	}
	sameIDOtherProvider := &coreauth.Auth{ID: "copilot-octo-user.json", Provider: "other"}
	EnsurePluginAuthIdentity(sameIDOtherProvider)
	if sameIDOtherProvider.ID == first.ID {
		t.Fatal("a different provider produced the same UUID")
	}
}

func TestEnsurePluginAuthIdentityKeepsExistingUUID(t *testing.T) {
	const existing = "4d810cfd-47c8-420d-8687-05f68428caba"
	auth := &coreauth.Auth{ID: existing, Provider: "copilot", FileName: "copilot-octo-user.json"}
	EnsurePluginAuthIdentity(auth)
	if auth.ID != existing || auth.Index != existing {
		t.Fatalf("ID/Index = %q/%q, want the existing UUID kept", auth.ID, auth.Index)
	}
}

func TestEnsurePluginAuthIdentityUsesFileNameWhenIDMissing(t *testing.T) {
	auth := &coreauth.Auth{Provider: "copilot", FileName: "copilot-octo-user.json"}
	EnsurePluginAuthIdentity(auth)
	withID := &coreauth.Auth{ID: "copilot-octo-user.json", Provider: "copilot"}
	EnsurePluginAuthIdentity(withID)
	if auth.ID != withID.ID || auth.Index != auth.ID {
		t.Fatalf("ID = %q, want %q derived from the file name", auth.ID, withID.ID)
	}
}

func TestEnsurePluginAuthIdentityIgnoresEmptyAuth(t *testing.T) {
	EnsurePluginAuthIdentity(nil)
	auth := &coreauth.Auth{Provider: "copilot"}
	EnsurePluginAuthIdentity(auth)
	if auth.ID != "" || auth.Index != "" {
		t.Fatalf("ID/Index = %q/%q, want untouched when there is no identifier", auth.ID, auth.Index)
	}
}

func TestPluginAuthIdentityAtomicOAuthCompletion(t *testing.T) {
	for _, outcome := range []string{"success", "cancelled"} {
		t.Run(outcome, func(t *testing.T) {
			repo, ctx := newOAuthSessionTestRepository(t)
			session, errSession := NewOAuthSessionRecord("copilot", "plugin-atomic-state", nil, time.Now())
			if errSession != nil {
				t.Fatal(errSession)
			}
			if errUpsert := repo.UpsertOAuthSession(ctx, session); errUpsert != nil {
				t.Fatal(errUpsert)
			}
			auth := &coreauth.Auth{ID: "copilot-octo-user.json", Provider: "copilot", Status: coreauth.StatusActive}
			if outcome == "success" {
				// Plugin identifiers cannot be persisted directly. The rejected write
				// must roll back session completion so normalization can be retried.
				if errComplete := repo.CompleteOAuthSessionWithAuths(ctx, session.State, []*coreauth.Auth{auth}); errComplete == nil {
					t.Fatal("unnormalized plugin identity unexpectedly completed the session")
				}
			} else if cancelled, errCancel := repo.CancelOAuthSession(ctx, session.State); !cancelled || errCancel != nil {
				t.Fatalf("cancel plugin session: %v, %v", cancelled, errCancel)
			}
			EnsurePluginAuthIdentity(auth)
			errComplete := repo.CompleteOAuthSessionWithAuths(ctx, session.State, []*coreauth.Auth{auth})
			if outcome == "success" && errComplete != nil {
				t.Fatal(errComplete)
			}
			if outcome == "cancelled" && !errors.Is(errComplete, ErrOAuthSessionNotPending) {
				t.Fatalf("cancelled plugin session completion error = %v", errComplete)
			}
			stored, errList := repo.ListAuths(ctx)
			if errList != nil {
				t.Fatal(errList)
			}
			if outcome == "cancelled" {
				if len(stored) != 0 {
					t.Fatal("cancelled plugin login persisted credentials")
				}
				return
			}
			if len(stored) != 1 || stored[0].ID != auth.ID || stored[0].FileName != "copilot-octo-user.json" {
				t.Fatalf("stored plugin identities = %#v, want normalized UUID and preserved file name", stored)
			}
			completed, errGet := repo.GetOAuthSession(ctx, session.State)
			if errGet != nil {
				t.Fatal(errGet)
			}
			if completed.Status != "complete" {
				t.Fatalf("plugin session status = %q, want complete", completed.Status)
			}
		})
	}
}
