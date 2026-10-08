package cluster

import (
	"context"
	"net/url"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestObservabilityReadDBDoesNotBlockLivenessOrConfigWrite(t *testing.T) {
	ctx := context.Background()
	repo, home, member := newQuiescenceMembership(t, ctx, "observability-reader")
	reader, errReader := OpenObservabilityReadDB(ctx, repo.db)
	if errReader != nil || reader == nil {
		t.Fatalf("OpenObservabilityReadDB() = %v, %v", reader, errReader)
	}
	sqlReader, errSQLReader := reader.DB()
	if errSQLReader != nil {
		t.Fatal(errSQLReader)
	}
	t.Cleanup(func() {
		if errClose := sqlReader.Close(); errClose != nil {
			t.Errorf("close observability reader: %v", errClose)
		}
	})
	repo = NewRepositoryWithObservabilityReadDB(repo.db, reader)
	if errWrite := repo.UpsertConfigValue(ctx, "debug", false); errWrite != nil {
		t.Fatal(errWrite)
	}

	// Pause the real dashboard query after it has acquired the reader connection.
	// Channels, rather than a long query or sleeps, make the contention deterministic.
	readStarted := make(chan struct{})
	releaseRead := make(chan struct{})
	var once sync.Once
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseRead) }) }
	t.Cleanup(release)
	if errCallback := reader.Callback().Row().After("gorm:row").Register("test:hold_observability_read", func(tx *gorm.DB) {
		once.Do(func() {
			close(readStarted)
			<-releaseRead
		})
	}); errCallback != nil {
		t.Fatal(errCallback)
	}
	readDone := make(chan error, 1)
	readCtx, cancelRead := context.WithCancel(ctx)
	defer cancelRead()
	go func() {
		_, errOverview := repo.UsageObservabilityOverview(readCtx, UsageObservabilityOverviewQuery{Interval: "day"})
		readDone <- errOverview
	}()
	guardCtx, cancelGuard := context.WithTimeout(ctx, 5*time.Second)
	defer cancelGuard()
	select {
	case <-readStarted:
	case errRead := <-readDone:
		t.Fatalf("dashboard exited before holding reader: %v", errRead)
	case <-guardCtx.Done():
		t.Fatal("dashboard did not acquire observability connection")
	}
	if inUse := sqlReader.Stats().InUse; inUse != 1 {
		t.Fatalf("reader connections in use = %d, want 1", inUse)
	}
	if errHeartbeat := repo.RefreshCPALiveness(guardCtx, ConnectionLifetime{
		Fingerprint: member.CertificateFingerprint,
		ConnectedAt: member.ConnectedAt,
		Home:        home,
	}); errHeartbeat != nil {
		t.Fatalf("heartbeat blocked by dashboard read: %v", errHeartbeat)
	}
	if errWrite := repo.UpsertConfigValue(guardCtx, "debug", true); errWrite != nil {
		t.Fatalf("config write blocked by dashboard read: %v", errWrite)
	}
	release()
	select {
	case errRead := <-readDone:
		if errRead != nil {
			t.Fatalf("dashboard query: %v", errRead)
		}
	case <-guardCtx.Done():
		t.Fatal("dashboard did not complete after release")
	}
	if errMutation := reader.Exec("UPDATE config SET version = version + 1").Error; errMutation == nil {
		t.Fatal("observability connection unexpectedly accepted a write")
	}
	if db, errDB := repo.database(); errDB != nil || db == reader {
		t.Fatalf("mutation database = %v, %v; must retain writer", db, errDB)
	}
	if errClose := sqlReader.Close(); errClose != nil {
		t.Fatal(errClose)
	}
	if _, errRead := repo.UsageObservabilityOverview(ctx, UsageObservabilityOverviewQuery{Interval: "day"}); errRead == nil {
		t.Fatal("closed reader unexpectedly fell back to writer")
	}
}

