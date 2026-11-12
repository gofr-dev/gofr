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

func (s scyllaMigrator) checkAndCreateMigrationTable(c *container.Container) error {
	createTableQuery := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			version bigint PRIMARY KEY,
			method text,
			start_time timestamp,
			duration bigint
		);
	`, scyllaDBMigrationTable)

	err := s.ScyllaDB.Exec(createTableQuery)
	if err != nil {
		c.Errorf("Failed to create migration table: %v", err)
		return err
	}

	return s.migrator.checkAndCreateMigrationTable(c)
}

type migrationRow struct {
	Version int64 `db:"version"`
}

func (s scyllaMigrator) getAppliedMigrations(c *container.Container) (map[int64]struct{}, error) {
	var migrations []migrationRow

	query := fmt.Sprintf("SELECT version FROM %s", scyllaDBMigrationTable)

	err := s.ScyllaDB.Query(&migrations, query)
	if err != nil {
		return nil, fmt.Errorf("scylladb: %w", err)
	}

	c.Debugf("ScyllaDB applied migrations fetched: %v", len(migrations))

	applied, err := s.migrator.getAppliedMigrations(c)
	if err != nil {
		return nil, err
	}

	for _, m := range migrations {
		applied[m.Version] = struct{}{}
	}

	return applied, nil
}

func (s scyllaMigrator) beginTransaction(c *container.Container) transactionData {
	return s.migrator.beginTransaction(c)
}

func (s scyllaMigrator) commitMigration(c *container.Container, data *transactionData) error {
	if data.UsedDatasources[dsScyllaDB] {
		insertStmt := fmt.Sprintf(`
		INSERT INTO %s (version, method, start_time, duration)
		VALUES (?, ?, ?, ?);
	`, scyllaDBMigrationTable)

		err := s.ScyllaDB.Exec(insertStmt,
			data.MigrationNumber,
			migrationMethodUP,
			data.StartTime,
			time.Since(data.StartTime).Milliseconds(),
		)
		if err != nil {
			c.Errorf("Failed to insert migration record: %v", err)
			return err
		}

		c.Debugf("Inserted migration record for version %v into ScyllaDB", data.MigrationNumber)
	}

	return s.migrator.commitMigration(c, data)
}

func (s scyllaMigrator) rollback(c *container.Container, data *transactionData) {
	s.migrator.rollback(c, data)
	c.Fatalf("Migration %v failed.", data.MigrationNumber)
}

func (s scyllaMigrator) lock(ctx context.Context, cancel context.CancelFunc, c *container.Container, ownerID string) error {
	return s.migrator.lock(ctx, cancel, c, ownerID)
}

func (s scyllaMigrator) unlock(c *container.Container, ownerID string) error {
	return s.migrator.unlock(c, ownerID)
}

func (scyllaMigrator) name() string {
	return "ScyllaDB"
}
