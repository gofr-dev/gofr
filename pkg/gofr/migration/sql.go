package migration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gofr.dev/pkg/gofr/container"
	gofrSql "gofr.dev/pkg/gofr/datasource/sql"
)

var errSQLLockRefreshFailed = errors.New("failed to refresh SQL lock: lock lost or stolen")

const (
	createSQLGoFrMigrationsTable = `CREATE TABLE IF NOT EXISTS gofr_migrations (
    version BIGINT not null ,
    method VARCHAR(4) not null ,
    start_time TIMESTAMP not null ,
    duration BIGINT,
    constraint primary_key primary key (version, method)
);`

	createSQLGoFrMigrationLocksTable = `CREATE TABLE IF NOT EXISTS gofr_migration_locks (
    lock_key VARCHAR(64) PRIMARY KEY,
    owner_id VARCHAR(64) NOT NULL,
    expires_at TIMESTAMP NOT NULL
);`

	getLastSQLGoFrMigration = `SELECT COALESCE(MAX(version), 0) FROM gofr_migrations;`

	insertGoFrMigrationRowMySQL    = `INSERT INTO gofr_migrations (version, method, start_time,duration) VALUES (?, ?, ?, ?);`
	insertGoFrMigrationRowPostgres = `INSERT INTO gofr_migrations (version, method, start_time,duration) VALUES ($1, $2, $3, $4);`

	deleteExpiredLocksMySQL    = "DELETE FROM gofr_migration_locks WHERE expires_at < ?"
	deleteExpiredLocksPostgres = "DELETE FROM gofr_migration_locks WHERE expires_at < $1"

	insertLockMySQL    = "INSERT INTO gofr_migration_locks (lock_key, owner_id, expires_at) VALUES (?, ?, ?)"
	insertLockPostgres = "INSERT INTO gofr_migration_locks (lock_key, owner_id, expires_at) VALUES ($1, $2, $3)"

	updateLockMySQL    = "UPDATE gofr_migration_locks SET expires_at = ? WHERE lock_key = ? AND owner_id = ?"
	updateLockPostgres = "UPDATE gofr_migration_locks SET expires_at = $1 WHERE lock_key = $2 AND owner_id = $3"

	deleteLockMySQL    = "DELETE FROM gofr_migration_locks WHERE lock_key = ? AND owner_id = ?"
	deleteLockPostgres = "DELETE FROM gofr_migration_locks WHERE lock_key = $1 AND owner_id = $2"

	mysql    = "mysql"
	postgres = "postgres"
	sqlite   = "sqlite"
)

// database/sql is the package imported so named it sqlDS.
type sqlDS struct {
	SQL
}

func (s *sqlDS) apply(m migrator) migrator {
	return sqlMigrator{
		SQL:      s.SQL,
		migrator: m,
	}
}

type sqlMigrator struct {
	SQL

	migrator
}

func (d sqlMigrator) checkAndCreateMigrationTable(c *container.Container) error {
	if _, err := c.SQL.Exec(createSQLGoFrMigrationsTable); err != nil {
		return err
	}

	if _, err := c.SQL.Exec(createSQLGoFrMigrationLocksTable); err != nil {
		return err
	}

	return d.migrator.checkAndCreateMigrationTable(c)
}

func (d sqlMigrator) getLastMigration(c *container.Container) (int64, error) {
	var lastMigration int64

	err := c.SQL.QueryRowContext(context.Background(), getLastSQLGoFrMigration).Scan(&lastMigration)
	if err != nil {
		return -1, fmt.Errorf("sql: %w", err)
	}

	c.Debugf("SQL last migration fetched value is: %v", lastMigration)

	lm2, err := d.migrator.getLastMigration(c)
	if err != nil {
		return -1, err
	}

	return max(lastMigration, lm2), nil
}

