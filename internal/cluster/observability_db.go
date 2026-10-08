package cluster

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// OpenObservabilityReadDB opens a separate read-only connection to a file-backed
// SQLite database. WAL readers must not occupy the scheduler's single writer
// connection while usage dashboards aggregate historical records. The caller
// owns closing the returned pool. PostgreSQL and in-memory SQLite use the
// existing pool and return nil here.
func OpenObservabilityReadDB(ctx context.Context, writer *gorm.DB, configuredSlowQueryThreshold ...time.Duration) (*gorm.DB, error) {
	if writer == nil || writer.Dialector == nil {
		return nil, fmt.Errorf("database connection is nil")
	}
	if writer.Dialector.Name() != "sqlite" {
		return nil, nil
	}
	ctx = contextOrBackground(ctx)
	var databases []struct {
		Name string
		File string
	}
	if errList := writer.WithContext(ctx).Raw("PRAGMA database_list").Scan(&databases).Error; errList != nil {
		return nil, fmt.Errorf("locate SQLite observability database: %w", errList)
	}
	path := ""
	for _, database := range databases {
		if database.Name == "main" {
			path = database.File
			break
		}
	}
	if path == "" {
		return nil, nil
	}
	var journalMode string
	if errJournal := writer.WithContext(ctx).Raw("PRAGMA journal_mode").Scan(&journalMode).Error; errJournal != nil {
		return nil, fmt.Errorf("read SQLite journal mode: %w", errJournal)
	}
	if journalMode != "wal" {
		return nil, fmt.Errorf("SQLite observability reader requires WAL journal mode")
	}
	threshold := defaultDatabaseSlowQueryThreshold
	if len(configuredSlowQueryThreshold) > 0 {
		threshold = configuredSlowQueryThreshold[0]
	}
	uriPath := filepath.ToSlash(path)
	if filepath.VolumeName(path) != "" && !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	uri := url.URL{Scheme: "file", Path: uriPath, RawQuery: "mode=ro"}
	reader, errOpen := gorm.Open(sqlite.Open(uri.String()), databaseGORMConfig(threshold))
	if errOpen != nil {
		return nil, fmt.Errorf("open SQLite observability reader: %w", errOpen)
	}
	sqlDB, errDB := reader.DB()
	if errDB != nil {
		return nil, errDB
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	if errPragma := reader.WithContext(ctx).Exec("PRAGMA busy_timeout=30000").Error; errPragma != nil {
		if errClose := sqlDB.Close(); errClose != nil {
			return nil, fmt.Errorf("configure SQLite observability reader: %w; close pool: %v", errPragma, errClose)
		}
		return nil, fmt.Errorf("configure SQLite observability reader: %w", errPragma)
	}
	if errPing := sqlDB.PingContext(ctx); errPing != nil {
		if errClose := sqlDB.Close(); errClose != nil {
			return nil, fmt.Errorf("ping SQLite observability reader: %w; close pool: %v", errPing, errClose)
		}
		return nil, errPing
	}
	return reader, nil
}

// NewRepositoryWithObservabilityReadDB reserves the optional read-only pool for
// usage observability queries. Mutation, dispatch, and liveness methods retain
// the original database connection. Configure this before serving requests.
func NewRepositoryWithObservabilityReadDB(db, observabilityDB *gorm.DB) *Repository {
	repo := NewRepository(db)
	if db != nil && db.Dialector != nil && db.Dialector.Name() == "sqlite" {
		repo.observabilityDB = observabilityDB
	}
	return repo
}

func (r *Repository) observabilityDatabase() (*gorm.DB, error) {
	if r != nil && r.observabilityDB != nil {
		return r.observabilityDB, nil
	}
	return r.database()
}
