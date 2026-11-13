package cluster

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestUsageObservabilityDashboardIndexesPreserveAmounts(t *testing.T) {
	amounts := []float64{0.1, 0.2, 0.3}
	var ascending, descending float64
	for index, amount := range amounts {
		ascending += amount
		descending += amounts[len(amounts)-1-index]
	}
	// This fixture guards scan directions, not every possible floating-point summation difference.
	if math.Float64bits(ascending) == math.Float64bits(descending) {
		t.Fatal("fractional fixture does not distinguish ascending and descending sums")
	}
	repo, closeRepo := newBillingTestRepository(t, t.Context())
	t.Cleanup(closeRepo)
	db, errDB := repo.database()
	if errDB != nil {
		t.Fatal(errDB)
	}
	for _, name := range []string{"idx_usage_dashboard_time_v1", "idx_usage_dashboard_provider_time_v1"} {
		if errDrop := db.Exec("DROP INDEX " + name).Error; errDrop != nil {
			t.Fatal(errDrop)
		}
	}
	from := time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	fixtures := []struct {
		provider, authIndex string
		at                  time.Time
		amount              float64
	}{
		{"codex", "dashboard-first", from, 0.1},
		{"codex", "dashboard-first", from.Add(time.Hour), 0.2},
		{"codex", "dashboard-first", from.Add(2 * time.Hour), 0.3},
		{"codex", "dashboard-second", from.Add(4 * time.Hour), 0.125},
		{"codex", "dashboard-second", from.Add(4 * time.Hour), 0.25},
		{"claude", "dashboard-other", from.Add(time.Hour), 0.5},
		{"codex", "dashboard-tied", from.Add(3 * time.Hour), 0.1},
		{"codex", "dashboard-tied", from.Add(3 * time.Hour), 0.2},
		{"codex", "dashboard-tied", from.Add(3 * time.Hour), 0.3},
	}
	for index, fixture := range fixtures {
		record := UsageRecord{Timestamp: fixture.at, Provider: fixture.provider, AuthIndex: fixture.authIndex,
			Model: "dashboard-model", LatencyMS: int64(100 - index), PayloadJSON: JSONB(`{}`)}
		if errCreate := db.Create(&record).Error; errCreate != nil {
			t.Fatal(errCreate)
		}
		charge := BillingChargeRecord{ID: fmt.Sprintf("dashboard-%d", index), UsageID: record.ID,
			PayloadHash: fmt.Sprintf("dashboard-hash-%d", index), Amount: fixture.amount, PriceSnapshot: JSONB(`{}`)}
		if errCreate := db.Create(&charge).Error; errCreate != nil {
			t.Fatal(errCreate)
		}
	}
	var queries []UsageObservabilityAggregateQuery
	for _, group := range []string{"model", "provider", "credential"} {
		for _, provider := range []string{"", " CoDeX ", "claude"} {
			queries = append(queries, UsageObservabilityAggregateQuery{
				From: &from, To: &to, GroupBy: group, Provider: provider, Metric: "total_amount", Direction: "desc",
			})
		}
	}
	before := make([]UsageObservabilityAggregateResult, len(queries))
	for index, query := range queries {
		var errList error
		before[index], errList = repo.ListUsageObservabilityAggregates(t.Context(), query)
		if errList != nil {
			t.Fatal(errList)
		}
		if before[index].Total == 0 || len(before[index].Items) == 0 {
			t.Fatal("baseline aggregate did not include the fractional charges")
		}
		if query.GroupBy == "credential" && query.Provider != "claude" {
			found := false
			for _, item := range before[index].Items {
				if item.ID == "dashboard-second" {
					found = true
					if item.RequestCount != 2 || item.TotalAmount == nil || *item.TotalAmount != 0.375 {
						t.Fatal("credential aggregate lost equal-timestamp charges")
					}
				}
			}
			if !found {
				t.Fatal("missing equal-timestamp credential group")
			}
		}
	}
	var overviewQueries []UsageObservabilityOverviewQuery
	var beforeOverviews []UsageObservabilityOverview
	for _, provider := range []string{"", " CoDeX ", "claude"} {
		query := UsageObservabilityOverviewQuery{From: &from, To: &to, Provider: provider, Timezone: "UTC", Interval: "hour"}
		overview, errOverview := repo.UsageObservabilityOverview(t.Context(), query)
		if errOverview != nil {
			t.Fatal(errOverview)
		}
		overviewQueries = append(overviewQueries, query)
		beforeOverviews = append(beforeOverviews, overview)
	}
	if errMigrate := migrateUsageObservabilityIndexes(db); errMigrate != nil {
		t.Fatal(errMigrate)
	}
	for index, query := range overviewQueries {
		after, errOverview := repo.UsageObservabilityOverview(t.Context(), query)
		if errOverview != nil {
			t.Fatal(errOverview)
		}
		if !reflect.DeepEqual(beforeOverviews[index].Totals, after.Totals) || !reflect.DeepEqual(beforeOverviews[index].Trend, after.Trend) {
			t.Fatalf("provider %q overview totals or trend changed after indexing", query.Provider)
		}
	}
	for index, query := range queries {
		after, errList := repo.ListUsageObservabilityAggregates(t.Context(), query)
		if errList != nil {
			t.Fatal(errList)
		}
		if !reflect.DeepEqual(before[index], after) {
			t.Fatalf("%s provider %q aggregate changed after indexing", query.GroupBy, query.Provider)
		}
		for itemIndex, item := range after.Items {
			if item.TotalAmount != nil && math.Float64bits(*item.TotalAmount) != math.Float64bits(*before[index].Items[itemIndex].TotalAmount) {
				t.Fatalf("%s provider %q amount bits changed after indexing", query.GroupBy, query.Provider)
			}
		}
	}
}

