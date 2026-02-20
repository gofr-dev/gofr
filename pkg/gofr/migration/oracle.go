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
	checkAndCreateOracleMigrationLocksTable = `
BEGIN
	EXECUTE IMMEDIATE 'CREATE TABLE gofr_migration_locks (
		lock_key VARCHAR2(64) PRIMARY KEY,
		owner_id VARCHAR2(64) NOT NULL,
		expires_at TIMESTAMP NOT NULL
		)';
EXCEPTION
	WHEN OTHERS THEN
		IF SQLCODE != -955 THEN RAISE; END IF;
END;
`
	deleteExpiredOracleLocks = `DELETE FROM gofr_migration_locks WHERE expires_at < :1`
	insertOracleLock         = `INSERT INTO gofr_migration_locks (lock_key, owner_id, expires_at) VALUES (:1, :2, :3)`
	updateOracleLock         = `UPDATE gofr_migration_locks SET expires_at = :1 WHERE lock_key = :2 AND owner_id = :3`
	deleteOracleLock         = `DELETE FROM gofr_migration_locks WHERE lock_key = :1 AND owner_id = :2`
)

// Create migration table if it doesn't exist.
func (om oracleMigrator) checkAndCreateMigrationTable(c *container.Container) error {
	err := om.Oracle.Exec(context.Background(), checkAndCreateOracleMigrationTable)
	if err != nil {
		return err
	}

	if err := om.Oracle.Exec(context.Background(), checkAndCreateOracleMigrationLocksTable); err != nil {
		return err
	}

	return om.migrator.checkAndCreateMigrationTable(c)
}

// Get the last applied migration version.
func (om oracleMigrator) getLastMigration(c *container.Container) (int64, error) {
	var (
		results             []map[string]any
		oracleLastMigration int64
	)

	err := om.Oracle.Select(context.Background(), &results, getLastOracleGoFrMigration)
	if err != nil {
		return -1, fmt.Errorf("oracle: %w", err)
	}

	if len(results) != 0 {
		oracleLastMigration = om.extractLastMigrationFromResults(results)
	}

	c.Debugf("Oracle last migration fetched value is: %v", oracleLastMigration)

	baseLastMigration, err := om.migrator.getLastMigration(c)
	if err != nil {
		return -1, err
	}

	return max(baseLastMigration, oracleLastMigration), nil
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

func (om oracleMigrator) lock(ctx context.Context, cancel context.CancelFunc, c *container.Container, ownerID string) error {
	for i := 0; ; i++ {
		_ = om.Oracle.Exec(ctx, deleteExpiredOracleLocks, time.Now().UTC())

		expiresAt := time.Now().UTC().Add(defaultLockTTL)

		err := om.Oracle.Exec(ctx, insertOracleLock, lockKey, ownerID, expiresAt)
		if err == nil {
			c.Debug("Oracle lock acquired successfully")

			go om.startRefresh(ctx, cancel, c, ownerID)

			return om.migrator.lock(ctx, cancel, c, ownerID)
		}

		if !isDuplicateKeyError(err) {
			c.Errorf("error while acquiring Oracle lock: %v", err)

			return errLockAcquisitionFailed
		}

		c.Debugf("Oracle lock already held, retrying in %v... (attempt %d)", defaultRetry, i+1)

		select {
		case <-time.After(defaultRetry):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (oracleMigrator) startRefresh(ctx context.Context, cancel context.CancelFunc, c *container.Container, ownerID string) {
	ticker := time.NewTicker(defaultRefresh)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			expiresAt := time.Now().UTC().Add(defaultLockTTL)
			if err := c.Oracle.Exec(ctx, updateOracleLock, expiresAt, lockKey, ownerID); err != nil {
				c.Error("failed to refresh Oracle lock: %v", err)
				cancel()
				return
			}
			
			c.Debugf("Oracle lock refreshed successfully")
		case <-ctx.Done():
			return
		}
	}
}

func (om oracleMigrator) unlock(c *container.Container, ownerID string) error {
	if err := c.Oracle.Exec(context.Background(), deleteOracleLock, lockKey, ownerID); err != nil {
		c.Errorf("unable to release Oracle lock: %v", err)
		return errLockReleaseFailed
	}

	c.Debug("Oracle lock released successfully")
	return om.migrator.unlock(c, ownerID)
}

func (oracleMigrator) name() string {
	return "Oracle"
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
