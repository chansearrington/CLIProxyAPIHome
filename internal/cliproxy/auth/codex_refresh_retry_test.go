package auth

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	codexauth "github.com/router-for-me/CLIProxyAPIHome/internal/auth/codex"
)

func TestCodexRefreshFailureRemainsRetryable(t *testing.T) {
	now := time.Now()
	auth := &Auth{Provider: "codex", Status: StatusActive, Metadata: map[string]any{"refresh_token": "same-refresh"}}
	failure := ApplyRefreshFailureState(auth, io.ErrUnexpectedEOF, now)
	if failure == nil || failure.Code != refreshTransientErrorCode || !failure.Retryable || auth.Disabled || !auth.NextRefreshAfter.After(now) {
		t.Fatal("temporary refresh failure became permanent")
	}
	if _, exists := auth.Metadata[codexauth.RefreshOutcomeMetadataKey]; exists {
		t.Fatal("unclassified failure created a persistent grant fence")
	}
}

func TestCodexRefreshPreservesGrantMatchedOutcomeFence(t *testing.T) {
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	http.DefaultTransport = refreshTestRoundTripper(func(*http.Request) (*http.Response, error) {
		t.Fatal("fenced grant was submitted")
		return nil, nil
	})
	auth := &Auth{Provider: "codex", Status: StatusActive, Metadata: map[string]any{
		"access_token": "previous", "refresh_token": t.Name(),
	}}
	codexauth.RecordRefreshOutcomeUnknown(auth.Metadata, codexauth.NewRefreshOutcomeUnknownError(codexauth.ClientID, t.Name(), "response_read", io.ErrUnexpectedEOF))
	updated, errRefresh := refreshCredential(context.Background(), nil, auth, nil)
	if updated != nil || !codexauth.IsRefreshOutcomeUnknown(errRefresh) {
		t.Fatalf("grant fence was not retained: %v", errRefresh)
	}
}
