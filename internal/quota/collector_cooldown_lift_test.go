package quota

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	coreauth "github.com/router-for-me/CLIProxyAPIHome/internal/cliproxy/auth"
	"github.com/router-for-me/CLIProxyAPIHome/internal/cluster"
)

type recordingCooldownLifter struct {
	mu    sync.Mutex
	calls []recordedCooldownLift
}

type recordedCooldownLift struct {
	credentialID string
	observedAt   time.Time
}

func (l *recordingCooldownLifter) LiftQuotaCooldownObservedAt(_ context.Context, credentialID string, observedAt time.Time) (coreauth.CooldownClearResult, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls = append(l.calls, recordedCooldownLift{credentialID: credentialID, observedAt: observedAt})
	return coreauth.CooldownClearResult{CredentialID: credentialID, Cleared: true, ClearedModels: []string{"gpt-6.1-sol"}}, nil
}

func (l *recordingCooldownLifter) recorded() []recordedCooldownLift {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]recordedCooldownLift(nil), l.calls...)
}

// seedCollectorBlockedAuth stores a Codex credential with a 429 quota block and
// no usage activity, as a blocked credential that receives no traffic looks.
func seedCollectorBlockedAuth(t *testing.T, repo *cluster.Repository, id string, blockedAt time.Time, recoverAt time.Time) {
	t.Helper()
	auth := &coreauth.Auth{
		ID: id, Index: id, Provider: "codex", Label: id, Status: coreauth.StatusError,
		Metadata: map[string]any{"type": "codex", "access_token": "probe-secret"}, CreatedAt: blockedAt, UpdatedAt: blockedAt,
		ModelStates: map[string]*coreauth.ModelState{
			"gpt-6.1-sol": {
				Status: coreauth.StatusError, Unavailable: true, NextRetryAfter: recoverAt,
				LastError: &coreauth.Error{Message: "usage limit reached", HTTPStatus: http.StatusTooManyRequests},
				Quota:     coreauth.QuotaState{Exceeded: true, Scope: "model", Reason: "quota", NextRecoverAt: recoverAt},
				UpdatedAt: blockedAt,
			},
		},
	}
	if _, errUpsert := repo.UpsertAuth(context.Background(), auth, "test"); errUpsert != nil {
		t.Fatalf("UpsertAuth() error = %v", errUpsert)
	}
}

