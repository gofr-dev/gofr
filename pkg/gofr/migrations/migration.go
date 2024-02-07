package migrations

import (
	"context"
	"database/sql"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/datasource/redis"
)

type TransactionDB int

const (
	SqlDB TransactionDB = iota + 1
)

type Logger interface {
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Log(args ...interface{})
	Logf(format string, args ...interface{})
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Notice(args ...interface{})
	Noticef(format string, args ...interface{})
	Warn(args ...interface{})
	Warnf(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
}

type Migrator struct {
	SQL
	*redis.Redis

	Logger
}

type SQL interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	Exec(query string, args ...interface{}) (sql.Result, error)
}

type MigrateFunc func(container Migrator) error

type Migration struct {
	UP   MigrateFunc
	DOWN MigrateFunc
}

func Migrate(migrationsMap map[int64]Migration, dialect TransactionDB, container *container.Container) {
	migrator := Migrator{Redis: container.Redis, Logger: container.Logger}

	switch dialect {
	case SqlDB:
		migrator.SQL = container.DB
	}

	_, err := container.DB.Exec(createGoFrMigrationsTable)
	container.Logger.Error(err)

	for _, value := range migrationsMap {
		var (
			tx  *sql.Tx
			err error
		)

		if migrator.SQL != nil {
			tx, err = container.DB.BeginTx(context.Background(), nil)
			if err != nil {
				return
			}

			tx.Exec(`INSERT INTO gofr_migrations (app_name, version, start_time, duration, method) VALUES (?, ?, ?, ?, ?);`)

			migrator.SQL = tx
		}

		value.UP(migrator)

		if migrator.SQL != nil {
			// update gofr migrations table with endtime
			tx.Exec("")
			tx.Commit()
		}
	}
}
