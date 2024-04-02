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
	CheckAndCreateMigrationTable(c *container.Container) error
	GetLastMigration(c *container.Container) int64

	BeginTransaction(c *container.Container) migrationData

	CommitMigration(c *container.Container, data migrationData) error
	Rollback(c *container.Container, data migrationData)
}

type Datasources interface {
	apply(m Migrator) Migrator
}

func (d Datasource) CheckAndCreateMigrationTable(*container.Container) error {
	return nil
}

func (d Datasource) GetLastMigration(*container.Container) int64 {
	return 0
}

func (d Datasource) BeginTransaction(*container.Container) migrationData {
	return migrationData{}
}

func (d Datasource) CommitMigration(c *container.Container, data migrationData) error {
	c.Infof("Migration %v ran successfully", data.MigrationNumber)

	return nil
}

func (d Datasource) Rollback(*container.Container, migrationData) {}

type migrationData struct {
	StartTime       time.Time
	MigrationNumber int64

	SQLTx   *gofrSql.Tx
	RedisTx goRedis.Pipeliner
}
