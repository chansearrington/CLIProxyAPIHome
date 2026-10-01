package quota

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPIHome/internal/cluster"
)

const copilotQuotaTestBody = `{
	"copilot_plan": "enterprise",
	"quota_reset_date": "2026-11-01",
	"quota_reset_date_utc": "2026-11-01T00:00:00.000Z",
	"quota_snapshots": {
		"chat": {"entitlement": 0, "remaining": 0, "percent_remaining": 100.0, "unlimited": true, "quota_id": "chat"},
		"completions": {"entitlement": 0, "remaining": 0, "percent_remaining": 100.0, "unlimited": true, "quota_id": "completions"},
		"premium_interactions": {"entitlement": 300, "remaining": 120, "percent_remaining": 40.0, "unlimited": false, "overage_permitted": true, "quota_id": "premium_interactions"}
	}
}`

// Copilot premium requests must keep GitHub's real allowance and remaining
// count, and unlimited buckets must stay unlimited instead of gaining a limit.
func TestParseCopilotQuotaWindows(t *testing.T) {
	now := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	windows, plan, errParse := parseCopilotQuotaWindows([]byte(copilotQuotaTestBody), now)
	if errParse != nil {
		t.Fatalf("parseCopilotQuotaWindows() error = %v", errParse)
	}
	if len(windows) != 3 {
		t.Fatalf("windows = %d, want 3", len(windows))
	}
	premium := windows[0]
	if premium.ID != "copilot-premium-interactions" || premium.Label == nil || *premium.Label != "Premium Requests" || premium.Unit != "requests" {
		t.Fatalf("unexpected premium window identity: %+v", premium)
	}
	if premium.Limit == nil || *premium.Limit != 300 || premium.Remaining == nil || *premium.Remaining != 120 || premium.Used == nil || *premium.Used != 180 {
		t.Fatalf("unexpected premium quantities: %+v", premium)
	}
	if premium.RemainingRatio == nil || *premium.RemainingRatio != 0.4 || premium.Status != "healthy" || premium.IsUnlimited {
		t.Fatalf("unexpected premium ratio/status: %+v", premium)
	}
	wantReset := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	if premium.ResetAt == nil || !premium.ResetAt.Equal(wantReset) || premium.PeriodUnit != "month" || premium.PeriodValue == nil || *premium.PeriodValue != 1 {
		t.Fatalf("unexpected premium period: %+v", premium)
	}
	for _, window := range windows[1:] {
		if !window.IsUnlimited || window.Status != "healthy" || window.Limit != nil || window.UsedRatio != nil {
			t.Fatalf("unlimited bucket gained a limit: %+v", window)
		}
	}
	if windows[1].ID != "copilot-chat" || windows[2].ID != "copilot-completions" {
		t.Fatalf("unexpected window order: %s, %s", windows[1].ID, windows[2].ID)
	}
	if plan == nil || plan.Name != "Enterprise" || plan.Premium {
		t.Fatalf("unexpected plan: %+v", plan)
	}
}

func TestParseCopilotQuotaWindowsFallbacks(t *testing.T) {
	now := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	body := []byte(`{
		"copilot_plan": "individual_pro",
		"quota_reset_date": "2026-11-01",
		"quota_snapshots": {
			"chat": {"entitlement": 0, "remaining": 0, "unlimited": false},
			"premium_interactions": {"entitlement": "1500", "percent_remaining": 0, "unlimited": false},
			"something_new": {"entitlement": 10, "remaining": 5, "unlimited": false}
		}
	}`)
	windows, plan, errParse := parseCopilotQuotaWindows(body, now)
	if errParse != nil {
		t.Fatalf("parseCopilotQuotaWindows() error = %v", errParse)
	}
	if len(windows) != 1 {
		t.Fatalf("windows = %+v, want only premium (zero-allowance and unknown buckets skipped)", windows)
	}
	premium := windows[0]
	if premium.Limit == nil || *premium.Limit != 1500 || premium.Remaining == nil || *premium.Remaining != 0 || premium.Status != "exhausted" {
		t.Fatalf("percent_remaining fallback not applied: %+v", premium)
	}
	if premium.ResetAt == nil || !premium.ResetAt.Equal(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("date-only reset fallback not applied: %+v", premium.ResetAt)
	}
	if plan == nil || plan.Name != "Pro+" || !plan.Premium {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	if empty, _, _ := parseCopilotQuotaWindows([]byte(`{"quota_snapshots":{}}`), now); len(empty) != 0 {
		t.Fatalf("empty snapshots produced windows: %+v", empty)
	}
}

func TestCollectorPersistsCopilotQuotaWithGitHubToken(t *testing.T) {
	repo := newCollectorTestRepository(t)
	now := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	seedCollectorProviderAuth(t, repo, "copilot-probe", "copilot", map[string]any{"type": "copilot", "github_access_token": "gho_fake_test_token", "github_login": "octo"})
	requestHeaders := make(chan http.Header, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requestHeaders <- request.Header.Clone()
		_, _ = w.Write([]byte(copilotQuotaTestBody))
	}))
	defer server.Close()

	collector := NewCollector(repo, Options{Owner: "home-a", CopilotUserURL: server.URL, Now: func() time.Time { return now }})
	collector.collect(context.Background())

	select {
	case headers := <-requestHeaders:
		if headers.Get("Authorization") != "Bearer gho_fake_test_token" || headers.Get("Accept") != "application/json" {
			t.Fatalf("unexpected Copilot request headers: %v", headers)
		}
	case <-time.After(time.Second):
		t.Fatal("Copilot probe request was not observed")
	}

	result, errList := repo.ListQuotaCredentials(context.Background(), cluster.QuotaListQuery{
		Limit: 50, IDs: map[string]struct{}{"copilot-probe": {}}, Sort: "risk_desc", Now: now.Add(time.Minute),
	})
	if errList != nil {
		t.Fatalf("ListQuotaCredentials() error = %v", errList)
	}
	if len(result.Items) != 1 {
		t.Fatalf("unexpected Copilot list items: %+v", result.Items)
	}
	item := result.Items[0]
	if item.Provider != "copilot" || item.QuotaStatus != "healthy" || item.CollectionStatus != "success" || item.Freshness != "fresh" {
		t.Fatalf("unexpected Copilot quota status: %+v", item)
	}
	if item.Plan == nil || item.Plan.Name != "Enterprise" {
		t.Fatalf("unexpected Copilot plan: %+v", item.Plan)
	}
	if item.WindowCount != 3 || len(item.PrimaryWindows) != 2 || item.PrimaryWindows[0].ID != "copilot-premium-interactions" {
		t.Fatalf("unexpected Copilot windows: count=%d primary=%+v", item.WindowCount, item.PrimaryWindows)
	}
}
