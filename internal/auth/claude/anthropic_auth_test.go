package claude

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type refreshRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f refreshRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestRefreshTokensPreservesUpstreamBody(t *testing.T) {
	t.Parallel()

	const responseBody = `{
				"error":{"type":"invalid_request_error","message":"Refresh request was rejected"}
			}`
	svc := &ClaudeAuth{httpClient: &http.Client{Transport: refreshRoundTripperFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(responseBody)),
		}, nil
	})}}

	_, errRefresh := svc.RefreshTokens(context.Background(), "test-refresh-token")
	if errRefresh == nil {
		t.Fatal("RefreshTokens() error = nil, want provider response error")
	}
	if got := errRefresh.Error(); got != responseBody {
		t.Fatalf("RefreshTokens() error = %q, want exact body %q", got, responseBody)
	}
}

type refreshErrorReader struct {
	err error
}

func (r *refreshErrorReader) Read([]byte) (int, error) {
	return 0, r.err
}

func (r *refreshErrorReader) Close() error {
	return nil
}

func TestRefreshTokensWithRetry_DoesNotReplayAfterResponseReadError(t *testing.T) {
	var calls int
	auth := &ClaudeAuth{
		httpClient: &http.Client{
			Transport: refreshRoundTripperFunc(func(*http.Request) (*http.Response, error) {
				calls++
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       &refreshErrorReader{err: io.ErrUnexpectedEOF},
					Header:     make(http.Header),
				}, nil
			}),
		},
	}

	_, errRefresh := auth.RefreshTokensWithRetry(context.Background(), "single-use-read-error-token", 3)
	if !errors.Is(errRefresh, io.ErrUnexpectedEOF) {
		t.Fatalf("refresh error = %v, want original response read error", errRefresh)
	}
	if calls != 1 {
		t.Fatalf("expected one refresh attempt after a response read error, got %d", calls)
	}
}

func TestRefreshTokensWithRetry_DoesNotReplayAfterJSONDecodeError(t *testing.T) {
	var calls int
	auth := &ClaudeAuth{
		httpClient: &http.Client{
			Transport: refreshRoundTripperFunc(func(*http.Request) (*http.Response, error) {
				calls++
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`invalid json payload`)),
					Header:     make(http.Header),
				}, nil
			}),
		},
	}

	_, errRefresh := auth.RefreshTokensWithRetry(context.Background(), "single-use-decode-error-token", 3)
	if errRefresh == nil || !strings.Contains(errRefresh.Error(), "failed to parse token response") {
		t.Fatalf("refresh error = %v, want response decoding error", errRefresh)
	}
	if calls != 1 {
		t.Fatalf("expected one refresh attempt after a JSON decode error, got %d", calls)
	}
}

func TestRefreshTokensWithRetry_DoesNotReplayAfterTransportError(t *testing.T) {
	var calls int
	transportErr := errors.New("connection reset by peer")
	auth := &ClaudeAuth{
		httpClient: &http.Client{
			Transport: refreshRoundTripperFunc(func(*http.Request) (*http.Response, error) {
				calls++
				return nil, transportErr
			}),
		},
	}

	_, errRefresh := auth.RefreshTokensWithRetry(context.Background(), "single-use-transport-error-token", 3)
	if !errors.Is(errRefresh, transportErr) {
		t.Fatalf("refresh error = %v, want original transport error", errRefresh)
	}
	if calls != 1 {
		t.Fatalf("expected one refresh attempt after an ambiguous transport error, got %d", calls)
	}
}

func TestRefreshTokensWithRetry_RetriesExplicitServerRejection(t *testing.T) {
	var calls int
	auth := &ClaudeAuth{
		httpClient: &http.Client{
			Transport: refreshRoundTripperFunc(func(*http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					return &http.Response{
						StatusCode: http.StatusServiceUnavailable,
						Body:       io.NopCloser(strings.NewReader(`{"error":"temporarily_unavailable"}`)),
						Header:     make(http.Header),
					}, nil
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"access_token":"new-test-access-token","refresh_token":"new-test-refresh-token","expires_in":3600}`)),
					Header:     make(http.Header),
				}, nil
			}),
		},
	}

	tokenData, errRefresh := auth.RefreshTokensWithRetry(context.Background(), "explicit-rejection-test-token", 3)
	if errRefresh != nil {
		t.Fatalf("refresh after explicit server rejection failed: %v", errRefresh)
	}
	if calls != 2 {
		t.Fatalf("expected two refresh attempts after a server rejection, got %d", calls)
	}
	if tokenData.AccessToken != "new-test-access-token" || tokenData.RefreshToken != "new-test-refresh-token" {
		t.Fatal("refresh did not return the new token pair")
	}
}
