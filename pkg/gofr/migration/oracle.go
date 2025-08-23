package migration

import (
	"context"
	"gofr.dev/pkg/gofr/container"
	"time"
)

type oracleDS struct {
	Oracle
}

type oracleMigrator struct {
	Oracle
	migrator
}

// Provides a wrapper to apply the oracle migrator logic
func (od oracleDS) apply(m migrator) migrator {
	return oracleMigrator{
		Oracle:   od.Oracle,
		migrator: m,
	}
}

const (
	CheckAndCreateOracleMigrationTable = `
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

// Create migration table if it doesn't exist
func (om oracleMigrator) checkAndCreateMigrationTable(_ *container.Container) error {
	return om.Oracle.Exec(context.Background(), CheckAndCreateOracleMigrationTable)
}

// Get the last applied migration version
func (om oracleMigrator) getLastMigration(c *container.Container) int64 {
	type LastMigration struct {
		LastMigration int64 `db:"last_migration"`
	}

	var lastMigrations []LastMigration
	var lastMigration int64

	err := om.Oracle.Select(context.Background(), &lastMigrations, getLastOracleGoFrMigration)
	if err != nil {
		return 0
	}
	if len(lastMigrations) != 0 {
		lastMigration = lastMigrations[0].LastMigration
	}
	lm2 := om.migrator.getLastMigration(c)
	if lm2 > lastMigration {
		return lm2
	}
	return lastMigration
}

// Begin a new migration transaction
func (om oracleMigrator) beginTransaction(c *container.Container) transactionData {
	td := om.migrator.beginTransaction(c)
	c.Debug("OracleDB Migrator begin successfully")
	return td
}

// Commit the migration
func (om oracleMigrator) commitMigration(c *container.Container, data transactionData) error {
	err := om.Oracle.Exec(context.Background(), insertOracleGoFrMigrationRow, data.MigrationNumber,
		"UP", data.StartTime, time.Since(data.StartTime).Milliseconds())
	if err != nil {
		return err
	}
	c.Debugf("inserted record for migration %v in oracle gofr_migrations table", data.MigrationNumber)
	return om.migrator.commitMigration(c, data)
}

// Rollback the migration
func (om oracleMigrator) rollback(c *container.Container, data transactionData) {
	om.migrator.rollback(c, data)
	c.Fatalf("migration %v failed and rolled back", data.MigrationNumber)
}
