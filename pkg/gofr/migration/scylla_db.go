package migration

import (
	"context"
	"fmt"
	"time"

	"gofr.dev/pkg/gofr/container"
)

type scyllaDS struct {
	ScyllaDB
}

type scyllaMigrator struct {
	ScyllaDB
	migrator
}

func (ds scyllaDS) apply(m migrator) migrator {
	return scyllaMigrator{
		ScyllaDB: ds.ScyllaDB,
		migrator: m,
	}
}

const (
	scyllaDBMigrationTable = "gofr_migrations"
)

func (sm scyllaMigrator) checkAndCreateMigrationTable(c *container.Container) error {
	createTableQuery := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			version bigint PRIMARY KEY,
			method text,
			start_time timestamp,
			duration bigint
		);
	`, scyllaDBMigrationTable)

	err := sm.ScyllaDB.Exec(createTableQuery)
	if err != nil {
		c.Errorf("Failed to create migration table: %v", err)
		return err
	}

	return sm.migrator.checkAndCreateMigrationTable(c)
}

type migrationRow struct {
	Version int64 `db:"version"`
}

func (sm scyllaMigrator) getLastMigration(c *container.Container) int64 {
	var migrations []migrationRow

	query := fmt.Sprintf("SELECT version FROM %s", scyllaDBMigrationTable)

	err := sm.ScyllaDB.Query(&migrations, query)
	if err != nil {
		c.Errorf("Failed to fetch migrations from ScyllaDB: %v", err)
		return 0
	}

	var lastVersion int64
	for _, m := range migrations {
		if m.Version > lastVersion {
			lastVersion = m.Version
		}
	}

	c.Debugf("ScyllaDB last migration fetched value is: %v", lastVersion)

	lm2 := sm.migrator.getLastMigration(c)

	return max(lastVersion, lm2)
}

func (sm scyllaMigrator) beginTransaction(c *container.Container) transactionData {
	return sm.migrator.beginTransaction(c)
}

func (sm scyllaMigrator) commitMigration(c *container.Container, data transactionData) error {
	insertStmt := fmt.Sprintf(`
		INSERT INTO %s (version, method, start_time, duration)
		VALUES (?, ?, ?, ?);
	`, scyllaDBMigrationTable)

	err := sm.ScyllaDB.Exec(insertStmt,
		data.MigrationNumber,
		"UP",
		data.StartTime,
		time.Since(data.StartTime).Milliseconds(),
	)
	if err != nil {
		c.Errorf("Failed to insert migration record: %v", err)
		return err
	}

	c.Debugf("Inserted migration record for version %v into ScyllaDB", data.MigrationNumber)

	return sm.migrator.commitMigration(c, data)
}

func (sm scyllaMigrator) rollback(c *container.Container, data transactionData) {
	sm.migrator.rollback(c, data)
	c.Fatalf("Migration %v failed.", data.MigrationNumber)
}

func (sm scyllaMigrator) lock(ctx context.Context, cancel context.CancelFunc, c *container.Container, ownerID string) error {
	return sm.migrator.lock(ctx, cancel, c, ownerID)
}

func (sm scyllaMigrator) unlock(c *container.Container, ownerID string) error {
	return sm.migrator.unlock(c, ownerID)
}

func (scyllaMigrator) name() string {
	return "ScyllaDB"
}
