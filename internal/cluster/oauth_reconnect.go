package cluster

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	coreauth "github.com/router-for-me/CLIProxyAPIHome/internal/cliproxy/auth"
)

var ErrOAuthReconnectIdentity = errors.New("sign-in account does not match the selected credential; sign in with the original account")
var ErrOAuthReconnectTargetChanged = errors.New("selected credential identity changed during sign-in")

func reconnectMetadataString(metadata map[string]any, key string) string {
	value, _ := metadata[key].(string)
	return strings.TrimSpace(value)
}

// completeOAuthReconnect runs inside the session completion transaction. It changes only
// token material on the locked current record, preserving identity, routing and history.
func (r *Repository) completeOAuthReconnect(ctx context.Context, session *OAuthSessionRecord, data map[string]any, fresh *coreauth.Auth) error {
	provider := session.Provider
	if provider == "anthropic" {
		provider = "claude"
	}
	if fresh == nil || fresh.Provider != provider || (provider != "claude" && provider != "codex") {
		return fmt.Errorf("reconnect provider mismatch")
	}
	email := reconnectMetadataString(data, "reconnect_email")
	accountID := reconnectMetadataString(data, "reconnect_account_id")
	if email == "" || !strings.EqualFold(email, reconnectMetadataString(fresh.Metadata, "email")) ||
		(provider == "codex" && (accountID == "" || accountID != reconnectMetadataString(fresh.Metadata, "account_id"))) {
		return ErrOAuthReconnectIdentity
	}
	now := time.Now().UTC()
	expiry, errExpiry := time.Parse(time.RFC3339, reconnectMetadataString(fresh.Metadata, "expired"))
	if errExpiry != nil || !expiry.After(now) || reconnectMetadataString(fresh.Metadata, "access_token") == "" ||
		reconnectMetadataString(fresh.Metadata, "refresh_token") == "" ||
		(provider == "codex" && reconnectMetadataString(fresh.Metadata, "id_token") == "") {
		return fmt.Errorf("reconnect did not return valid OAuth tokens")
	}
	var identityMismatch bool
	_, _, _, errMutate := r.MutateAuth(ctx, reconnectMetadataString(data, "reconnect_credential_id"), "reconnect", func(current *coreauth.Auth) bool {
		if current.Provider != provider || !strings.EqualFold(email, reconnectMetadataString(current.Metadata, "email")) ||
			(provider == "codex" && accountID != reconnectMetadataString(current.Metadata, "account_id")) ||
			reconnectMetadataString(current.Metadata, "type") != provider {
			identityMismatch = true
			return false
		}
		keys := []string{"access_token", "refresh_token", "expired", "last_refresh"}
		if provider == "codex" {
			keys = append(keys, "id_token")
		}
		for _, key := range keys {
			current.Metadata[key] = fresh.Metadata[key]
		}
		coreauth.ApplyReauthenticationSuccessState(current, now)
		clearRefreshLease(current)
		current.Metadata["disabled"] = false
		return true
	})
	if errMutate != nil {
		return errMutate
	}
	if identityMismatch {
		return ErrOAuthReconnectTargetChanged
	}
	return nil
}
