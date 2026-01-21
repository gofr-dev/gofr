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

	getLastSQLGoFrMigration = `SELECT COALESCE(MAX(version), 0) FROM gofr_migrations;`

	insertGoFrMigrationRowMySQL = `INSERT INTO gofr_migrations (version, method, start_time,duration) VALUES (?, ?, ?, ?);`

	insertGoFrMigrationRowPostgres = `INSERT INTO gofr_migrations (version, method, start_time,duration) VALUES ($1, $2, $3, $4);`

	mysql    = "mysql"
	postgres = "postgres"
	sqlite   = "sqlite"

	sqlLockTimeout = 1 // timeout in seconds for GET_LOCK
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
	lockTx *gofrSql.Tx
}

func (d *sqlMigrator) checkAndCreateMigrationTable(c *container.Container) error {
	if _, err := c.SQL.Exec(createSQLGoFrMigrationsTable); err != nil {
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

func (d *sqlMigrator) Lock(c *container.Container) error {
	// Start a transaction to get a dedicated connection from the pool
	tx, err := c.SQL.Begin()
	if err != nil {
		c.Errorf("unable to begin transaction for lock: %v", err)

		return errLockAcquisitionFailed
	}

	d.lockTx = tx

	for i := 0; i < maxRetries; i++ {
		var status int

		var err error

		switch c.SQL.Dialect() {
		case mysql:
			// GET_LOCK returns 1 if acquired, 0 if timed out, NULL on error.
			// We use a short 1s timeout in the DB and retry in Go for better control.
			err = tx.QueryRow("SELECT GET_LOCK(?, ?)", lockKey, sqlLockTimeout).Scan(&status)
		case postgres:
			// pg_try_advisory_lock returns true if acquired, false otherwise.
			var pgStatus bool

			err = tx.QueryRow("SELECT pg_try_advisory_lock(hashtext(?))", lockKey).Scan(&pgStatus)
			if pgStatus {
				status = 1
			}
		}

		if err == nil && status == 1 {
			c.Debug("SQL lock acquired successfully")

			return nil
		}

		if err != nil {
			_ = tx.Rollback()

			c.Errorf("error while acquiring SQL lock: %v", err)

			return errLockAcquisitionFailed
		}

		c.Debugf("SQL lock already held, retrying in %v... (attempt %d/%d)", retryInterval, i+1, maxRetries)
		time.Sleep(retryInterval)
	}

	_ = tx.Rollback()

	return errLockAcquisitionFailed
}

func (d *sqlMigrator) Unlock(c *container.Container) error {
	if d.lockTx == nil {
		return nil
	}

	defer func() {
		d.lockTx = nil
	}()

	switch c.SQL.Dialect() {
	case mysql:
		_, err := d.lockTx.Exec("SELECT RELEASE_LOCK(?)", lockKey)
		if err != nil {
			_ = d.lockTx.Rollback()

			c.Errorf("unable to release lock: %v", err)

			return errLockReleaseFailed
		}

		c.Debug("SQL lock released successfully")
	case postgres:
		_, err := d.lockTx.Exec("SELECT pg_advisory_unlock(hashtext(?))", lockKey)
		if err != nil {
			_ = d.lockTx.Rollback()

			c.Errorf("unable to release lock: %v", err)

			return errLockReleaseFailed
		}

		c.Debug("SQL lock released successfully")
	}

	// Rolling back or committing the transaction will release the connection back to the pool.
	return d.lockTx.Rollback()
}

func (*sqlMigrator) Name() string {
	return "SQL"
}
