package migration

import (
	"context"
	"fmt"
	"time"

	"gofr.dev/pkg/gofr/container"
)

type cassandraDS struct {
	container.CassandraWithContext
}

type cassandraMigrator struct {
	container.CassandraWithContext

	migrator
}

func (cs cassandraDS) apply(m migrator) migrator {
	return cassandraMigrator{
		CassandraWithContext: cs.CassandraWithContext,
		migrator:             m,
	}
}

const (
	checkAndCreateCassandraMigrationTable = `CREATE TABLE IF NOT EXISTS gofr_migrations (version bigint,
    method text, start_time timestamp, duration bigint, PRIMARY KEY (version, method));`

	getCassandraGoFrMigrationVersions = `SELECT version FROM gofr_migrations`

	insertCassandraGoFrMigrationRow = `INSERT INTO gofr_migrations (version, method, start_time, duration) VALUES (?, ?, ?, ?);`
)

func (cs cassandraMigrator) checkAndCreateMigrationTable(c *container.Container) error {
	if err := c.Cassandra.ExecWithCtx(context.Background(), checkAndCreateCassandraMigrationTable); err != nil {
		return err
	}

	return cs.migrator.checkAndCreateMigrationTable(c)
}

func (cs cassandraMigrator) getAppliedMigrations(c *container.Container) (map[int64]struct{}, error) {
	var versions []int64

	err := c.Cassandra.QueryWithCtx(context.Background(), &versions, getCassandraGoFrMigrationVersions)
	if err != nil {
		return nil, fmt.Errorf("cassandra: %w", err)
	}

	c.Debugf("cassandra applied migrations fetched: %v", versions)

	applied, err := cs.migrator.getAppliedMigrations(c)
	if err != nil {
		return nil, err
	}

	for _, v := range versions {
		applied[v] = struct{}{}
	}

	return applied, nil
}

func (cs cassandraMigrator) beginTransaction(c *container.Container) transactionData {
	cmt := cs.migrator.beginTransaction(c)

	c.Debug("cassandra migrator begin successfully")

	return cmt
}

func (cs cassandraMigrator) commitMigration(c *container.Container, data *transactionData) error {
	if data.UsedDatasources[dsCassandra] {
		err := cs.CassandraWithContext.ExecWithCtx(context.Background(), insertCassandraGoFrMigrationRow, data.MigrationNumber,
			migrationMethodUP, data.StartTime, time.Since(data.StartTime).Milliseconds())
		if err != nil {
			return err
		}

		c.Debugf("inserted record for migration %v in cassandra gofr_migrations table", data.MigrationNumber)
	}

	return cs.migrator.commitMigration(c, data)
}

func (cs cassandraMigrator) rollback(c *container.Container, data *transactionData) {
	cs.migrator.rollback(c, data)

	c.Fatalf("migration %v failed and rolled back", data.MigrationNumber)
}

func (cs cassandraMigrator) lock(ctx context.Context, cancel context.CancelFunc, c *container.Container, ownerID string) error {
	return cs.migrator.lock(ctx, cancel, c, ownerID)
}

func (cs cassandraMigrator) unlock(c *container.Container, ownerID string) error {
	return cs.migrator.unlock(c, ownerID)
}

func (cassandraMigrator) name() string {
	return "Cassandra"
}
