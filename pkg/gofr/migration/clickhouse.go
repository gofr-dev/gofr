package migration

import (
	"context"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/datasource"
	"time"
)

type clickHouse struct {
	datasource.Clickhouse
}

type clickHouseMigrator struct {
	datasource.Clickhouse

	Manager
}

func (ch clickHouse) Apply(m Manager) Manager {
	return clickHouseMigrator{
		Clickhouse: ch.Clickhouse,
		Manager:    m,
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

func (ch clickHouseMigrator) CheckAndCreateMigrationTable(c *container.Container) error {
	if err := c.Clickhouse.Exec(context.Background(), CheckAndCreateChMigrationTable); err != nil {
		return err
	}

	return ch.Manager.CheckAndCreateMigrationTable(c)
}

func (ch clickHouseMigrator) GetLastMigration(c *container.Container) int64 {
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

	lm2 := ch.Manager.GetLastMigration(c)

	if lm2 > lastMigration {
		return lm2
	}

	return lastMigration
}

func (ch clickHouseMigrator) BeginTransaction(c *container.Container) transactionData {
	cmt := ch.Manager.BeginTransaction(c)

	c.Debug("Clickhouse Migrator begin successfully")

	return cmt
}

func (ch clickHouseMigrator) CommitMigration(c *container.Container, data transactionData) error {
	err := ch.Clickhouse.Exec(context.Background(), insertChGoFrMigrationRow, data.MigrationNumber,
		"UP", data.StartTime, time.Since(data.StartTime).Milliseconds())
	if err != nil {
		return err
	}

	c.Debugf("inserted record for migration %v in clickhouse gofr_migrations table", data.MigrationNumber)

	return ch.Manager.CommitMigration(c, data)
}

func (ch clickHouseMigrator) Rollback(c *container.Container, data transactionData) {
	c.Errorf("Migration %v failed", data.MigrationNumber)

	ch.Manager.Rollback(c, data)
}
