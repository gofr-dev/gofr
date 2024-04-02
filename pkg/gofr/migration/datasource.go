package migration

import (
	goRedis "github.com/redis/go-redis/v9"
	"gofr.dev/pkg/gofr/container"
	gofrSql "gofr.dev/pkg/gofr/datasource/sql"
	"time"
)

type Datasource struct {
	Logger

	SQL    db
	Redis  commands
	PubSub client
}

type Migrator interface {
	CheckAndCreateMigrationTable(c container.Interface) error
	GetLastMigration(c container.Interface) int64
	CommitMigration(c container.Interface, data commit) error
	Rollback(c container.Interface, data commit) error
}

type Datasources interface {
	apply(m Migrator) Migrator
}

func (d Datasource) CheckAndCreateMigrationTable(c container.Interface) error {
	return nil
}

func (d Datasource) GetLastMigration(c container.Interface) error {
	return nil
}

func (d Datasource) CommitMigration(c container.Interface, data commit) error {
	return nil
}

func (d Datasource) Rollback(c container.Interface) error {
	return nil
}

type commit struct {
	StartTime       time.Time
	MigrationNumber int64

	SQLTx   *gofrSql.Tx
	RedisTx goRedis.Pipeliner
}
