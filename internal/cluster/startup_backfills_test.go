package cluster

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestStartupBackfillsRepairLegacyAndSkipCleanRestart(t *testing.T) {
	for _, backend := range []string{"sqlite", "postgres"} {
		for _, version := range []int{17, currentDatabaseVersion} {
			t.Run(fmt.Sprintf("%s/v%d", backend, version), func(t *testing.T) {
				var db *gorm.DB
				if backend == "sqlite" {
					db = openDatabaseSnapshotSQLiteTestDB(t, filepath.Join(t.TempDir(), "startup.db"))
				} else {
					dsn := strings.TrimSpace(os.Getenv("CLIPROXY_HOME_TEST_POSTGRES_DSN"))
					if dsn == "" {
						t.Skip("CLIPROXY_HOME_TEST_POSTGRES_DSN is not set")
					}
					var errOpen error
					db, errOpen = gorm.Open(postgres.Open(dsn), databaseGORMConfig(defaultDatabaseSlowQueryThreshold))
					if errOpen != nil {
						t.Fatalf("open postgres: %v", errOpen)
					}
					sqlDB, errDB := db.DB()
					if errDB != nil {
						t.Fatal(errDB)
					}
					dropDatabaseSnapshotPostgresTables(t, db)
					t.Cleanup(func() {
						dropDatabaseSnapshotPostgresTables(t, db)
						if errClose := sqlDB.Close(); errClose != nil {
							t.Errorf("close postgres: %v", errClose)
						}
					})
				}
				if errMigrate := AutoMigrate(db); errMigrate != nil {
					t.Fatal(errMigrate)
				}
				// Existing v17 and upstream v19 databases must both acquire the fleet indexes.
				indexNames := []string{"idx_usage_service_tier_pending_v1", "idx_billing_charge_base_amount_pending_v1"}
				if backend == "sqlite" {
					indexNames = append(indexNames, "idx_usage_dashboard_time_v1", "idx_usage_dashboard_provider_time_v1")
				}
				for _, indexName := range indexNames {
					if errDrop := db.Exec("DROP INDEX IF EXISTS " + indexName).Error; errDrop != nil {
						t.Fatal(errDrop)
					}
				}
				if errVersion := db.Save(&schemaMigrationRecord{ID: schemaMigrationRecordID, Version: int64(version), UpdatedAt: time.Now().UTC()}).Error; errVersion != nil {
					t.Fatal(errVersion)
				}
				if errAlter := db.Exec(`ALTER TABLE "usage" ADD COLUMN request_service_tier TEXT`).Error; errAlter != nil {
					t.Fatal(errAlter)
				}
				// The second pass simulates an older writer after a successful migration.
				for pass := 0; pass < 2; pass++ {
					usageIDs := seedStartupBackfillRows(t, db, pass)
					if errUpdate := db.Exec(`UPDATE "usage" SET request_service_tier = 'priority' WHERE id = ?`, usageIDs[1]).Error; errUpdate != nil {
						t.Fatal(errUpdate)
					}
					if errMigrate := AutoMigrate(db); errMigrate != nil {
						t.Fatal(errMigrate)
					}
					for _, indexName := range indexNames {
						var count int64
						var errIndex error
						if backend == "sqlite" {
							errIndex = db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?", indexName).Scan(&count).Error
						} else {
							errIndex = db.Raw("SELECT COUNT(*) FROM pg_indexes WHERE schemaname = current_schema() AND indexname = ?", indexName).Scan(&count).Error
						}
						if errIndex != nil || count != 1 {
							t.Fatalf("missing index %s after v%d startup: %v", indexName, version, errIndex)
						}
					}
					assertStartupBackfillRows(t, db, pass, usageIDs, "priority")
					assertCleanStartupSkipsBackfillUpdates(t, db)
				}
				if backend == "postgres" {
					assertPostgresSchemaMigrationVersion(t, db, currentDatabaseVersion)
				}
			})
		}
	}
}

func TestStartupBackfillsRepairLegacySnapshotImport(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	source := openDatabaseSnapshotSQLiteTestDB(t, filepath.Join(t.TempDir(), "source.db"))
	usageIDs := seedStartupBackfillRows(t, source, 0)
	for _, version := range []int{17, 18, currentDatabaseVersion} {
		t.Run(fmt.Sprintf("v%d", version), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "snapshot.zip")
			if _, errExport := exportDatabaseSnapshotVersion(ctx, source, path, version); errExport != nil {
				t.Fatal(errExport)
			}
			snapshot, errOpen := OpenDatabaseSnapshot(ctx, path)
			if errOpen != nil {
				t.Fatal(errOpen)
			}
			t.Cleanup(func() {
				if errClose := snapshot.Close(); errClose != nil {
					t.Errorf("close snapshot: %v", errClose)
				}
			})
			// Target indexes have already been installed by a previous clean startup.
			target := openDatabaseSnapshotSQLiteTestDB(t, filepath.Join(t.TempDir(), "target.db"))
			if _, errImport := ImportDatabaseSnapshot(ctx, target, snapshot, nil); errImport != nil {
				t.Fatal(errImport)
			}
			assertStartupBackfillRows(t, target, 0, usageIDs, defaultUsageServiceTier)
			assertCleanStartupSkipsBackfillUpdates(t, target)
		})
	}
}

