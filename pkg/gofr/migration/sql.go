package migration

import (
	"context"
	"database/sql"
	"time"

	"gofr.dev/pkg/gofr/container"
	gofrSql "gofr.dev/pkg/gofr/datasource/sql"
)

const (
	createSQLGoFrMigrationsTable = `CREATE TABLE IF NOT EXISTS gofr_migrations (
    version BIGINT not null ,
    method VARCHAR(4) not null ,
    start_time TIMESTAMP not null ,
    duration BIGINT,
    constraint primary_key primary key (version, method)
);`

	getLastSQLGoFrMigration = `SELECT COALESCE(MAX(version), 0) FROM gofr_migrations;`

	insertGoFrMigrationRowMySQL = `INSERT INTO gofr_migrations (version, method, start_time,duration) VALUES (?, ?, ?, ?);`

	insertGoFrMigrationRowPostgres = `INSERT INTO gofr_migrations (version, method, start_time,duration) VALUES ($1, $2, $3, $4);`
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

func insertMigrationRecord(tx *gofrSql.Tx, query string, version int64, startTime time.Time) error {
	_, err := tx.Exec(query, version, "UP", startTime, time.Since(startTime).Milliseconds())

	return err
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

func (d sqlMigrator) checkAndCreateMigrationTable(c *container.Container) error {
	if _, err := c.SQL.Exec(createSQLGoFrMigrationsTable); err != nil {
		return err
	}

	return d.Migrator.checkAndCreateMigrationTable(c)
}

func (d sqlMigrator) getLastMigration(c *container.Container) int64 {
	var lastMigration int64

	err := c.SQL.QueryRowContext(context.Background(), getLastSQLGoFrMigration).Scan(&lastMigration)
	if err != nil {
		return 0
	}

	c.Debugf("SQL last migration fetched value is: %v", lastMigration)

	lm2 := d.Migrator.getLastMigration(c)

	if lm2 > lastMigration {
		return lm2
	}

	return lastMigration
}

func (d sqlMigrator) commitMigration(c *container.Container, data migrationData) error {
	switch c.SQL.Dialect() {
	case "mysql", "sqlite":
		err := insertMigrationRecord(data.SQLTx, insertGoFrMigrationRowMySQL, data.MigrationNumber, data.StartTime)
		if err != nil {
			return err
		}

		c.Debugf("inserted record for migration %v in gofr_migrations table", data.MigrationNumber)

	case "postgres":
		err := insertMigrationRecord(data.SQLTx, insertGoFrMigrationRowPostgres, data.MigrationNumber, data.StartTime)
		if err != nil {
			return err
		}

		c.Debugf("inserted record for migration %v in gofr_migrations table", data.MigrationNumber)
	}

	// Commit transaction
	if err := data.SQLTx.Commit(); err != nil {
		return err
	}

	return d.Migrator.commitMigration(c, data)
}

func (d sqlMigrator) beginTransaction(c *container.Container) migrationData {
	sqlTx, err := c.SQL.Begin()
	if err != nil {
		c.Errorf("unable to begin transaction: %v", err)

		return migrationData{}
	}

	cmt := d.Migrator.beginTransaction(c)

	cmt.SQLTx = sqlTx

	c.Debug("SQL Transaction begin successful")

	return cmt
}

func (d sqlMigrator) rollback(c *container.Container, data migrationData) {
	if data.SQLTx == nil {
		return
	}

	if err := data.SQLTx.Rollback(); err != nil {
		c.Error("unable to rollback transaction: %v", err)
	}

	c.Errorf("Migration %v failed and rolled back", data.MigrationNumber)

	d.Migrator.rollback(c, data)
}
