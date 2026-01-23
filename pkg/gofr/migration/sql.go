package migration

import (
	"context"
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

	createSQLGoFrMigrationLocksTable = `CREATE TABLE IF NOT EXISTS gofr_migration_locks (
    lock_key VARCHAR(255) PRIMARY KEY,
    owner_id VARCHAR(255) NOT NULL,
    expires_at TIMESTAMP NOT NULL
);`

	getLastSQLGoFrMigration = `SELECT COALESCE(MAX(version), 0) FROM gofr_migrations;`

	insertGoFrMigrationRowMySQL    = `INSERT INTO gofr_migrations (version, method, start_time,duration) VALUES (?, ?, ?, ?);`
	insertGoFrMigrationRowPostgres = `INSERT INTO gofr_migrations (version, method, start_time,duration) VALUES ($1, $2, $3, $4);`

	mysql    = "mysql"
	postgres = "postgres"
	sqlite   = "sqlite"

	sqlLockTTL = 10 * time.Second
)

// database/sql is the package imported so named it sqlDS.
type sqlDS struct {
	SQL
}

func (s *sqlDS) apply(m migrator) migrator {
	return &sqlMigrator{
		SQL:      s.SQL,
		migrator: m,
	}
}

type sqlMigrator struct {
	SQL

	migrator
}

func (d *sqlMigrator) checkAndCreateMigrationTable(c *container.Container) error {
	if _, err := c.SQL.Exec(createSQLGoFrMigrationsTable); err != nil {
		return err
	}

	if _, err := c.SQL.Exec(createSQLGoFrMigrationLocksTable); err != nil {
		return err
	}

	return d.migrator.checkAndCreateMigrationTable(c)
}

func (d *sqlMigrator) getLastMigration(c *container.Container) int64 {
	var lastMigration int64

	err := c.SQL.QueryRowContext(context.Background(), getLastSQLGoFrMigration).Scan(&lastMigration)
	if err != nil {
		return 0
	}

	c.Debugf("SQL last migration fetched value is: %v", lastMigration)

	lm2 := d.migrator.getLastMigration(c)

	if lm2 > lastMigration {
		return lm2
	}

	return lastMigration
}

func (d *sqlMigrator) commitMigration(c *container.Container, data transactionData) error {
	switch c.SQL.Dialect() {
	case mysql, sqlite:
		err := insertMigrationRecord(data.SQLTx, insertGoFrMigrationRowMySQL, data.MigrationNumber, data.StartTime)
		if err != nil {
			return err
		}

		c.Debugf("inserted record for migration %v in gofr_migrations table", data.MigrationNumber)

	case postgres:
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

	return d.migrator.commitMigration(c, data)
}

func insertMigrationRecord(tx *gofrSql.Tx, query string, version int64, startTime time.Time) error {
	_, err := tx.Exec(query, version, "UP", startTime, time.Since(startTime).Milliseconds())

	return err
}

func (d *sqlMigrator) beginTransaction(c *container.Container) transactionData {
	sqlTx, err := c.SQL.Begin()
	if err != nil {
		c.Errorf("unable to begin transaction: %v", err)

		return transactionData{}
	}

	cmt := d.migrator.beginTransaction(c)

	cmt.SQLTx = sqlTx

	c.Debug("SQL Transaction begin successful")

	return cmt
}

func (d *sqlMigrator) rollback(c *container.Container, data transactionData) {
	if data.SQLTx == nil {
		return
	}

	if err := data.SQLTx.Rollback(); err != nil {
		c.Error("unable to rollback transaction: %v", err)
	}

	d.migrator.rollback(c, data)

	c.Fatalf("Migration %v failed and rolled back", data.MigrationNumber)
}

func (*sqlMigrator) Lock(c *container.Container, ownerID string) error {
	for i := 0; ; i++ {
		// 1. Clean up expired locks using UTC time to avoid timezone mismatches
		_, _ = c.SQL.Exec("DELETE FROM gofr_migration_locks WHERE expires_at < ?", time.Now().UTC())

		// 2. Try to acquire lock
		expiresAt := time.Now().UTC().Add(sqlLockTTL)

		_, err := c.SQL.Exec("INSERT INTO gofr_migration_locks (lock_key, owner_id, expires_at) VALUES (?, ?, ?)",
			lockKey, ownerID, expiresAt)
		if err == nil {
			c.Debug("SQL lock acquired successfully")

			return nil
		}

		c.Debugf("SQL lock already held, retrying in %v... (attempt %d)", retryInterval, i+1)
		time.Sleep(retryInterval)
	}
}

func (*sqlMigrator) Unlock(c *container.Container, ownerID string) error {
	_, err := c.SQL.Exec("DELETE FROM gofr_migration_locks WHERE lock_key = ? AND owner_id = ?", lockKey, ownerID)
	if err != nil {
		c.Errorf("unable to release SQL lock: %v", err)

		return errLockReleaseFailed
	}

	c.Debug("SQL lock released successfully")

	return nil
}

func (*sqlMigrator) Refresh(c *container.Container, ownerID string) error {
	expiresAt := time.Now().UTC().Add(sqlLockTTL)

	_, err := c.SQL.Exec("UPDATE gofr_migration_locks SET expires_at = ? WHERE lock_key = ? AND owner_id = ?",
		expiresAt, lockKey, ownerID)
	if err != nil {
		return err
	}

	c.Debug("SQL lock refreshed successfully")

	return nil
}

func (d *sqlMigrator) Next() migrator {
	return d.migrator
}

func (*sqlMigrator) Name() string {
	return "SQL"
}
