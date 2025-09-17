package migration

import (
	"context"
	"errors"
	"time"

	"gofr.dev/pkg/gofr/container"
)

var errInvalidOracleTransaction = errors.New("invalid Oracle transaction")

type oracleDS struct {
	Oracle
}

type oracleMigrator struct {
	Oracle
	migrator
}

// Provides a wrapper to apply the oracle migrator logic.
func (od oracleDS) apply(m migrator) migrator {
	return oracleMigrator{
		Oracle:   od.Oracle,
		migrator: m,
	}
}

const (
	checkAndCreateOracleMigrationTable = `
BEGIN
    EXECUTE IMMEDIATE 'CREATE TABLE gofr_migrations (
        version NUMBER NOT NULL,
        method VARCHAR2(64) NOT NULL,
        start_time TIMESTAMP NOT NULL,
        duration NUMBER NULL,
        PRIMARY KEY (version, method)
    )';
EXCEPTION
    WHEN OTHERS THEN
        IF SQLCODE != -955 THEN RAISE; END IF;
END;
`
	getLastOracleGoFrMigration = `
SELECT NVL(MAX(version), 0) AS last_migration
FROM gofr_migrations
`
	insertOracleGoFrMigrationRow = `
INSERT INTO gofr_migrations (version, method, start_time, duration)
VALUES (:1, :2, :3, :4)
`
)

// Create migration table if it doesn't exist.
func (om oracleMigrator) checkAndCreateMigrationTable(_ *container.Container) error {
	return om.Oracle.Exec(context.Background(), checkAndCreateOracleMigrationTable)
}

// Get the last applied migration version.
func (om oracleMigrator) getLastMigration(c *container.Container) int64 {
	type LastMigration struct {
		LastMigration int64 `db:"last_migration"`
	}

	var (
		lastMigrations []LastMigration
		lastMigration  int64
	)

	err := om.Oracle.Select(context.Background(), &lastMigrations, getLastOracleGoFrMigration)
	if err != nil {
		return 0
	}

	if len(lastMigrations) != 0 {
		lastMigration = lastMigrations[0].LastMigration
	}

	c.Debugf("Oracle last migration fetched value is: %v", lastMigration)

	lm2 := om.migrator.getLastMigration(c)

	if lm2 > lastMigration {
		return lm2
	}

	return lastMigration
}

// Begin a new migration transaction.
func (om oracleMigrator) beginTransaction(c *container.Container) transactionData {
	// Begin a proper transaction
	tx, err := om.Oracle.Begin()
	if err != nil {
		c.Errorf("unable to begin Oracle transaction: %v", err)

		return transactionData{}
	}

	td := om.migrator.beginTransaction(c)
	td.OracleTx = tx // Store the transaction in transactionData

	c.Debug("Oracle Transaction begin successful")

	return td
}

// Commit the migration.
func (om oracleMigrator) commitMigration(c *container.Container, data transactionData) error {
	if data.OracleTx == nil {
		c.Error("invalid Oracle transaction")

		return errInvalidOracleTransaction
	}

	// Insert migration record using the transaction
	err := data.OracleTx.ExecContext(context.Background(), insertOracleGoFrMigrationRow,
		data.MigrationNumber, "UP", data.StartTime, time.Since(data.StartTime).Milliseconds())

	if err != nil {
		c.Errorf("failed to insert migration record: %v", err)

		err := data.OracleTx.Rollback()

		if rollbackErr := data.OracleTx.Rollback(); rollbackErr != nil {
			c.Errorf("also failed to rollback: %v", rollbackErr)
		}

		return err
	}

	c.Debugf("inserted record for migration %v in Oracle gofr_migrations table", data.MigrationNumber)

	// Commit the transaction
	if err := data.OracleTx.Commit(); err != nil {
		c.Errorf("failed to commit Oracle transaction: %v", err)

		return err
	}

	return om.migrator.commitMigration(c, data)
}

// Rollback the migration.
func (om oracleMigrator) rollback(c *container.Container, data transactionData) {
	if data.OracleTx != nil {
		if err := data.OracleTx.Rollback(); err != nil {
			c.Errorf("unable to rollback Oracle transaction: %v", err)
		} else {
			c.Debug("Oracle transaction successfully rolled back")
		}
	}

	om.migrator.rollback(c, data)

	c.Fatalf("migration %v failed and rolled back", data.MigrationNumber)
}
