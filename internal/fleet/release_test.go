package fleet

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

type releaseTransport func(*http.Request) (*http.Response, error)

func (f releaseTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestReleaseHTTPRateLimitCauseAndReset(t *testing.T) {
	reset := time.Date(2026, 11, 18, 8, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		status  int
		headers http.Header
		code    string
		after   time.Duration
	}{
		{403, http.Header{"X-Ratelimit-Remaining": {"0"}, "X-Ratelimit-Reset": {strconv.FormatInt(reset.Unix(), 10)}}, "rate_limited", 0},
		{429, http.Header{"Retry-After": {"7200"}}, "rate_limited", 2 * time.Hour},
		{403, http.Header{}, "release_http_403", 0},
		{503, http.Header{"Retry-After": {"7200"}}, "release_http_503", 2 * time.Hour},
	} {
		client := &http.Client{Transport: releaseTransport(func(r *http.Request) (*http.Response, error) {
			if r.Header.Get("Authorization") != "" {
				t.Error("release check added a token requirement")
			}
			return &http.Response{StatusCode: tc.status, Header: tc.headers, Body: io.NopCloser(strings.NewReader("untrusted response not logged"))}, nil
		})}
		_, errFetch := fetchStableRelease(context.Background(), client, nil)
		var failure *ReleaseCheckError
		if !errors.As(errFetch, &failure) || failure.Code != tc.code || failure.RetryAfter != tc.after {
			t.Fatalf("bad failure: %+v", errFetch)
		}
		if tc.status == 403 && tc.code == "rate_limited" && !failure.RetryAt.Equal(reset.Add(time.Minute)) {
			t.Fatal("missing rate-limit reset")
		}
	}
}

func TestReleaseChecksContinueWhilePausedAndRespectResetAcrossPolicyChanges(t *testing.T) {
	s, store := fixture(t)
	ctx := context.Background()
	st, _ := s.Status(ctx)
	st.Policy.PatchUpdates = false
	if errPolicy := s.SetPolicy(ctx, st.Policy); errPolicy != nil {
		t.Fatal(errPolicy)
	}
	reset := store.now.Add(3 * time.Hour)
	calls := 0
	s.FetchRelease = func(context.Context) (Release, error) {
		calls++
		return Release{}, &ReleaseCheckError{Code: "rate_limited", RetryAt: reset}
	}
	if errCheck := s.Check(ctx); errCheck != nil {
		t.Fatal(errCheck)
	}
	st, _ = s.Status(ctx)
	if calls != 1 || st.CheckError != "rate_limited" || !st.CheckRetryAt.Equal(reset) {
		t.Fatalf("paused check hid real cause/reset: %+v calls %d", st, calls)
	}
	// A policy write forces ordinary checks but must not defeat GitHub's backoff.
	if errPolicy := s.SetPolicy(ctx, st.Policy); errPolicy != nil {
		t.Fatal(errPolicy)
	}
	store.now = reset.Add(-time.Second)
	if errCheck := s.Check(ctx); errCheck != nil {
		t.Fatal(errCheck)
	}
	if calls != 1 {
		t.Fatal("rate-limited API was retried early")
	}
	store.now = reset
	s.FetchRelease = func(context.Context) (Release, error) { calls++; return candidate("8.0.31"), nil }
	if errCheck := s.Check(ctx); errCheck != nil {
		t.Fatal(errCheck)
	}
	st, _ = s.Status(ctx)
	if calls != 2 || st.Latest == nil || st.Latest.Version != "8.0.31" || st.CheckError != "" || !st.CheckRetryAt.IsZero() || st.Rollout != nil {
		t.Fatal("paused status did not refresh cleanly")
	}
	store.now = store.now.Add(time.Hour)
	if errCheck := s.Check(ctx); errCheck != nil {
		t.Fatal(errCheck)
	}
	if calls != 3 {
		t.Fatal("paused release status became stale")
	}
}
