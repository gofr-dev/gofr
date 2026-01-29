package migration

import (
	"context"
	"database/sql"
	"fmt"
	"os"
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
	createGoFrMigrationLockTable = `CREATE TABLE IF NOT EXISTS gofr_migration_locks (
		lock_id VARCHAR(255) PRIMARY KEY,
		owner_id VARCHAR(255),
		last_heartbeat TIMESTAMP
	);`

	acquireLockQueryMySQL    = `INSERT INTO gofr_migration_locks (lock_id, owner_id, last_heartbeat) VALUES ('migration_lock', ?, ?);`
	acquireLockQueryPostgres = `INSERT INTO gofr_migration_locks (lock_id, owner_id, last_heartbeat) VALUES ('migration_lock', $1, $2);`

	refreshLockQueryMySQL    = `UPDATE gofr_migration_locks SET last_heartbeat = ? WHERE lock_id = 'migration_lock' AND owner_id = ?;`
	refreshLockQueryPostgres = `UPDATE gofr_migration_locks SET last_heartbeat = $1 WHERE lock_id = 'migration_lock' AND owner_id = $2;`

	releaseLockQueryMySQL    = `DELETE FROM gofr_migration_locks WHERE lock_id = 'migration_lock' AND owner_id = ?;`
	releaseLockQueryPostgres = `DELETE FROM gofr_migration_locks WHERE lock_id = 'migration_lock' AND owner_id = $1;`

	checkLockQuery = `SELECT owner_id, last_heartbeat FROM gofr_migration_locks WHERE lock_id = 'migration_lock';`
)

// database/sql is the package imported so named it sqlDS.
type sqlDS struct {
	SQL
}

func (s *sqlDS) apply(m migrator) migrator {
	return &sqlMigrator{
		SQL:      s.SQL,
		migrator: m,
		ownerID:  fmt.Sprintf("%s-%d", getHostname(), time.Now().UnixNano()),
	}
}

type sqlMigrator struct {
	SQL

	migrator
	ownerID string
}

func getHostname() string {
	name, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return name
}

func (d *sqlMigrator) checkAndCreateMigrationTable(c *container.Container) error {
	if _, err := c.SQL.Exec(createSQLGoFrMigrationsTable); err != nil {
		return err
	}

	if _, err := c.SQL.Exec(createGoFrMigrationLockTable); err != nil {
		return err
	}

	return d.migrator.checkAndCreateMigrationTable(c)
}

func (d *sqlMigrator) getLastMigration(c *container.Container) int64 {
	var lastMigration int64

	err := c.SQL.QueryRowContext(context.Background(), getLastSQLGoFrMigration).Scan(&lastMigration)
	if err != nil {
		return -1
	}

	c.Debugf("SQL last migration fetched value is: %v", lastMigration)

	lm2 := d.migrator.getLastMigration(c)
	if lm2 == -1 {
		return -1
	}

	if lm2 > lastMigration {
		return lm2
	}

	return lastMigration
}

func (d *sqlMigrator) commitMigration(c *container.Container, data transactionData) error {
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

func (d *sqlMigrator) lock(c *container.Container) error {
	for {
		var err error
		switch c.SQL.Dialect() {
		case "mysql", "sqlite":
			_, err = c.SQL.Exec(acquireLockQueryMySQL, d.ownerID, time.Now())
		case "postgres":
			_, err = c.SQL.Exec(acquireLockQueryPostgres, d.ownerID, time.Now())
		}

		if err == nil {
			c.Debugf("Acquired migration lock with ownerID: %s", d.ownerID)
			return nil
		}

		// Check if lock is expired
		var ownerID string
		var lastHeartbeat time.Time

		err = c.SQL.QueryRow(checkLockQuery).Scan(&ownerID, &lastHeartbeat)
		if err != nil {
			c.Errorf("failed to check lock status: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		if time.Since(lastHeartbeat) > 15*time.Second {
			c.Infof("Lock expired (owner: %s, last heartbeat: %v), attempting to steal...", ownerID, lastHeartbeat)
			// Expired, try to delete it
			switch c.SQL.Dialect() {
			case "mysql", "sqlite":
				c.SQL.Exec(releaseLockQueryMySQL, ownerID)
			case "postgres":
				c.SQL.Exec(releaseLockQueryPostgres, ownerID)
			}
			continue
		}

		c.Infof("Migration lock held by %s, waiting...", ownerID)
		time.Sleep(2 * time.Second)
	}
}

func (d *sqlMigrator) unlock(c *container.Container) {
	var err error
	switch c.SQL.Dialect() {
	case "mysql", "sqlite":
		_, err = c.SQL.Exec(releaseLockQueryMySQL, d.ownerID)
	case "postgres":
		_, err = c.SQL.Exec(releaseLockQueryPostgres, d.ownerID)
	}

	if err != nil {
		c.Errorf("failed to release migration lock: %v", err)
	} else {
		c.Debugf("Released migration lock for ownerID: %s", d.ownerID)
	}

	d.migrator.unlock(c)
}

func (d *sqlMigrator) refreshLock(c *container.Container) error {
	var err error
	var res sql.Result
	switch c.SQL.Dialect() {
	case "mysql", "sqlite":
		res, err = c.SQL.Exec(refreshLockQueryMySQL, time.Now(), d.ownerID)
	case "postgres":
		res, err = c.SQL.Exec(refreshLockQueryPostgres, time.Now(), d.ownerID)
	}

	if err != nil {
		return err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return fmt.Errorf("failed to refresh lock, lock lost or stolen")
	}

	c.Debugf("Refreshed migration lock for ownerID: %s", d.ownerID)
	return nil
}