func codexUsageServer(t *testing.T, primaryUsed int, secondaryUsed int) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Errorf("quota probe method = %s, want GET", request.Method)
		}
		switch request.URL.Path {
		case "/usage":
			_, _ = w.Write([]byte(`{"plan_type":"pro","rate_limit":{` +
				`"primary_window":{"used_percent":` + strconv.Itoa(primaryUsed) + `,"limit_window_seconds":18000,"reset_at":1792771200},` +
				`"secondary_window":{"used_percent":` + strconv.Itoa(secondaryUsed) + `,"limit_window_seconds":604800,"reset_at":1793318400}}}`))
		case "/resets":
			_, _ = w.Write([]byte(`{"available_count":0,"credits":[]}`))
		default:
			http.NotFound(w, request)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestCollectorHealthyCheckLiftsStaleQuotaCooldown(t *testing.T) {
	repo := newCollectorTestRepository(t)
	now := time.Date(2026, 10, 23, 14, 19, 0, 0, time.UTC)
	blockedAt := time.Date(2026, 10, 19, 8, 0, 0, 0, time.UTC)
	seedCollectorBlockedAuth(t, repo, "codex-stale-block", blockedAt, time.Date(2026, 10, 25, 8, 0, 0, 0, time.UTC))
	server := codexUsageServer(t, 0, 0)
	lifter := &recordingCooldownLifter{}

	collector := NewCollector(repo, Options{
		Owner: "home-a", HomeID: "home-a", CodexUsageURL: server.URL + "/usage", CodexResetCreditsURL: server.URL + "/resets",
		Now: func() time.Time { return now }, CooldownLifter: lifter,
	})
	// No usage activity is recorded: the blocked credential is claimed only
	// because it has an active quota block.
	collector.collect(context.Background())

	calls := lifter.recorded()
	if len(calls) != 1 || calls[0].credentialID != "codex-stale-block" || !calls[0].observedAt.Equal(now) {
		t.Fatalf("lift calls = %+v, want one lift for codex-stale-block observed at %s", calls, now)
	}
	item, errGet := repo.GetQuotaCredential(context.Background(), "codex-stale-block", now)
	if errGet != nil {
		t.Fatalf("GetQuotaCredential() error = %v", errGet)
	}
	if item.CollectionStatus != "success" {
		t.Fatalf("snapshot collection status = %q, want success", item.CollectionStatus)
	}
}

func TestCollectorExhaustedCheckKeepsQuotaCooldown(t *testing.T) {
	repo := newCollectorTestRepository(t)
	now := time.Date(2026, 10, 23, 14, 19, 0, 0, time.UTC)
	blockedAt := time.Date(2026, 10, 19, 8, 0, 0, 0, time.UTC)
	seedCollectorBlockedAuth(t, repo, "codex-still-exhausted", blockedAt, time.Date(2026, 10, 25, 8, 0, 0, 0, time.UTC))
	server := codexUsageServer(t, 20, 100)
	lifter := &recordingCooldownLifter{}

	collector := NewCollector(repo, Options{
		Owner: "home-a", HomeID: "home-a", CodexUsageURL: server.URL + "/usage", CodexResetCreditsURL: server.URL + "/resets",
		Now: func() time.Time { return now }, CooldownLifter: lifter,
	})
	collector.collect(context.Background())

	if calls := lifter.recorded(); len(calls) != 0 {
		t.Fatalf("lift calls = %+v, want none for an exhausted check", calls)
	}
	item, errGet := repo.GetQuotaCredential(context.Background(), "codex-still-exhausted", now)
	if errGet != nil {
		t.Fatalf("GetQuotaCredential() error = %v", errGet)
	}
	if item.CollectionStatus != "success" || item.QuotaStatus != "exhausted" {
		t.Fatalf("snapshot = status %q collection %q, want exhausted/success", item.QuotaStatus, item.CollectionStatus)
	}
}

func TestCollectorHealthyCheckSkipsLiftForUnblockedCredential(t *testing.T) {
	repo := newCollectorTestRepository(t)
	now := time.Now().UTC()
	seedCollectorAuth(t, repo, "codex-unblocked", map[string]any{"type": "codex", "access_token": "probe-secret"})
	server := codexUsageServer(t, 10, 10)
	lifter := &recordingCooldownLifter{}

	collector := NewCollector(repo, Options{
		Owner: "home-a", HomeID: "home-a", CodexUsageURL: server.URL + "/usage", CodexResetCreditsURL: server.URL + "/resets",
		Now: func() time.Time { return now }, CooldownLifter: lifter,
	})
	collector.collect(context.Background())

	if calls := lifter.recorded(); len(calls) != 0 {
		t.Fatalf("lift calls = %+v, want none for an unblocked credential", calls)
	}
	// seedCollectorAuth records recent usage, so the healthy check really ran.
	item, errGet := repo.GetQuotaCredential(context.Background(), "codex-unblocked", now)
	if errGet != nil {
		t.Fatalf("GetQuotaCredential() error = %v", errGet)
	}
	if item.CollectionStatus != "success" || item.QuotaStatus != "healthy" {
		t.Fatalf("snapshot = status %q collection %q, want healthy/success", item.QuotaStatus, item.CollectionStatus)
	}
}

func TestQuotaWindowsNotExhausted(t *testing.T) {
	zero, half, halfRatio := 0.0, 50.0, 0.5
	tests := []struct {
		name    string
		windows []cluster.QuotaWindow
		want    bool
	}{
		{name: "no windows"},
		{name: "healthy", windows: []cluster.QuotaWindow{{Status: "healthy", Remaining: &half}}, want: true},
		{name: "low", windows: []cluster.QuotaWindow{{Status: "low"}}, want: true},
		{name: "one exhausted", windows: []cluster.QuotaWindow{{Status: "healthy", Remaining: &half}, {Status: "exhausted", Remaining: &zero}}},
		{name: "error window", windows: []cluster.QuotaWindow{{Status: "healthy"}, {Status: "error"}}},
		{name: "unknown with nothing remaining", windows: []cluster.QuotaWindow{{Status: "unknown", Remaining: &zero}}},
		{name: "unknown with remaining", windows: []cluster.QuotaWindow{{Status: "unknown", Remaining: &half}}, want: true},
		{name: "unknown only", windows: []cluster.QuotaWindow{{Status: "unknown"}}},
		{name: "unmeasured window beside healthy", windows: []cluster.QuotaWindow{{Status: "healthy", Remaining: &half}, {Status: "unknown"}}},
		{name: "unmeasured window beside unlimited", windows: []cluster.QuotaWindow{{Status: "unknown", IsUnlimited: true}, {Status: "unknown"}}},
		{name: "remaining ratio only", windows: []cluster.QuotaWindow{{Status: "unknown", RemainingRatio: &halfRatio}}, want: true},
		{name: "remaining ratio zero", windows: []cluster.QuotaWindow{{Status: "unknown", RemainingRatio: &zero}}},
		{name: "unlimited", windows: []cluster.QuotaWindow{{Status: "unknown", IsUnlimited: true, Remaining: &zero}}, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := quotaWindowsNotExhausted(test.windows); got != test.want {
				t.Fatalf("quotaWindowsNotExhausted() = %v, want %v", got, test.want)
			}
		})
	}
}

// repoMutatorStore backs a real coreauth.Manager with the test database row,
// so the collector drives the production lift rule instead of a fake.
type repoMutatorStore struct {
	repo *cluster.Repository
}

func (s *repoMutatorStore) List(context.Context) ([]*coreauth.Auth, error) { return nil, nil }

func (s *repoMutatorStore) Save(_ context.Context, auth *coreauth.Auth) (string, error) {
	if auth == nil {
		return "", nil
	}
	return auth.ID, nil
}

func (s *repoMutatorStore) Delete(context.Context, string) error { return nil }

