package migration

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"gofr.dev/pkg/gofr/container"
)

var (
	errInvalidOracleTransaction      = errors.New("invalid Oracle transaction")
	errNestedTransactionNotSupported = errors.New("nested transactions not supported")
)

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
func (om oracleMigrator) checkAndCreateMigrationTable(c *container.Container) error {
	err := om.Oracle.Exec(context.Background(), checkAndCreateOracleMigrationTable)
	if err != nil {
		c.Errorf("Failed to create Oracle migration table: %v", err)
	} else {
		c.Infof("Oracle migration table checked/created successfully")
	}

	return err
}

// Get the last applied migration version.
func (om oracleMigrator) getLastMigration(c *container.Container) int64 {
	var results []map[string]any

	err := om.Oracle.Select(context.Background(), &results, getLastOracleGoFrMigration)
	if err != nil {
		c.Errorf("Failed to fetch last migration: %v", err)
		return 0
	}

	oracleLastMigration := om.extractLastMigrationFromResults(results)
	c.Debugf("Oracle last migration fetched value is: %v", oracleLastMigration)

	baseLastMigration := om.migrator.getLastMigration(c)

	if baseLastMigration > oracleLastMigration {
		return baseLastMigration
	}

	return oracleLastMigration
}

// extractLastMigrationFromResults handles Oracle number type conversion.
func (om oracleMigrator) extractLastMigrationFromResults(results []map[string]any) int64 {
	if len(results) == 0 {
		return 0
	}

	lastMigVal, exists := results[0]["LAST_MIGRATION"]
	if !exists {
		return 0
	}

	return om.convertToInt64(lastMigVal)
}

// convertToInt64 converts various Oracle number types to int64.
func (om oracleMigrator) convertToInt64(value any) int64 {
	switch v := value.(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	default:
		return om.parseStringValue(v)
	}
}

// parseStringValue handles godror.Number type by converting to string then parsing.
func (oracleMigrator) parseStringValue(value any) int64 {
	str := fmt.Sprintf("%v", value)
	if str == "" || str == "<nil>" {
		return 0
	}

	parsed, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return 0
	}

	return parsed
}

// Commit the migration and insert a record into the migration table.
func (om oracleMigrator) commitMigration(c *container.Container, data transactionData) error {
	if data.OracleTx == nil {
		c.Error("invalid Oracle transaction")
		return errInvalidOracleTransaction
	}

	// Insert migration record using the transaction.
	err := data.OracleTx.ExecContext(context.Background(), insertOracleGoFrMigrationRow,
		data.MigrationNumber, "UP", data.StartTime, time.Since(data.StartTime).Milliseconds())
	if err != nil {
		c.Errorf("failed to insert migration record: %v", err)

		return err
	}

	c.Debugf("inserted record for migration %v in Oracle gofr_migrations table", data.MigrationNumber)

	// Commit the transaction.
	if err := data.OracleTx.Commit(); err != nil {
		c.Errorf("failed to commit Oracle transaction: %v", err)
		return err
	}

	return om.migrator.commitMigration(c, data)
}

// Rollback the migration transaction.
func (om oracleMigrator) rollback(c *container.Container, data transactionData) {
	if data.OracleTx != nil {
		if err := data.OracleTx.Rollback(); err != nil {
			c.Fatalf("unable to rollback Oracle transaction: %v", err)
		} else {
			c.Fatalf("Oracle migration failed, transaction rolled back - exiting application")
		}
	}

	// Call the base migrator's rollback.
	om.migrator.rollback(c, data)
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

type oracleTransactionWrapper struct {
	tx container.OracleTx
}

func (otw *oracleTransactionWrapper) Exec(ctx context.Context, query string, args ...any) error {
	return otw.tx.ExecContext(ctx, query, args...)
}

func (otw *oracleTransactionWrapper) Select(ctx context.Context, dest any, query string, args ...any) error {
	return otw.tx.SelectContext(ctx, dest, query, args...)
}

func (*oracleTransactionWrapper) Begin() (container.OracleTx, error) {
	return nil, errNestedTransactionNotSupported
}
