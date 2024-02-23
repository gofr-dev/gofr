package migration

import (
	"context"
	"database/sql"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/lib/pq"

	"gofr.dev/pkg/gofr/container"
	gofrSql "gofr.dev/pkg/gofr/datasource/sql"
)

type db interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	Exec(query string, args ...interface{}) (sql.Result, error)
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

type sqlDB struct {
	db
}

func newMysql(d db) sqlDB {
	return sqlDB{db: d}
}

func (s *sqlDB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return s.db.Query(query, args...)
}
func (s *sqlDB) QueryRow(query string, args ...interface{}) *sql.Row {
	return s.db.QueryRow(query, args...)
}
func (s *sqlDB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return s.db.QueryRowContext(ctx, query, args...)
}
func (s *sqlDB) Exec(query string, args ...interface{}) (sql.Result, error) {
	return s.db.Exec(query, args...)
}

func (s *sqlDB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return s.db.ExecContext(ctx, query, args...)
}

func ensureSQLMigrationTableExists(c *container.Container) error {
	switch c.DB.Driver().(type) {
	case *mysql.MySQLDriver:
		var exists int

		err := c.DB.QueryRow(checkSQLGoFrMigrationsTable).Scan(&exists)
		if err != nil {
			return err
		}

		if exists != 1 {
			if _, err := c.DB.Exec(createSQLGoFrMigrationsTable); err != nil {
				return err
			}
		}
	case *pq.Driver:
		var exists bool

		err := c.DB.QueryRow(checkSQLGoFrMigrationsTable).Scan(&exists)
		if err != nil {
			return err
		}

		if !exists {
			if _, err := c.DB.Exec(createSQLGoFrMigrationsTable); err != nil {
				return err
			}
		}
	}

	return nil
}

func getSQLLastMigration(c *container.Container) int64 {
	var lastMigration int64

	err := c.DB.QueryRowContext(context.Background(), getLastSQLGoFrMigration).Scan(&lastMigration)
	if err != nil {
		return 0
	}

	return lastMigration
}

func insertMigrationRecord(tx *gofrSql.Tx, query string, version int64, startTime time.Time) error {
	_, err := tx.Exec(query, version, "UP", startTime, time.Since(startTime).Milliseconds())

	return err
}

func rollbackAndLog(c *container.Container, tx *gofrSql.Tx) {
	if err := tx.Rollback(); err != nil {
		c.Logger.Error("unable to rollback transaction: %v", err)
	}
}

func sqlPostRun(c *container.Container, tx *gofrSql.Tx, currentMigration int64, start time.Time) {
	switch c.DB.Driver().(type) {
	case *mysql.MySQLDriver:
		err := insertMigrationRecord(tx, insertGoFrMigrationRowMySQL, currentMigration, start)
		if err != nil {
			rollbackAndLog(c, tx)

			return
		}
	case *pq.Driver:
		err := insertMigrationRecord(tx, insertGoFrMigrationRowPostgres, currentMigration, start)
		if err != nil {
			rollbackAndLog(c, tx)

			return
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		c.Logger.Error("unable to commit transaction: %v", err)

		return
	}

	c.Logger.Infof("Migration %v ran successfully", currentMigration)
}

const (
	createSQLGoFrMigrationsTable = `CREATE TABLE IF NOT EXISTS gofr_migrations (
    version BIGINT not null ,
    method VARCHAR(4) not null ,
    start_time TIMESTAMP not null ,
    duration BIGINT,
    constraint primary_key primary key (version, method)
);`

	checkSQLGoFrMigrationsTable = `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'gofr_migrations');`

	getLastSQLGoFrMigration = `SELECT COALESCE(MAX(version), 0) FROM gofr_migrations;`

	insertGoFrMigrationRowMySQL = `INSERT INTO gofr_migrations (version, method, start_time,duration) VALUES (?, ?, ?, ?);`

	insertGoFrMigrationRowPostgres = `INSERT INTO gofr_migrations (version, method, start_time,duration) VALUES ($1, $2, $3, $4);`
)