func (s *repoMutatorStore) MutateAuthState(ctx context.Context, id string, mutate func(auth *coreauth.Auth) bool) (*coreauth.Auth, error) {
	auth, _, _, errMutate := s.repo.MutateAuth(ctx, id, "update", mutate)
	return auth, errMutate
}

// advancingClock returns a later instant on every call, so the probe start,
// anything recorded during the probe and the probe end are all distinct.
type advancingClock struct {
	mu   sync.Mutex
	next time.Time
}

func (c *advancingClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.next
	c.next = c.next.Add(time.Minute)
	return now
}

func TestCollectorLiftKeepsQuotaBlockRecordedDuringProbe(t *testing.T) {
	const authID = "codex-block-during-probe"
	repo := newCollectorTestRepository(t)
	base := time.Now().UTC().Truncate(time.Second)
	clock := &advancingClock{next: base}
	recoverAt := base.Add(48 * time.Hour)
	seedCollectorBlockedAuth(t, repo, authID, base.Add(-4*24*time.Hour), recoverAt)

	seeded, _, errGet := repo.GetAuth(context.Background(), authID)
	if errGet != nil || seeded == nil {
		t.Fatalf("GetAuth() = %v, %v", seeded, errGet)
	}
	manager := coreauth.NewManager(&repoMutatorStore{repo: repo}, nil, nil)
	if _, errRegister := manager.Register(context.Background(), seeded); errRegister != nil {
		t.Fatalf("Register() error = %v", errRegister)
	}

	// While the usage request is in flight, another model on the credential
	// gets a 429. The check cannot speak for that newer block.
	var duringProbe time.Time
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/usage":
			duringProbe = clock.Now()
			_, _, _, errMutate := repo.MutateAuth(request.Context(), authID, "update", func(auth *coreauth.Auth) bool {
				auth.ModelStates["gpt-6.1-codex"] = &coreauth.ModelState{
					Status: coreauth.StatusError, Unavailable: true, NextRetryAfter: recoverAt,
					LastError: &coreauth.Error{Message: "usage limit reached", HTTPStatus: http.StatusTooManyRequests},
					Quota:     coreauth.QuotaState{Exceeded: true, Scope: "model", Reason: "quota", NextRecoverAt: recoverAt},
					UpdatedAt: duringProbe,
				}
				return true
			})
			if errMutate != nil {
				t.Errorf("record in-flight 429: %v", errMutate)
			}
			_, _ = w.Write([]byte(`{"plan_type":"pro","rate_limit":{` +
				`"primary_window":{"used_percent":0,"limit_window_seconds":18000,"reset_at":1792771200},` +
				`"secondary_window":{"used_percent":0,"limit_window_seconds":604800,"reset_at":1793318400}}}`))
		case "/resets":
			_, _ = w.Write([]byte(`{"available_count":0,"credits":[]}`))
		default:
			http.NotFound(w, request)
		}
	}))
	t.Cleanup(server.Close)

	collector := NewCollector(repo, Options{
		Owner: "home-a", HomeID: "home-a", CodexUsageURL: server.URL + "/usage", CodexResetCreditsURL: server.URL + "/resets",
		Now: clock.Now, CooldownLifter: manager,
	})
	collector.collect(context.Background())

	if duringProbe.IsZero() {
		t.Fatal("usage endpoint was not called")
	}
	item, errItem := repo.GetQuotaCredential(context.Background(), authID, clock.Now())
	if errItem != nil {
		t.Fatalf("GetQuotaCredential() error = %v", errItem)
	}
	if item.ObservedAt == nil || !item.ObservedAt.After(duringProbe) {
		t.Fatalf("snapshot observed_at = %v, want after the in-flight 429 at %s", item.ObservedAt, duringProbe)
	}

	persisted, _, errPersisted := repo.GetAuth(context.Background(), authID)
	if errPersisted != nil || persisted == nil {
		t.Fatalf("GetAuth() = %v, %v", persisted, errPersisted)
	}
	if got := persisted.ModelStates["gpt-6.1-sol"]; got == nil || got.Quota.Exceeded || got.Unavailable {
		t.Fatalf("gpt-6.1-sol persisted state = %#v, want the block from before the check lifted", got)
	}
	// Comparing against the probe end instead of its start would lift this too.
	if got := persisted.ModelStates["gpt-6.1-codex"]; got == nil || !got.Quota.Exceeded || !got.Unavailable || !got.NextRetryAfter.Equal(recoverAt) {
		t.Fatalf("gpt-6.1-codex persisted state = %#v, want the 429 recorded during the probe kept", got)
	}
	local, ok := manager.GetByID(authID)
	if !ok || local == nil {
		t.Fatal("local auth missing after lift")
	}
	if got := local.ModelStates["gpt-6.1-sol"]; got == nil || got.Quota.Exceeded || got.Unavailable {
		t.Fatalf("gpt-6.1-sol local state = %#v, want lifted", got)
	}
	if got := local.ModelStates["gpt-6.1-codex"]; got == nil || !got.Quota.Exceeded || !got.Unavailable {
		t.Fatalf("gpt-6.1-codex local state = %#v, want the 429 recorded during the probe kept", got)
	}
}