func TestUsageObservabilityDashboardCoveringPlans(t *testing.T) {
	repo, closeRepo := newBillingTestRepository(t, t.Context())
	t.Cleanup(closeRepo)
	db, errDB := repo.database()
	if errDB != nil {
		t.Fatal(errDB)
	}
	for _, name := range []string{"idx_usage_dashboard_time_v1", "idx_usage_dashboard_provider_time_v1"} {
		if !db.Migrator().HasIndex(&UsageRecord{}, name) {
			t.Fatalf("dashboard index %s was not migrated", name)
		}
	}
	if errMigrate := migrateUsageObservabilityIndexes(db); errMigrate != nil {
		t.Fatalf("repeat migration: %v", errMigrate)
	}
	from := time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	for _, provider := range []string{"", " CoDeX ", "claude", "missing"} {
		for _, group := range []string{"model", "provider", "credential", "user", "client_key", "endpoint", "home_ip", "executor_type", "status_code"} {
			t.Run(provider+"/"+group, func(t *testing.T) {
				query := UsageObservabilityRecordQuery{From: &from, To: &to, Provider: provider}
				wantIndex := "idx_usage_dashboard_time_v1"
				if provider != "" {
					wantIndex = "idx_usage_dashboard_provider_time_v1"
				}
				for name, scope := range map[string]*gorm.DB{
					"metrics": usageObservabilityAggregateItemsQuery(db, query, group, "total_tokens", "desc"),
					"p95":     usageObservabilityAggregateP95Query(db, query, group),
				} {
					var rows []usageObservabilityAggregateRow
					statement := scope.Session(&gorm.Session{DryRun: true}).Find(&rows).Statement
					if statement.Error != nil {
						t.Fatal(statement.Error)
					}
					var plan []struct{ Detail string }
					if errPlan := db.Raw("EXPLAIN QUERY PLAN "+statement.SQL.String(), statement.Vars...).Scan(&plan).Error; errPlan != nil {
						t.Fatal(errPlan)
					}
					found := false
					for _, row := range plan {
						if strings.Contains(row.Detail, "USING COVERING INDEX "+wantIndex) {
							found = true
						}
					}
					if !found {
						t.Fatalf("%s did not use %s: %+v", name, wantIndex, plan)
					}
				}
			})
		}
	}
}
