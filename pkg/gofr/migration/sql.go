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
	migrationVersion int64
	used             bool

	db
}

func newMysql(version int64, d db) sqlDB {
	return sqlDB{db: d, migrationVersion: version}
}

func (s sqlDB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	s.used = true
	return s.db.Query(query, args...)
}
func (s sqlDB) QueryRow(query string, args ...interface{}) *sql.Row {
	s.used = true
	return s.db.QueryRow(query, args...)
}
func (s sqlDB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	s.used = true
	return s.db.QueryRowContext(ctx, query, args...)
}
func (s sqlDB) Exec(query string, args ...interface{}) (sql.Result, error) {
	s.used = true
	return s.db.Exec(query, args...)
}

func ensureSQLMigrationTableExists(c *gofrContainer.Container) error {
	var exists int

	err := c.DB.QueryRow(checkMySQLGoFrMigrationsTable).Scan(&exists)
	if err != nil {
		return err
	}

	if exists != 1 {
		if _, err := c.DB.Exec(createMySQLGoFrMigrationsTable); err != nil {
			return err
		}
	}

	return nil
}

func getLastMigration(c *gofrContainer.Container) int64 {
	var lastMigration int64

	err := c.DB.QueryRowContext(context.Background(), getLastMySQLGoFrMigration).Scan(&lastMigration)
	if err != nil {
		return 0
	}

	return lastMigration
}

func insertMigrationRecord(tx *gofrSql.Tx, version int64, startTime time.Time) error {
	_, err := tx.Exec(insertGoFrMigrationRow, version, "UP", startTime, time.Since(startTime).Milliseconds())

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

	insertGoFrMigrationRow = `INSERT INTO gofr_migrations (version, method, start_time,duration) VALUES (?, ?, ?, ?);`
)
