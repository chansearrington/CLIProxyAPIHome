package management

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	coreauth "github.com/router-for-me/CLIProxyAPIHome/internal/cliproxy/auth"
	"github.com/router-for-me/CLIProxyAPIHome/internal/cluster"
)

func TestOAuthReconnectStartBindsExistingCredential(t *testing.T) {
	db, cleanup := openManagementLogTestDB(t)
	defer cleanup()
	repo := cluster.NewRepository(db)
	id := "11111111-1111-4111-8111-111111111111"
	auth := &coreauth.Auth{ID: id, Index: id, Provider: "claude", Disabled: true, StatusMessage: "unauthorized", Metadata: map[string]any{"type": "claude", "email": "owner@example.com"}}
	if _, errUpsert := repo.UpsertAuth(context.Background(), auth, "create"); errUpsert != nil {
		t.Fatal(errUpsert)
	}
	h := NewHandler(repo, nil, "127.0.0.1", 0)
	router := gin.New()
	router.GET("/oauth/auth-url", h.StartOAuthV8)
	for _, tc := range []struct {
		query  string
		status int
	}{
		{"provider=claude&credential_id=" + id, 200},
		{"provider=codex&credential_id=" + id, 400},
		{"provider=antigravity&credential_id=" + id, 400},
		{"provider=claude&credential_id=missing", 404},
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("GET", "/oauth/auth-url?"+tc.query, nil))
		if response.Code != tc.status {
			t.Fatalf("%s: got %d want %d", tc.query, response.Code, tc.status)
		}
		if tc.status == 200 {
			var started struct{ State string }
			if errDecode := json.Unmarshal(response.Body.Bytes(), &started); errDecode != nil {
				t.Fatal(errDecode)
			}
			session, errSession := repo.GetOAuthSession(context.Background(), started.State)
			if errSession != nil {
				t.Fatal(errSession)
			}
			data, errData := cluster.OAuthSessionData(session)
			if errData != nil || data["reconnect_credential_id"] != id || data["reconnect_email"] != "owner@example.com" {
				t.Fatal("reconnect target not bound to session")
			}
		}
	}
	entry := authFileEntry(auth)
	if entry["reconnect_supported"] != true || entry["reauth_required"] != true {
		t.Fatal("missing reconnect capability or sign-in status")
	}
}
