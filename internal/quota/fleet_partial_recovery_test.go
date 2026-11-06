package quota

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	coreauth "github.com/router-for-me/CLIProxyAPIHome/internal/cliproxy/auth"
)

func TestQuotaRecoveryRequiresCompleteAmountsButNotProfileMetadata(t *testing.T) {
	repo := newCollectorTestRepository(t)
	const id = "partial-recovery-fixture"
	seedCollectorProviderAuth(t, repo, id, "xai", map[string]any{"type": "xai", "access_token": "synthetic"})
	now := time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)
	if _, _, _, errMutate := repo.MutateAuth(context.Background(), id, "fixture", func(auth *coreauth.Auth) bool {
		auth.ModelStates = map[string]*coreauth.ModelState{"model": {Quota: coreauth.QuotaState{Exceeded: true}, UpdatedAt: now.Add(-time.Hour)}}
		return true
	}); errMutate != nil {
		t.Fatal(errMutate)
	}
	var phase atomic.Int32
	server := serveXAIProbeFixture(t, func() xaiProbeFixture {
		fixture := xaiPaidFixture(now)
		fixture.monthly = `{"config":{"monthlyLimit":15000,"used":1000}}`
		used := "100"
		if phase.Load() == 2 {
			used = "10"
			fixture.profileStatus = 503
		}
		fixture.weekly = strings.Replace(fixture.weekly, `"currentPeriod":`, `"creditUsagePercent":`+used+`,"currentPeriod":`, 1)
		if phase.Load() == 1 {
			fixture.weeklyStatus = 503
		}
		return fixture
	})
	lifter := &recordingCooldownLifter{}
	collector := NewCollector(repo, Options{Owner: "partial-recovery", XAIBillingURL: server.URL + "/v1/billing", Now: func() time.Time { return now }, CooldownLifter: lifter})
	for round := int32(0); round < 3; round++ {
		phase.Store(round)
		collector.collectCredential(context.Background(), &coreauth.Auth{ID: id, Provider: "xai"}, true)
		item, errItem := repo.GetQuotaCredential(context.Background(), id, now)
		if errItem != nil {
			t.Fatal(errItem)
		}
		if round < 2 && len(lifter.recorded()) != 0 {
			t.Fatal("incomplete or exhausted amounts reopened routing")
		}
		if round == 1 && item.QuotaStatus != "exhausted" {
			t.Fatal("the missing exhausted family was not retained")
		}
		if round == 2 && (len(lifter.recorded()) != 1 || item.CollectionStatus != "partial") {
			t.Fatal("unrelated profile failure blocked complete healthy quota recovery")
		}
		now = now.Add(time.Minute)
	}
}