func (d sqlMigrator) commitMigration(c *container.Container, data transactionData) error {
	if data.UsedDatasources[dsSQL] {
		dialect := c.SQL.Dialect()

		switch dialect {
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
	}

	// Always commit the transaction regardless of whether SQL was used,
	// to avoid leaving dangling transactions.
	if err := data.SQLTx.Commit(); err != nil {
		return err
	}

	return d.migrator.commitMigration(c, data)
}

func insertMigrationRecord(tx *gofrSql.Tx, query string, version int64, startTime time.Time) error {
	_, err := tx.Exec(query, version, "UP", startTime, time.Since(startTime).Milliseconds())

	return err
}

func (d sqlMigrator) beginTransaction(c *container.Container) transactionData {
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

func (d sqlMigrator) rollback(c *container.Container, data transactionData) {
	if data.SQLTx == nil {
		return
	}

	if err := data.SQLTx.Rollback(); err != nil {
		c.Error("unable to rollback transaction: %v", err)
	}

	d.migrator.rollback(c, data)

	c.Fatalf("Migration %v failed and rolled back", data.MigrationNumber)
}

func (d sqlMigrator) lock(ctx context.Context, cancel context.CancelFunc, c *container.Container, ownerID string) error {
	dialect := c.SQL.Dialect()

	var cleanupQuery, insertQuery string
	if dialect == postgres {
		cleanupQuery = deleteExpiredLocksPostgres
		insertQuery = insertLockPostgres
	} else {
		cleanupQuery = deleteExpiredLocksMySQL
		insertQuery = insertLockMySQL
	}

	for i := 0; ; i++ {
		_, err := c.SQL.ExecContext(ctx, cleanupQuery, time.Now().UTC())
		if err != nil {
			c.Errorf("failed to clean up expired locks: %v", err)
		}

		expiresAt := time.Now().UTC().Add(defaultLockTTL)

		_, err = c.SQL.ExecContext(ctx, insertQuery, lockKey, ownerID, expiresAt)
		if err == nil {
			c.Debug("SQL lock acquired successfully")

			go d.startRefresh(ctx, cancel, c, ownerID, dialect)

			return d.migrator.lock(ctx, cancel, c, ownerID)
		}

		if !isDuplicateKeyError(err) {
			c.Errorf("error while acquiring sql lock: %v", err)

			return errLockAcquisitionFailed
		}

		c.Debugf("SQL lock already held, retrying in %v... (attempt %d)", defaultRetry, i+1)

		select {
		case <-time.After(defaultRetry):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func isDuplicateKeyError(err error) bool {
	msg := strings.ToLower(err.Error())

	return strings.Contains(msg, "duplicate") ||
		strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "integrity constraint") ||
		strings.Contains(msg, "primary key constraint") ||
		strings.Contains(msg, "constraint failed") // SQLite often returns "UNIQUE constraint failed" or "PRIMARY KEY constraint failed"
}

func (sqlMigrator) startRefresh(ctx context.Context, cancel context.CancelFunc, c *container.Container, ownerID, dialect string) {
	ticker := time.NewTicker(defaultRefresh)
	defer ticker.Stop()

	var updateQuery string
	if dialect == postgres {
		updateQuery = updateLockPostgres
	} else {
		updateQuery = updateLockMySQL
	}

	for {
		select {
		case <-ticker.C:
			expiresAt := time.Now().UTC().Add(defaultLockTTL)

			res, err := c.SQL.Exec(updateQuery, expiresAt, lockKey, ownerID)
			if err != nil {
				c.Errorf("failed to refresh SQL lock: %v", err)

				cancel()

				return
			}

			rows, err := res.RowsAffected()
			if err != nil {
				c.Errorf("failed to check rows affected for SQL lock: %v", err)

				cancel()

				return
			}

			if rows == 0 {
				c.Errorf("%v", errSQLLockRefreshFailed)

				cancel()

				return
			}

			c.Debug("SQL lock refreshed successfully")
		case <-ctx.Done():
			return
		}
	}
}

func (d sqlMigrator) unlock(c *container.Container, ownerID string) error {
	dialect := c.SQL.Dialect()

	var deleteQuery string
	if dialect == postgres {
		deleteQuery = deleteLockPostgres
	} else {
		deleteQuery = deleteLockMySQL
	}

	result, err := c.SQL.Exec(deleteQuery, lockKey, ownerID)
	if err != nil {
		c.Errorf("unable to release SQL lock: %v", err)

		return errLockReleaseFailed
	}

	// Check if we actually deleted the lock (i.e., we still owned it)
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		c.Errorf("unable to check SQL lock release status: %v", err)
		return errLockReleaseFailed
	}

	if rowsAffected == 0 {
		c.Errorf("failed to release SQL lock: lock was already released or stolen")
		return errLockReleaseFailed
	}

	c.Debug("SQL lock released successfully")

	return d.migrator.unlock(c, ownerID)
}

func (sqlMigrator) name() string {
	return "SQL"
}
