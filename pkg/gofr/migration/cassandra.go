package migration

import (
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

func (c *cassandra) Query(result interface{}, stmt string, values ...interface{}) error {
	return c.Cassandra.Query(result, stmt, values...)
}

func (c *cassandra) QueryRow(result interface{}, stmt string, values ...interface{}) error {
	return c.Cassandra.QueryRow(result, stmt, values...)
}

func (c *cassandra) Exec(stmt string, values ...interface{}) error {
	return c.Cassandra.Exec(stmt, values...)
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
	if err := d.Cassandra.Exec(createCassandraGoFrMigrationsTable); err != nil {
		return err
	}

	return d.Migrator.checkAndCreateMigrationTable(c)
}

func (d cassandraMigrator) getLastMigration(c *container.Container) int64 {
	var lastMigration int64

	err := d.Cassandra.QueryRow(&lastMigration, getLastGoFrMigrationCassandra)
	if err != nil {
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
	err := d.Cassandra.Exec(insertGoFrMigrationRowCassandra, data.MigrationNumber, "UP", data.StartTime,
		time.Since(data.StartTime).Milliseconds())
	if err != nil {
		return err
	}

	return d.Migrator.commitMigration(c, data)
}

func (d cassandraMigrator) rollback(c *container.Container, data migrationData) {
	d.Migrator.rollback(c, data)
}