func TestObservabilityReadDBMemoryAndPostgresKeepExistingPool(t *testing.T) {
	for _, path := range []string{":memory:", "file::memory:?cache=shared", "file:observability-shared-memory?mode=memory&cache=shared"} {
		t.Run(path, func(t *testing.T) {
			db, errOpen := OpenSQLite(context.Background(), path)
			if errOpen != nil {
				t.Fatal(errOpen)
			}
			sqlDB, errSQLDB := db.DB()
			if errSQLDB != nil {
				t.Fatal(errSQLDB)
			}
			t.Cleanup(func() {
				if errClose := sqlDB.Close(); errClose != nil {
					t.Errorf("close memory database: %v", errClose)
				}
			})
			reader, errReader := OpenObservabilityReadDB(context.Background(), db)
			if errReader != nil || reader != nil {
				t.Fatalf("memory reader = %v, %v; want nil fallback", reader, errReader)
			}
			if selected, errSelected := NewRepositoryWithObservabilityReadDB(db, reader).observabilityDatabase(); errSelected != nil || selected != db {
				t.Fatal("in-memory database did not retain existing pool")
			}
		})
	}
	// Dialect-only fixture requires no PostgreSQL server or network connection.
	postgresDB := &gorm.DB{Config: &gorm.Config{Dialector: postgres.Open("")}}
	reader, errReader := OpenObservabilityReadDB(context.Background(), postgresDB)
	if errReader != nil || reader != nil {
		t.Fatalf("PostgreSQL reader = %v, %v; want existing pool", reader, errReader)
	}
	if selected, errSelected := NewRepositoryWithObservabilityReadDB(postgresDB, &gorm.DB{}).observabilityDatabase(); errSelected != nil || selected != postgresDB {
		t.Fatal("PostgreSQL observability did not retain existing pool")
	}
}

func TestObservabilityReadDBEscapedFilename(t *testing.T) {
	path := filepath.Join(t.TempDir(), "home #?%.db")
	uri := url.URL{Scheme: "file", Path: path, RawQuery: "cache=private"}
	db, errOpen := OpenSQLite(context.Background(), uri.String())
	if errOpen != nil {
		t.Fatal(errOpen)
	}
	sqlDB, errSQLDB := db.DB()
	if errSQLDB != nil {
		t.Fatal(errSQLDB)
	}
	defer func() {
		if errClose := sqlDB.Close(); errClose != nil {
			t.Errorf("close escaped filename writer: %v", errClose)
		}
	}()
	if errCreate := db.Exec("CREATE TABLE reader_fixture (value TEXT NOT NULL)").Error; errCreate != nil {
		t.Fatal(errCreate)
	}
	if errInsert := db.Exec("INSERT INTO reader_fixture VALUES (?)", "same database").Error; errInsert != nil {
		t.Fatal(errInsert)
	}
	reader, errReader := OpenObservabilityReadDB(context.Background(), db)
	if errReader != nil || reader == nil {
		t.Fatalf("escaped filename reader = %v, %v", reader, errReader)
	}
	sqlReader, errSQLReader := reader.DB()
	if errSQLReader != nil {
		t.Fatal(errSQLReader)
	}
	defer func() {
		if errClose := sqlReader.Close(); errClose != nil {
			t.Errorf("close escaped filename reader: %v", errClose)
		}
	}()
	var value string
	if errRead := reader.Raw("SELECT value FROM reader_fixture").Scan(&value).Error; errRead != nil {
		t.Fatal(errRead)
	}
	if value != "same database" {
		t.Fatalf("reader value = %q, want original database", value)
	}
	// Closing the idle connection forces the next operation to open a new one;
	// read-only enforcement must live in the URI rather than a one-time PRAGMA.
	sqlReader.SetMaxIdleConns(0)
	if errWrite := reader.Exec("UPDATE reader_fixture SET value = ?", "changed").Error; errWrite == nil {
		t.Fatal("replacement reader connection unexpectedly accepted a write")
	}
}

func TestObservabilityReadDBRejectsNonWALFile(t *testing.T) {
	db, errOpen := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "non-wal.db")), databaseGORMConfig(defaultDatabaseSlowQueryThreshold))
	if errOpen != nil {
		t.Fatal(errOpen)
	}
	sqlDB, errSQLDB := db.DB()
	if errSQLDB != nil {
		t.Fatal(errSQLDB)
	}
	defer func() {
		if errClose := sqlDB.Close(); errClose != nil {
			t.Errorf("close non-WAL database: %v", errClose)
		}
	}()
	reader, errReader := OpenObservabilityReadDB(context.Background(), db)
	if errReader == nil || reader != nil {
		t.Fatalf("non-WAL reader = %v, %v; want rejection", reader, errReader)
	}
}
