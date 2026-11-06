package cluster

import (
	"context"
	"strings"
	"testing"
)

func TestFleetPendingIndexRemainsUsableWithoutDuplicateBuild(t *testing.T) {
	repo, closeRepo := newBillingTestRepository(t, context.Background())
	defer closeRepo()
	db, errDB := repo.database()
	if errDB != nil {
		t.Fatal(errDB)
	}
	if errDrop := db.Exec(`DROP INDEX idx_usage_token_accounting_pending_v2`).Error; errDrop != nil {
		t.Fatal(errDrop)
	}
	if errCreate := db.Exec(`CREATE INDEX idx_usage_pending_token_accounting_v2 ON usage(id) WHERE token_accounting_version <> 2`).Error; errCreate != nil {
		t.Fatal(errCreate)
	}
	if errMigrate := migrateUsageObservabilityIndexes(db); errMigrate != nil {
		t.Fatal(errMigrate)
	}
	var duplicate int64
	if errCount := db.Raw(`SELECT count(*) FROM sqlite_master WHERE type='index' AND name='idx_usage_token_accounting_pending_v2'`).Scan(&duplicate).Error; errCount != nil {
		t.Fatal(errCount)
	}
	if duplicate != 0 {
		t.Fatal("created a redundant pending-accounting index")
	}
	var plan []struct{ Detail string }
	if errPlan := db.Raw(`EXPLAIN QUERY PLAN SELECT id FROM usage WHERE token_accounting_version <> 2 ORDER BY id LIMIT 1`).Scan(&plan).Error; errPlan != nil {
		t.Fatal(errPlan)
	}
	for _, row := range plan {
		if strings.Contains(row.Detail, "idx_usage_pending_token_accounting_v2") {
			return
		}
	}
	t.Fatal("pending-accounting query did not use the preserved fleet index")
}
