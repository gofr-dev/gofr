package migrators

import (
	"context"
	"gofr.dev/pkg/gofr/migration"
	"time"

	"gofr.dev/pkg/gofr/container"
	gofrSql "gofr.dev/pkg/gofr/datasource/sql"
)

const (
	createMySQLGoFrMigrationsTable = `CREATE TABLE IF NOT EXISTS gofr_migrations (
    version BIGINT not null ,
    method VARCHAR(4) not null ,
    start_time TIMESTAMP not null ,
    duration BIGINT,
    constraint primary_key primary key (version, method)
);`

	checkMySQLGoFrMigrationsTable = `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'gofr_migrations');`

	getLastMySQLGoFrMigration = `SELECT COALESCE(MAX(version), 0) FROM gofr_migrations;`

	insertGoFrMigrationRow = `INSERT INTO gofr_migrations (version, method, start_time) VALUES (?, ?, ?);`

	updateDurationInMigrationRecord = `UPDATE gofr_migrations SET duration = ? WHERE version = ? AND method = 'UP' AND duration IS NULL;`
)

type sqlDB struct {
}

func SQL() migration.Migrator {
	return sqlDB{}
}

func (s sqlDB) Run(keys []int64, migrationsMap map[int64]migration.Migrate, c *container.Container) {
	migrator := migration.Datasource{Redis: c.Redis, Logger: c.Logger, DB: c.DB}

	// Ensure migration table exists
	ensureMigrationTableExists(c)

	// Get last migration version
	lastMigration := getLastMigration(c)

	// Iterate over migrations
	for _, version := range keys {
		if version <= lastMigration {
			continue
		}

		// Begin transaction
		tx, err := c.DB.Begin()
		if err != nil {
			c.Logger.Error("unable to begin transaction: %v", err)

			return
		}

		// Insert migration record
		startTime := time.Now().UTC()
		if err := insertMigrationRecord(tx, version, startTime); err != nil {
			c.Logger.Errorf("unable to insert migration record: %v", err)
			rollbackAndLog(c, tx)

			return
		}

		// Run migration
		if err := migrationsMap[version].UP(migrator); err != nil {
			c.Logger.Errorf("unable to run migration: %v", err)
			rollbackAndLog(c, tx)

			return
		}

		// Update migration duration
		if err := updateMigrationDuration(tx, version, startTime); err != nil {
			c.Logger.Errorf("unable to update migration duration: %v", err)
			rollbackAndLog(c, tx)

			return
		}

		// Commit transaction
		if err := tx.Commit(); err != nil {
			c.Logger.Error("unable to commit transaction: %v", err)

			return
		}

		c.Logger.Infof("MIGRATION [%v] ran successfully", version)
	}
}

func ensureMigrationTableExists(c *container.Container) {
	var exists int

	_ = c.DB.QueryRow(checkMySQLGoFrMigrationsTable).Scan(&exists)

	if exists != 1 {
		if _, err := c.DB.Exec(createMySQLGoFrMigrationsTable); err != nil {
			c.Logger.Errorf("unable to create gofr_migrations table: %v", err)
		}
	}
}

func getLastMigration(c *container.Container) int64 {
	var lastMigration int64

	_ = c.DB.QueryRowContext(context.Background(), getLastMySQLGoFrMigration).Scan(&lastMigration)

	return lastMigration
}

func insertMigrationRecord(tx migration.DB, version int64, startTime time.Time) error {
	_, err := tx.Exec(insertGoFrMigrationRow, version, "UP", startTime)

	return err
}

func updateMigrationDuration(tx migration.DB, version int64, startTime time.Time) error {
	_, err := tx.Exec(updateDurationInMigrationRecord, time.Since(startTime).Milliseconds(), version)

	return err
}

func rollbackAndLog(c *container.Container, tx *gofrSql.Tx) {
	if err := tx.Rollback(); err != nil {
		c.Logger.Error("unable to rollback transaction: %v", err)
	}
}
