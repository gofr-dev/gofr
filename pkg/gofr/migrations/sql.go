package migrations

import (
	"context"
	"database/sql"
	"time"

	"gofr.dev/pkg/gofr/container"
	gofrSql "gofr.dev/pkg/gofr/datasource/sql"
)

type sqlDB struct {
}

func NewSQL() Migrator {
	return sqlDB{}
}

type SQL interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	Exec(query string, args ...interface{}) (sql.Result, error)
}

func (s sqlDB) Migrate(keys []int64, migrationsMap map[int64]Migration, c *container.Container) {
	migrator := Datasource{Redis: c.Redis, Logger: c.Logger, DB: c.DB}

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
		startTime := time.Now()
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

func insertMigrationRecord(tx SQL, version int64, startTime time.Time) error {
	_, err := tx.Exec(insertGoFrMigrationRow, version, "UP", startTime)

	return err
}

func updateMigrationDuration(tx SQL, version int64, startTime time.Time) error {
	_, err := tx.Exec(updateDurationInMigrationRecord, time.Since(startTime).Milliseconds(), version)

	return err
}

func rollbackAndLog(c *container.Container, tx *gofrSql.Tx) {
	if err := tx.Rollback(); err != nil {
		c.Logger.Error("unable to rollback transaction: %v", err)
	}
}
