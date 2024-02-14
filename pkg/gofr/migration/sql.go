package migration

import (
	"context"
	"database/sql"
	gofrContainer "gofr.dev/pkg/gofr/container"
	gofrSql "gofr.dev/pkg/gofr/datasource/sql"
	"time"
)

type db interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	Exec(query string, args ...interface{}) (sql.Result, error)
}

type sqlDB struct {
	container        *gofrContainer.Container
	migrationVersion int64

	db
}

func newMysql(c *gofrContainer.Container) sqlDB {
	return sqlDB{container: c}
}

func (s sqlDB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return s.container.DB.Query(query, args)
}
func (s sqlDB) QueryRow(query string, args ...interface{}) *sql.Row {
	return s.container.DB.QueryRow(query, args)
}
func (s sqlDB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return s.container.DB.QueryRowContext(ctx, query, args)
}
func (s sqlDB) Exec(query string, args ...interface{}) (sql.Result, error) {
	ensureMigrationTableExists(s.container)

	// Get last migration version
	lastMigration := getLastMigration(s.container)
	if s.migrationVersion <= lastMigration {
		// TODO exit as it has already executed
	}

	// Begin transaction
	tx, err := s.container.DB.Begin()
	if err != nil {
		s.container.Logger.Error("unable to begin transaction: %v", err)

		return nil, err
	}

	// Insert migration record
	startTime := time.Now().UTC()
	if err := insertMigrationRecord(tx, s.migrationVersion, startTime); err != nil {
		s.container.Logger.Errorf("unable to insert migration record: %v", err)
		rollbackAndLog(s.container, tx)

		return nil, err
	}

	// Run migration
	result, err := tx.Exec(query, args...)
	if err != nil {
		s.container.Logger.Errorf("unable to run migration: %v", err)
		rollbackAndLog(s.container, tx)

		return nil, err
	}

	// Update migration duration
	if err := updateMigrationDuration(tx, s.migrationVersion, startTime); err != nil {
		s.container.Logger.Errorf("unable to update migration duration: %v", err)
		rollbackAndLog(s.container, tx)

		return nil, err
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		s.container.Logger.Error("unable to commit transaction: %v", err)

		return nil, err
	}

	return result, nil
}

func ensureMigrationTableExists(c *gofrContainer.Container) {
	var exists int

	_ = c.DB.QueryRow(checkMySQLGoFrMigrationsTable).Scan(&exists)

	if exists != 1 {
		if _, err := c.DB.Exec(createMySQLGoFrMigrationsTable); err != nil {
			c.Logger.Errorf("unable to create gofr_migrations table: %v", err)
		}
	}
}

func getLastMigration(c *gofrContainer.Container) int64 {
	var lastMigration int64

	_ = c.DB.QueryRowContext(context.Background(), getLastMySQLGoFrMigration).Scan(&lastMigration)

	return lastMigration
}

func insertMigrationRecord(tx db, version int64, startTime time.Time) error {
	_, err := tx.Exec(insertGoFrMigrationRow, version, "UP", startTime)

	return err
}

func updateMigrationDuration(tx db, version int64, startTime time.Time) error {
	_, err := tx.Exec(updateDurationInMigrationRecord, time.Since(startTime).Milliseconds(), version)

	return err
}

func rollbackAndLog(c *gofrContainer.Container, tx *gofrSql.Tx) {
	if err := tx.Rollback(); err != nil {
		c.Logger.Error("unable to rollback transaction: %v", err)
	}
}

const (
	createMySQLGoFrMigrationsTable = `CREATE TABLE IF NOT EXISTS gofr_migrations (
    version BIGINT not null ,
    method VARCHAR(4) not null ,
    start_time TIMESTAMP not null ,
    duration BIGINT,
    constraint primary_key primary key (version, method)
);`

	checkMySQLGoFrMigrationsTable = `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'gofr_migrations');`

	getLastMySQLGoFrMigration = `SELECT COALESCE(MAX(version), 0) FROM gofr_migrations;`

	insertGoFrMigrationRow = `INSERT INTO gofr_migrations (version, method, start_time) VALUES (?, ?, ?);`

	updateDurationInMigrationRecord = `UPDATE gofr_migrations SET duration = ? WHERE version = ? AND method = 'UP' AND duration IS NULL;`
)
