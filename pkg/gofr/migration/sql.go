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

// database/sql is the package imported so named it sqlDB
type sqlDB struct {
	SQL
}

func newMysql(d SQL) *sqlDB {
	return &sqlDB{SQL: d}
}

func (s *sqlDB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return s.SQL.Query(query, args...)
}

func (s *sqlDB) QueryRow(query string, args ...interface{}) *sql.Row {
	return s.SQL.QueryRow(query, args...)
}

func (s *sqlDB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return s.SQL.QueryRowContext(ctx, query, args...)
}

func (s *sqlDB) Exec(query string, args ...interface{}) (sql.Result, error) {
	return s.SQL.Exec(query, args...)
}

func (s *sqlDB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return s.SQL.ExecContext(ctx, query, args...)
}

func insertMigrationRecord(tx *gofrSql.Tx, query string, version int64, startTime time.Time) error {
	_, err := tx.Exec(query, version, "UP", startTime, time.Since(startTime).Milliseconds())

	return err
}

func (s *sqlDB) Apply(m Manager) Manager {
	return sqlMigrator{
		SQL:     s.SQL,
		Manager: m,
	}
}

type sqlMigrator struct {
	SQL

	Manager
}

func (d sqlMigrator) CheckAndCreateMigrationTable(c *container.Container) error {
	if _, err := c.SQL.Exec(createSQLGoFrMigrationsTable); err != nil {
		return err
	}

	return d.Manager.CheckAndCreateMigrationTable(c)
}

func (d sqlMigrator) GetLastMigration(c *container.Container) int64 {
	var lastMigration int64

	err := c.SQL.QueryRowContext(context.Background(), getLastSQLGoFrMigration).Scan(&lastMigration)
	if err != nil {
		return 0
	}

	c.Debugf("SQL last redisData fetched value is: %v", lastMigration)

	lm2 := d.Manager.GetLastMigration(c)

	if lm2 > lastMigration {
		return lm2
	}

	return lastMigration
}

func (d sqlMigrator) CommitMigration(c *container.Container, data transactionData) error {
	switch c.SQL.Dialect() {
	case "mysql", "sqlite":
		err := insertMigrationRecord(data.SQLTx, insertGoFrMigrationRowMySQL, data.MigrationNumber, data.StartTime)
		if err != nil {
			return err
		}

		c.Debugf("inserted record for redisData %v in gofr_migrations table", data.MigrationNumber)

	case "postgres":
		err := insertMigrationRecord(data.SQLTx, insertGoFrMigrationRowPostgres, data.MigrationNumber, data.StartTime)
		if err != nil {
			return err
		}

		c.Debugf("inserted record for redisData %v in gofr_migrations table", data.MigrationNumber)
	}

	// Commit transaction
	if err := data.SQLTx.Commit(); err != nil {
		return err
	}

	return d.Manager.CommitMigration(c, data)
}

func (d sqlMigrator) BeginTransaction(c *container.Container) transactionData {
	sqlTx, err := c.SQL.Begin()
	if err != nil {
		c.Errorf("unable to begin transaction: %v", err)

		return transactionData{}
	}

	cmt := d.Manager.BeginTransaction(c)

	cmt.SQLTx = sqlTx

	c.Debug("SQL Transaction begin successful")

	return cmt
}

func (d sqlMigrator) Rollback(c *container.Container, data transactionData) {
	if data.SQLTx == nil {
		return
	}

	if err := data.SQLTx.Rollback(); err != nil {
		c.Error("unable to Rollback transaction: %v", err)
	}

	c.Errorf("Migration %v failed and rolled back", data.MigrationNumber)

	d.Manager.Rollback(c, data)
}