func seedStartupBackfillRows(t *testing.T, db *gorm.DB, pass int) []uint {
	t.Helper()
	now := time.Date(2026, time.November, 13, 0, 0, 0, 0, time.UTC)
	var usageIDs []uint
	for index, tier := range []any{nil, "   ", "default", "auto", ""} {
		record := UsageRecord{Timestamp: now, ServiceTier: "auto", PayloadJSON: JSONB(fmt.Sprintf(`{"execution_id":"startup-%d-%d"}`, pass, index)), CreatedAt: now}
		if errCreate := db.Create(&record).Error; errCreate != nil {
			t.Fatal(errCreate)
		}
		if errUpdate := db.Model(&UsageRecord{}).Where("id = ?", record.ID).Update("service_tier", tier).Error; errUpdate != nil {
			t.Fatal(errUpdate)
		}
		usageIDs = append(usageIDs, record.ID)
		baseAmounts := []float64{0, 3, 0, 0, 0}
		amounts := []float64{2, 3, 4, 0, -2}
		multipliers := []float64{1, 1, 0.5, 1, 1}
		charge := BillingChargeRecord{ID: fmt.Sprintf("startup-%d-%d", pass, index), UsageID: record.ID, PayloadHash: fmt.Sprintf("startup-%d-%d", pass, index), Amount: amounts[index], BaseAmount: baseAmounts[index], BillingMultiplier: multipliers[index], PriceSnapshot: JSONB(`{}`), CreatedAt: now}
		if errCreate := db.Create(&charge).Error; errCreate != nil {
			t.Fatal(errCreate)
		}
	}
	return usageIDs
}

func assertStartupBackfillRows(t *testing.T, db *gorm.DB, pass int, usageIDs []uint, blankTier string) {
	t.Helper()
	for index, wantTier := range []string{"auto", blankTier, "default", "auto", "auto"} {
		var usage UsageRecord
		if errFind := db.First(&usage, usageIDs[index]).Error; errFind != nil {
			t.Fatal(errFind)
		}
		if usage.ServiceTier != wantTier {
			t.Errorf("usage %d tier = %q, want %q", usage.ID, usage.ServiceTier, wantTier)
		}
		var charge BillingChargeRecord
		if errFind := db.First(&charge, "id = ?", fmt.Sprintf("startup-%d-%d", pass, index)).Error; errFind != nil {
			t.Fatal(errFind)
		}
		if wantBase := []float64{2, 3, 0, 0, -2}[index]; charge.BaseAmount != wantBase {
			t.Errorf("charge %s base = %v, want %v", charge.ID, charge.BaseAmount, wantBase)
		}
	}
}

func assertCleanStartupSkipsBackfillUpdates(t *testing.T, db *gorm.DB) {
	t.Helper()
	var logs bytes.Buffer
	queryDB := db.Session(&gorm.Session{Logger: logger.New(log.New(&logs, "", 0), logger.Config{LogLevel: logger.Info, ParameterizedQueries: true})})
	if errMigrate := AutoMigrate(queryDB); errMigrate != nil {
		t.Fatal(errMigrate)
	}
	if db.Dialector.Name() == "postgres" && (strings.Contains(logs.String(), "CREATE TABLE") || strings.Contains(logs.String(), "ALTER TABLE")) {
		t.Fatal("current PostgreSQL startup reran schema DDL")
	}
	for _, statement := range []string{`UPDATE "usage"`, "UPDATE `usage`", `UPDATE "billing_charge"`, "UPDATE `billing_charge`"} {
		if strings.Contains(logs.String(), statement) {
			t.Errorf("clean startup issued %s", statement)
		}
	}
}

func TestStartupBackfillProbesUseSparseIndexes(t *testing.T) {
	t.Parallel()

	db := openDatabaseSnapshotSQLiteTestDB(t, filepath.Join(t.TempDir(), "plans.db"))
	for _, query := range []struct{ sql, index string }{
		{`SELECT 1 FROM "usage" WHERE service_tier IS NULL OR TRIM(service_tier) = '' LIMIT 1`, "idx_usage_service_tier_pending_v1"},
		{`SELECT 1 FROM "billing_charge" WHERE base_amount = 0 AND amount <> 0 AND billing_multiplier = 1 LIMIT 1`, "idx_billing_charge_base_amount_pending_v1"},
	} {
		var plans []struct{ Detail string }
		if errPlan := db.Raw("EXPLAIN QUERY PLAN " + query.sql).Scan(&plans).Error; errPlan != nil {
			t.Fatal(errPlan)
		}
		if len(plans) != 1 || !strings.Contains(plans[0].Detail, query.index) {
			t.Errorf("query plan = %+v, want %s", plans, query.index)
		}
	}
}
