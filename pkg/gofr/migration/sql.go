package migration

import (
	"context"
	goSql "database/sql"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/datasource/sql"
	"time"
)

type db interface {
	Query(query string, args ...interface{}) (*goSql.Rows, error)
	QueryRow(query string, args ...interface{}) *goSql.Row
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *goSql.Row
	Exec(query string, args ...interface{}) (goSql.Result, error)
}

type sqlDB struct {
	db
	usageTracker
}

func newMysql(d db, s usageTracker) sqlDB {
	return sqlDB{db: d, usageTracker: s}
}

func (s *sqlDB) Query(query string, args ...interface{}) (*goSql.Rows, error) {
	s.set()
	return s.db.Query(query, args...)
}
func (s *sqlDB) QueryRow(query string, args ...interface{}) *goSql.Row {
	s.set()
	return s.db.QueryRow(query, args...)
}
func (s *sqlDB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *goSql.Row {
	s.set()
	return s.db.QueryRowContext(ctx, query, args...)
}
func (s *sqlDB) Exec(query string, args ...interface{}) (goSql.Result, error) {
	s.set()
	return s.db.Exec(query, args...)
}

func ensureSQLMigrationTableExists(c *container.Container) error {
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

func getLastMigration(c *container.Container) int64 {
	var lastMigration int64

	err := c.DB.QueryRowContext(context.Background(), getLastMySQLGoFrMigration).Scan(&lastMigration)
	if err != nil {
		return 0
	}

	return lastMigration
}

func insertMigrationRecord(tx *sql.Tx, version int64, startTime time.Time) error {
	_, err := tx.Exec(insertGoFrMigrationRow, version, "UP", startTime, time.Since(startTime).Milliseconds())

	return err
}

func rollbackAndLog(c *container.Container, tx *sql.Tx) {
	if err := tx.Rollback(); err != nil {
		c.Logger.Error("unable to rollback transaction: %v", err)
	}
}

func sqlPostRun(c *container.Container, tx *sql.Tx, currentMigration int64, start time.Time, s usageTracker) {
	if s.get() != true {
		rollbackAndLog(c, tx)

		return
	}

	err := insertMigrationRecord(tx, currentMigration, start)
	if err != nil {
		rollbackAndLog(c, tx)

		return
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		c.Logger.Error("unable to commit transaction: %v", err)

		return
	}

	c.Logger.Infof("Migration %v ran successfully", currentMigration)
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
