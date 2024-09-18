package migration

import (
	"time"

	"gofr.dev/pkg/gofr/container"
)

type cassandraDS struct {
	Cassandra
}

type cassandraMigrator struct {
	Cassandra

	migrator
}

func (cs cassandraDS) apply(m migrator) migrator {
	return cassandraMigrator{
		Cassandra: cs.Cassandra,
		migrator:  m,
	}
}

const (
	CheckAndCreateCassandraMigrationTable = `CREATE TABLE IF NOT EXISTS gofr_migrations (
version bigint,
method text,
start_time timestamp,
duration bigint,
PRIMARY KEY (version, method)
);`

	getLastCassandraGoFrMigration = `SELECT version FROM gofr_migrations`

	insertCassandraGoFrMigrationRow = `INSERT INTO gofr_migrations (version, method, start_time, duration) VALUES (?, ?, ?, ?);`
)

func (cs cassandraMigrator) checkAndCreateMigrationTable(c *container.Container) error {
	if err := c.Cassandra.Exec(CheckAndCreateCassandraMigrationTable); err != nil {
		return err
	}

	return cs.migrator.checkAndCreateMigrationTable(c)
}

func (cs cassandraMigrator) getLastMigration(c *container.Container) int64 {
	var lastMigration int64 // Default to 0 if no migrations found

	var lastMigrations []int64

	err := c.Cassandra.Query(&lastMigrations, getLastCassandraGoFrMigration)
	if err != nil {
		return 0
	}

	for _, version := range lastMigrations {
		if version > lastMigration {
			lastMigration = version
		}
	}

	c.Debugf("Cassandra last migration fetched value is: %v", lastMigration)

	lm2 := cs.migrator.getLastMigration(c)

	if lm2 > lastMigration {
		return lm2
	}

	return lastMigration
}

func (cs cassandraMigrator) beginTransaction(c *container.Container) transactionData {
	cmt := cs.migrator.beginTransaction(c)

	c.Debug("Cassandra Migrator begin successfully")

	return cmt
}

func (cs cassandraMigrator) commitMigration(c *container.Container, data transactionData) error {
	err := cs.Cassandra.Exec(insertCassandraGoFrMigrationRow, data.MigrationNumber,
		"UP", data.StartTime, time.Since(data.StartTime).Milliseconds())
	if err != nil {
		return err
	}

	c.Debugf("inserted record for migration %v in cassandra gofr_migrations table", data.MigrationNumber)

	return cs.migrator.commitMigration(c, data)
}

func (cs cassandraMigrator) rollback(c *container.Container, data transactionData) {
	cs.migrator.rollback(c, data)

	c.Fatalf("migration %v failed and rolled back", data.MigrationNumber)
}
