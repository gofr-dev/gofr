package migration

import (
	"time"

	goRedis "github.com/redis/go-redis/v9"
	"gofr.dev/pkg/gofr/container"
	gofrSql "gofr.dev/pkg/gofr/datasource/sql"
)

type Datasource struct {
	Logger

	SQL    db
	Redis  commands
	PubSub client
}

type Migrator interface {
	checkAndCreateMigrationTable(c *container.Container) error
	getLastMigration(c *container.Container) int64

	beginTransaction(c *container.Container) migrationData

	commitMigration(c *container.Container, data migrationData) error
	rollback(c *container.Container, data migrationData)
}

type Datasources interface {
	apply(m Migrator) Migrator
}

func (d Datasource) checkAndCreateMigrationTable(*container.Container) error {
	return nil
}

func (d Datasource) getLastMigration(*container.Container) int64 {
	return 0
}

func (d Datasource) beginTransaction(*container.Container) migrationData {
	return migrationData{}
}

func (d Datasource) commitMigration(c *container.Container, data migrationData) error {
	c.Infof("Migration %v ran successfully", data.MigrationNumber)

	return nil
}

func (d Datasource) rollback(*container.Container, migrationData) {}

type migrationData struct {
	StartTime       time.Time
	MigrationNumber int64

	SQLTx   *gofrSql.Tx
	RedisTx goRedis.Pipeliner
}
