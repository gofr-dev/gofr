package migration

import (
	"context"
	"fmt"
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

	getChGoFrMigrationVersions = `SELECT DISTINCT version FROM gofr_migrations;`

	insertChGoFrMigrationRow = `INSERT INTO gofr_migrations (version, method, start_time, duration) VALUES (?, ?, ?, ?);`
)

func (ch clickHouseMigrator) checkAndCreateMigrationTable(c *container.Container) error {
	if err := c.Clickhouse.Exec(context.Background(), CheckAndCreateChMigrationTable); err != nil {
		return err
	}

	return ch.migrator.checkAndCreateMigrationTable(c)
}

func (ch clickHouseMigrator) getAppliedMigrations(c *container.Container) (map[int64]struct{}, error) {
	type appliedMigration struct {
		Version int64 `ch:"version"`
	}

	var migrations []appliedMigration

	err := c.Clickhouse.Select(context.Background(), &migrations, getChGoFrMigrationVersions)
	if err != nil {
		return nil, fmt.Errorf("clickhouse: %w", err)
	}

	c.Debugf("Clickhouse applied migrations fetched: %v", len(migrations))

	applied, err := ch.migrator.getAppliedMigrations(c)
	if err != nil {
		return nil, err
	}

	for _, m := range migrations {
		applied[m.Version] = struct{}{}
	}

	return applied, nil
}

func (ch clickHouseMigrator) beginTransaction(c *container.Container) transactionData {
	cmt := ch.migrator.beginTransaction(c)

	c.Debug("Clickhouse Migrator begin successfully")

	return cmt
}

func (ch clickHouseMigrator) commitMigration(c *container.Container, data *transactionData) error {
	if data.UsedDatasources[dsClickhouse] {
		err := ch.Clickhouse.Exec(context.Background(), insertChGoFrMigrationRow, data.MigrationNumber,
			migrationMethodUP, data.StartTime, time.Since(data.StartTime).Milliseconds())
		if err != nil {
			return err
		}

		c.Debugf("inserted record for migration %v in clickhouse gofr_migrations table", data.MigrationNumber)
	}

	return ch.migrator.commitMigration(c, data)
}

func (ch clickHouseMigrator) rollback(c *container.Container, data *transactionData) {
	ch.migrator.rollback(c, data)

	c.Fatalf("migration %v failed and rolled back", data.MigrationNumber)
}

func (ch clickHouseMigrator) lock(ctx context.Context, cancel context.CancelFunc, c *container.Container, ownerID string) error {
	return ch.migrator.lock(ctx, cancel, c, ownerID)
}

func (ch clickHouseMigrator) unlock(c *container.Container, ownerID string) error {
	return ch.migrator.unlock(c, ownerID)
}

func (clickHouseMigrator) name() string {
	return "Clickhouse"
}
