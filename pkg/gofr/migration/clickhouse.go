package migration

import (
	"context"
	"time"

	"gofr.dev/pkg/gofr/container"
)

type clickHouseDS struct {
	Clickhouse
}

type clickHouseMigrator struct {
	Clickhouse

	migrator
}

func (ch clickHouseDS) apply(m migrator) migrator {
	return clickHouseMigrator{
		Clickhouse: ch.Clickhouse,
		migrator:   m,
	}
}

const (
	CheckAndCreateChMigrationTable = `CREATE TABLE IF NOT EXISTS gofr_migrations
(
    version    Int64     NOT NULL,
    method     String    NOT NULL,
    start_time DateTime  NOT NULL,
    duration   Int64     NULL,
    PRIMARY KEY (version, method)
) ENGINE = MergeTree()
ORDER BY (version, method);
`

	getLastChGoFrMigration = `SELECT COALESCE(MAX(version), 0) as last_migration FROM gofr_migrations;`

	insertChGoFrMigrationRow = `INSERT INTO gofr_migrations (version, method, start_time, duration) VALUES (?, ?, ?, ?);`
)

func (ch clickHouseMigrator) checkAndCreateMigrationTable(c *container.Container) error {
	if err := c.Clickhouse.Exec(context.Background(), CheckAndCreateChMigrationTable); err != nil {
		return err
	}

	return ch.migrator.checkAndCreateMigrationTable(c)
}

func (ch clickHouseMigrator) getLastMigration(c *container.Container) int64 {
	type LastMigration struct {
		Timestamp int64 `ch:"last_migration"`
	}

	var lastMigrations []LastMigration

	var lastMigration int64

	err := c.Clickhouse.Select(context.Background(), &lastMigrations, getLastChGoFrMigration)
	if err != nil {
		return 0
	}

	c.Debugf("SQL last migration fetched value is: %v", lastMigration)

	if len(lastMigrations) != 0 {
		lastMigration = lastMigrations[0].Timestamp
	}

	lm2 := ch.migrator.getLastMigration(c)

	if lm2 > lastMigration {
		return lm2
	}

	return lastMigration
}

func (ch clickHouseMigrator) beginTransaction(c *container.Container) transactionData {
	cmt := ch.migrator.beginTransaction(c)

	c.Debug("Clickhouse Migrator begin successfully")

	return cmt
}

func (ch clickHouseMigrator) commitMigration(c *container.Container, data transactionData) error {
	err := ch.Clickhouse.Exec(context.Background(), insertChGoFrMigrationRow, data.MigrationNumber,
		"UP", data.StartTime, time.Since(data.StartTime).Milliseconds())
	if err != nil {
		return err
	}

	c.Debugf("inserted record for migration %v in clickhouse gofr_migrations table", data.MigrationNumber)

	return ch.migrator.commitMigration(c, data)
}

func (ch clickHouseMigrator) rollback(c *container.Container, data transactionData) {
	c.Errorf("Migration %v failed", data.MigrationNumber)

	ch.migrator.rollback(c, data)
}
