package cluster

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestStartupBackfillsRepairLegacyAndSkipCleanRestart(t *testing.T) {
	for _, backend := range []string{"sqlite", "postgres"} {
		t.Run(backend, func(t *testing.T) {
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
			// Simulate an existing v17 database without the unversioned fleet indexes.
			indexNames := []string{"idx_usage_service_tier_pending_v1", "idx_billing_charge_base_amount_pending_v1"}
			if backend == "sqlite" {
				indexNames = append(indexNames, "idx_usage_dashboard_time_v1", "idx_usage_dashboard_provider_time_v1")
			}
			for _, indexName := range indexNames {
				if errDrop := db.Exec("DROP INDEX IF EXISTS " + indexName).Error; errDrop != nil {
					t.Fatal(errDrop)
				}
			}
			if backend == "postgres" {
				if errVersion := db.Model(&schemaMigrationRecord{}).Where("id = ?", schemaMigrationRecordID).Update("version", 17).Error; errVersion != nil {
					t.Fatal(errVersion)
				}
			}
			if errAlter := db.Exec(`ALTER TABLE "usage" ADD COLUMN request_service_tier TEXT`).Error; errAlter != nil {
				t.Fatal(errAlter)
			}
			// The second pass simulates older inserts and updates after a successful migration.
			var firstUsageIDs []uint
			for pass := 0; pass < 2; pass++ {
				usageIDs := seedStartupBackfillRows(t, db, pass)
				if errUpdate := db.Exec(`UPDATE "usage" SET request_service_tier = 'priority' WHERE id = ?`, usageIDs[1]).Error; errUpdate != nil {
					t.Fatal(errUpdate)
				}
				if pass == 0 {
					firstUsageIDs = usageIDs
				} else {
					if errUpdate := db.Exec(`UPDATE "usage" SET service_tier = '' WHERE id = ?`, firstUsageIDs[0]).Error; errUpdate != nil {
						t.Fatal(errUpdate)
					}
					if errUpdate := db.Exec(`UPDATE "billing_charge" SET base_amount = 0 WHERE id = ?`, "startup-0-0").Error; errUpdate != nil {
						t.Fatal(errUpdate)
					}
				}
				var migrationLogs bytes.Buffer
				queryDB := db.Session(&gorm.Session{Logger: logger.New(log.New(&migrationLogs, "", 0), logger.Config{LogLevel: logger.Info, ParameterizedQueries: true})})
				if errMigrate := AutoMigrate(queryDB); errMigrate != nil {
					t.Fatal(errMigrate)
				}
				if backend == "postgres" {
					for _, statement := range []string{"CREATE TABLE", "ALTER TABLE", "DROP "} {
						if strings.Contains(migrationLogs.String(), statement) {
							t.Errorf("current PostgreSQL v17 startup issued full schema DDL: %s", statement)
						}
					}
				}
				for _, indexName := range indexNames {
					var count int64
					if backend == "sqlite" {
						if errCount := db.Raw(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?`, indexName).Scan(&count).Error; errCount != nil {
							t.Fatal(errCount)
						}
					} else if errCount := db.Raw(`SELECT COUNT(*) FROM pg_indexes WHERE schemaname = current_schema() AND indexname = ?`, indexName).Scan(&count).Error; errCount != nil {
						t.Fatal(errCount)
					}
					if count != 1 {
						t.Errorf("v17 startup did not install index %s", indexName)
					}
				}
				assertStartupBackfillRows(t, db, pass, usageIDs, "priority")
				assertStartupBackfillRows(t, db, 0, firstUsageIDs, "priority")
				assertCleanStartupSkipsBackfillUpdates(t, db)
			}
			if backend == "postgres" {
				assertPostgresSchemaMigrationVersion(t, db, currentDatabaseVersion)
			}
		})
	}
}

func TestStartupBackfillsRepairLegacySnapshotImport(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	source := openDatabaseSnapshotSQLiteTestDB(t, filepath.Join(t.TempDir(), "source.db"))
	usageIDs := seedStartupBackfillRows(t, source, 0)
	for _, version := range []int{16, 17} {
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
		record := UsageRecord{Timestamp: now, ServiceTier: "auto", PayloadJSON: JSONB(`{}`), CreatedAt: now}
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
	indexNames := []string{"idx_usage_service_tier_pending_v1", "idx_billing_charge_base_amount_pending_v1", "idx_usage_dashboard_time_v1", "idx_usage_dashboard_provider_time_v1"}
	type indexColumn struct {
		Seqno int
		Cid   int
		Name  *string
		Desc  int
		Coll  *string
		Key   int
	}
	type indexState struct {
		SchemaRowID int64
		Type        string
		Name        string
		TblName     string
		Rootpage    int64
		SQL         string
		Columns     []indexColumn `gorm:"-"`
	}
	readIndexes := func() []indexState {
		t.Helper()
		var indexes []indexState
		if errSchema := db.Raw(`SELECT rowid AS schema_row_id, type, name, tbl_name, rootpage, sql FROM sqlite_master WHERE type = 'index' AND name IN ? ORDER BY name`, indexNames).Scan(&indexes).Error; errSchema != nil {
			t.Fatal(errSchema)
		}
		if len(indexes) != len(indexNames) {
			t.Fatalf("startup indexes = %d, want %d", len(indexes), len(indexNames))
		}
		for index := range indexes {
			state := &indexes[index]
			if errColumns := db.Raw("PRAGMA index_xinfo('" + state.Name + "')").Scan(&state.Columns).Error; errColumns != nil {
				t.Fatal(errColumns)
			}
			if state.Rootpage <= 0 || state.SQL == "" || len(state.Columns) == 0 {
				t.Fatalf("incomplete startup index metadata: %+v", state)
			}
		}
		return indexes
	}
	var indexesBefore []indexState
	if db.Dialector.Name() == "sqlite" {
		indexesBefore = readIndexes()
	}
	var logs bytes.Buffer
	queryDB := db.Session(&gorm.Session{Logger: logger.New(log.New(&logs, "", 0), logger.Config{LogLevel: logger.Info, ParameterizedQueries: true})})
	if errMigrate := AutoMigrate(queryDB); errMigrate != nil {
		t.Fatal(errMigrate)
	}
	if db.Dialector.Name() == "sqlite" {
		// The global DDL counter also changes for the unchanged billing-model-price index.
		if indexesAfter := readIndexes(); !reflect.DeepEqual(indexesAfter, indexesBefore) {
			t.Errorf("clean startup changed fleet index identity/definitions: before=%+v after=%+v", indexesBefore, indexesAfter)
		}
		// Rootpages can be reused after a rebuild, so also reject destructive recorded SQL.
		sql := strings.NewReplacer(`"`, "", "`", "").Replace(logs.String())
		rebuild := regexp.MustCompile(`(?i)\bDROP\s+INDEX\s+(IF\s+EXISTS\s+)?(main\.)?(` + strings.Join(indexNames, "|") + `)\b|\bREINDEX\b|\b(DROP\s+TABLE\s+(IF\s+EXISTS\s+)?|ALTER\s+TABLE\s+)(main\.)?(usage|billing_charge)\b`)
		if statement := rebuild.FindString(sql); statement != "" {
			t.Errorf("clean startup rebuilt fleet indexes: %s", statement)
		}
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
