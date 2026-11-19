package codex

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"
)

type retryRoundTripper func(*http.Request) (*http.Response, error)

func (f retryRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestRefreshTokensRetriesRecoverableFailures(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		calls := 0
		auth := &CodexAuth{httpClient: &http.Client{Transport: retryRoundTripper(func(req *http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				return nil, &net.OpError{Op: "dial", Net: "tcp", Err: io.ErrUnexpectedEOF}
			}
			return &http.Response{StatusCode: http.StatusOK, Request: req, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"access_token":"recovered","refresh_token":"rotated","expires_in":3600}`))}, nil
		})}}
		data, errRefresh := auth.RefreshTokensWithRetry(context.Background(), t.Name(), 3)
		if errRefresh != nil || data == nil || data.AccessToken != "recovered" || data.RefreshToken != "rotated" || calls != 2 {
			t.Fatalf("pre-submission retry did not recover: error=%v calls=%d", errRefresh, calls)
		}
	})
}

func TestRefreshTokensRetryExhaustionDoesNotFenceUnsubmittedGrant(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		calls := 0
		transport := retryRoundTripper(func(req *http.Request) (*http.Response, error) {
			calls++
			if calls <= 3 {
				return nil, &net.OpError{Op: "dial", Net: "tcp", Err: io.ErrUnexpectedEOF}
			}
			return &http.Response{StatusCode: http.StatusOK, Request: req, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"access_token":"recovered","expires_in":3600}`))}, nil
		})
		client := &CodexAuth{httpClient: &http.Client{Transport: transport}}
		if _, errRefresh := client.RefreshTokensWithRetry(context.Background(), t.Name(), 3); errRefresh == nil || calls != 3 || IsRefreshOutcomeUnknown(errRefresh) {
			t.Fatalf("unsubmitted retry limit not honored: error=%v calls=%d", errRefresh, calls)
		}
		data, errRefresh := client.RefreshTokensWithRetry(context.Background(), t.Name(), 3)
		if errRefresh != nil || data == nil || data.AccessToken != "recovered" || calls != 4 {
			t.Fatalf("unsubmitted grant remained fenced: error=%v calls=%d", errRefresh, calls)
		}
	})
}

func TestRefreshTokensStopsOnRejectedGrantOrCancellation(t *testing.T) {
	for _, code := range []string{"invalid_grant", "refresh_token_expired", "refresh_token_revoked", "refresh_token_reused"} {
		t.Run(code, func(t *testing.T) {
			calls := 0
			auth := &CodexAuth{httpClient: &http.Client{Transport: retryRoundTripper(func(req *http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: http.StatusUnauthorized, Request: req, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"error":"` + code + `"}`))}, nil
			})}}
			if _, errRefresh := auth.RefreshTokensWithRetry(context.Background(), t.Name(), 3); errRefresh == nil || calls != 1 {
				t.Fatalf("grant rejection retried: error=%v calls=%d", errRefresh, calls)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	auth := &CodexAuth{httpClient: &http.Client{Transport: retryRoundTripper(func(*http.Request) (*http.Response, error) {
		t.Fatal("canceled refresh was submitted")
		return nil, nil
	})}}
	if _, errRefresh := auth.RefreshTokensWithRetry(ctx, "unused", 3); !errors.Is(errRefresh, context.Canceled) {
		t.Fatalf("cancellation lost: %v", errRefresh)
	}
}
