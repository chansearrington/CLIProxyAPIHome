package cluster

import (
	"context"
	"testing"
	"time"
)

func TestUsageDedupeProcessedDuplicatesWarmStart(t *testing.T) {
	for _, backend := range []string{"sqlite", "postgres"} {
		t.Run(backend, func(t *testing.T) {
			var repo *Repository
			if backend == "postgres" {
				repo = newPostgresQuiescenceRepository(t)
			} else {
				var closeRepo func()
				repo, closeRepo = newBillingTestRepository(t, context.Background())
				defer closeRepo()
			}
			db, errDB := repo.database()
			if errDB != nil {
				t.Fatal(errDB)
			}
			payload := legacyUsagePayload("fake-pending-duplicate", time.Now().UTC())
			for i := 0; i < 3; i++ {
				insertLegacyUsageRow(t, repo, payload)
			}
			// Two copies share a batch; the third crosses its boundary.
			scanned, keyed, errBackfill := backfillUsageDedupeKeys(db, 2)
			if errBackfill != nil || scanned != 3 || keyed != 1 {
				t.Fatalf("scanned=%d keyed=%d err=%v", scanned, keyed, errBackfill)
			}
			// Processed rows must not be hashed again even if their archived payload changes.
			if errChange := db.Model(&UsageRecord{}).Where("1 = 1").Update("payload", "{}").Error; errChange != nil {
				t.Fatal(errChange)
			}
			if errMigrate := AutoMigrateContext(t.Context(), db); errMigrate != nil {
				t.Fatal(errMigrate)
			}
			scanned, keyed, errBackfill = backfillUsageDedupeKeys(db, 2)
			if errBackfill != nil || scanned != 0 || keyed != 0 {
				t.Fatalf("warm start scanned=%d keyed=%d err=%v", scanned, keyed, errBackfill)
			}
			late := insertLegacyUsageRow(t, repo, payload)
			if errMigrate := AutoMigrateContext(t.Context(), db); errMigrate != nil {
				t.Fatal(errMigrate)
			}
			var stored UsageRecord
			if errGet := db.First(&stored, late.ID).Error; errGet != nil {
				t.Fatal(errGet)
			}
			if stored.DedupeKey == "" {
				t.Fatal("late old-writer duplicate was not marked processed")
			}
		})
	}
}
