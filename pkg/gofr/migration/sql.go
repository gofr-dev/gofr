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

func newMysql(d db) *sqlDB {
	return &sqlDB{db: d}
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

func ensureSQLMigrationTableExists(c container.Interface) error {
	// this can be replaced with having switch case only in the exists variable - but we have chosen to differentiate based
	// on driver because if new dialect comes will follow the same, also this complete has to be refactored as mentioned in RUN.
	switch c.GetDB().Driver().(type) {
	case *mysql.MySQLDriver:
		var exists int

		err := c.GetDB().QueryRow(checkSQLGoFrMigrationsTable).Scan(&exists)
		if err != nil {
			return err
		}

		if exists != 1 {
			if _, err := c.GetDB().Exec(createSQLGoFrMigrationsTable); err != nil {
				return err
			}
		}
	case *pq.Driver:
		var exists bool

		err := c.GetDB().QueryRow(checkSQLGoFrMigrationsTable).Scan(&exists)
		if err != nil {
			return err
		}

		if !exists {
			if _, err := c.GetDB().Exec(createSQLGoFrMigrationsTable); err != nil {
				return err
			}
		}
	}

	return nil
}

func getSQLLastMigration(c container.Interface) int64 {
	var lastMigration int64

	err := c.GetDB().QueryRowContext(context.Background(), getLastSQLGoFrMigration).Scan(&lastMigration)
	if err != nil {
		return 0
	}

	return lastMigration
}

func insertMigrationRecord(tx *gofrSql.Tx, query string, version int64, startTime time.Time) error {
	_, err := tx.Exec(query, version, "UP", startTime, time.Since(startTime).Milliseconds())

	return err
}

func rollbackAndLog(c container.Interface, version int64, tx *gofrSql.Tx, err error) {
	c.Error(err)

	if tx == nil {
		return
	}

	if err := tx.Rollback(); err != nil {
		c.Error("unable to rollback transaction: %v", err)
	}

	c.Errorf("Migration %v rolled back", version)
}

func sqlPostRun(c container.Interface, tx *gofrSql.Tx, currentMigration int64, start time.Time) {
	switch c.GetDB().Driver().(type) {
	case *mysql.MySQLDriver:
		err := insertMigrationRecord(tx, insertGoFrMigrationRowMySQL, currentMigration, start)
		if err != nil {
			rollbackAndLog(c, currentMigration, tx, err)

			return
		}
	case *pq.Driver:
		err := insertMigrationRecord(tx, insertGoFrMigrationRowPostgres, currentMigration, start)
		if err != nil {
			rollbackAndLog(c, currentMigration, tx, err)

			return
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		c.Error("unable to commit transaction: %v", err)

		return
	}

	c.Infof("Migration %v ran successfully", currentMigration)
}

type sqlMigratorObject struct {
	db
}

type sqlMigrator struct {
	db

	Migrator
}

func (s sqlMigratorObject) apply(m Migrator) Migrator {
	return sqlMigrator{
		db:       s.db,
		Migrator: m,
	}
}

func (d sqlMigrator) CheckAndCreateMigrationTable(c container.Interface) error {
	// this can be replaced with having switch case only in the exists variable - but we have chosen to differentiate based
	// on driver because if new dialect comes will follow the same, also this complete has to be refactored as mentioned in RUN.
	switch c.GetDB().Driver().(type) {
	case *mysql.MySQLDriver:
		var exists int

		err := c.GetDB().QueryRow(checkSQLGoFrMigrationsTable).Scan(&exists)
		if err != nil {
			return err
		}

		if exists != 1 {
			if _, err := c.GetDB().Exec(createSQLGoFrMigrationsTable); err != nil {
				return err
			}
		}
	case *pq.Driver:
		var exists bool

		err := c.GetDB().QueryRow(checkSQLGoFrMigrationsTable).Scan(&exists)
		if err != nil {
			return err
		}

		if !exists {
			if _, err := c.GetDB().Exec(createSQLGoFrMigrationsTable); err != nil {
				return err
			}
		}
	}

	return d.Migrator.CheckAndCreateMigrationTable(c)
}

func (d sqlMigrator) GetLastMigration(c container.Interface) int64 {
	var lastMigration int64

	err := c.GetDB().QueryRowContext(context.Background(), getLastSQLGoFrMigration).Scan(&lastMigration)
	if err != nil {
		return 0
	}

	lm2 := d.Migrator.GetLastMigration(c)

	if lm2 > lastMigration {
		return lm2
	}

	return lastMigration
}

func (d sqlMigrator) CommitMigration(c container.Interface, data commit) error {
	switch c.GetDB().Driver().(type) {
	case *mysql.MySQLDriver:
		err := insertMigrationRecord(data.SQLTx, insertGoFrMigrationRowMySQL, data.MigrationNumber, data.StartTime)
		if err != nil {
			return err
		}

	case *pq.Driver:
		err := insertMigrationRecord(data.SQLTx, insertGoFrMigrationRowPostgres, data.MigrationNumber, data.StartTime)
		if err != nil {
			return err
		}
	}

	// Commit transaction
	if err := data.SQLTx.Commit(); err != nil {
		c.Error("unable to commit transaction: %v", err)

		return err
	}

	c.Infof("Migration %v ran successfully", data.MigrationNumber)

	return nil
}

func (d sqlMigrator) Rollback(c container.Interface, data commit) error {
	if data.SQLTx == nil {
		return nil
	}

	if err := data.SQLTx.Rollback(); err != nil {
		c.Error("unable to rollback transaction: %v", err)
	}

	c.Errorf("Migration %v rolled back", data.MigrationNumber)

	return nil
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
