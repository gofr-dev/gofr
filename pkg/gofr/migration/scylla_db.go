package migration

import (
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
	return &scyllaMigrator{
		ScyllaDB: ds.ScyllaDB,
		migrator: m,
	}
}

const (
	scyllaDBMigrationTable = "gofr_migrations"
)

func (s *scyllaMigrator) checkAndCreateMigrationTable(c *container.Container) error {
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

	return nil
}

type migrationRow struct {
	Version int64 `db:"version"`
}

func (s *scyllaMigrator) getLastMigration(c *container.Container) int64 {
	var migrations []migrationRow

	query := fmt.Sprintf("SELECT version FROM %s", scyllaDBMigrationTable)

	err := s.ScyllaDB.Query(&migrations, query)
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

	lm2 := s.migrator.getLastMigration(c)

	return max(lastVersion, lm2)
}

func (s *scyllaMigrator) beginTransaction(c *container.Container) transactionData {
	return s.migrator.beginTransaction(c)
}

func (s *scyllaMigrator) commitMigration(c *container.Container, data transactionData) error {
	insertStmt := fmt.Sprintf(`
		INSERT INTO %s (version, method, start_time, duration)
		VALUES (?, ?, ?, ?);
	`, scyllaDBMigrationTable)

	err := s.ScyllaDB.Exec(insertStmt,
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

	return s.migrator.commitMigration(c, data)
}

func (s *scyllaMigrator) rollback(c *container.Container, data transactionData) {
	s.migrator.rollback(c, data)
	c.Fatalf("Migration %v failed.", data.MigrationNumber)
}

func (*scyllaMigrator) AcquireLock(*container.Container) error {
	return nil
}

func (*scyllaMigrator) ReleaseLock(*container.Container) error {
	return nil
}

func (*scyllaMigrator) Name() string {
	return "ScyllaDB"
}
