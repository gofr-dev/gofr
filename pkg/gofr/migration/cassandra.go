package migration

import (
	"fmt"
	"time"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/datasource"
)

const (
	createCassandraGoFrMigrationsTable = `CREATE TABLE IF NOT EXISTS gofr_migrations (
    version BIGINT not null ,
    method VARCHAR(4) not null ,
    start_time TIMESTAMP not null ,
    duration BIGINT,
    constraint primary_key primary key (version, method)
);`

	getLastGoFrMigrationCassandra = `SELECT MAX(version) as version FROM gofr_migrations;`

	insertGoFrMigrationRowCassandra = `INSERT INTO gofr_migrations (version, method, start_time,duration) VALUES (?, ?, ?, ?) 
IF NOT EXISTS;`
)

type cassandra struct {
	datasource.Cassandra
}

func newCassandra(c datasource.Cassandra) datasource.Cassandra {
	return &cassandra{Cassandra: c}
}

func (c *cassandra) Query(stmt string, values ...interface{}) datasource.Query {
	return c.Cassandra.Query(stmt, values...)
}

func (c *cassandra) Iter(stmt string, values ...interface{}) datasource.Iter {
	return c.Cassandra.Iter(stmt, values...)
}

type cassandraMigratorObject struct {
	datasource.Cassandra
}

type cassandraMigrator struct {
	datasource.Cassandra

	Migrator
}

func (c cassandraMigratorObject) apply(m Migrator) Migrator {
	return cassandraMigrator{
		Cassandra: c.Cassandra,
		Migrator:  m,
	}
}

func (d cassandraMigrator) checkAndCreateMigrationTable(c *container.Container) error {
	if err := d.Cassandra.Query(createCassandraGoFrMigrationsTable).Exec(); err != nil {
		return err
	}

	return d.Migrator.checkAndCreateMigrationTable(c)
}

func (d cassandraMigrator) getLastMigration(c *container.Container) int64 {
	var lastMigration int64

	iter := d.Cassandra.Iter(getLastGoFrMigrationCassandra)

	defer iter.Close()

	if !iter.Scan(&lastMigration) {
		return 0
	}

	c.Logger.Debugf("Cassandra last migration fetched value is: %v", lastMigration)

	last := d.Migrator.getLastMigration(c)
	if last > lastMigration {
		return last
	}

	return lastMigration
}

func (d cassandraMigrator) beginTransaction(c *container.Container) migrationData {
	return d.Migrator.beginTransaction(c)
}

func (d cassandraMigrator) commitMigration(c *container.Container, data migrationData) error {
	applied, err := d.Cassandra.Query(insertGoFrMigrationRowCassandra, data.MigrationNumber, "UP", data.StartTime,
		time.Since(data.StartTime).Milliseconds()).ScanCAS()
	if err != nil {
		return err
	}

	if !applied {
		return migrationExistsErr(data.MigrationNumber)
	}

	return d.Migrator.commitMigration(c, data)
}

func (d cassandraMigrator) rollback(c *container.Container, data migrationData) {
	d.Migrator.rollback(c, data)
}

type migrationExistsErr int64

func (m migrationExistsErr) Error() string {
	return fmt.Sprintf("migration %d already exists", m)
}
