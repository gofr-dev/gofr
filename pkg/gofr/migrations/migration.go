package migrations

import (
	"context"
	"database/sql"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/datasource/redis"
	gofrSql "gofr.dev/pkg/gofr/datasource/sql"
	"time"

	"github.com/gogo/protobuf/sortkeys"
)

type TransactionDB int

const (
	SqlDB TransactionDB = iota + 1
)

type Logger interface {
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
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
	var startTime time.Time

	switch dialect {
	case SqlDB:
		migrator.SQL = container.DB
	}

	// sort the migration based on timestamp, for version based migration, in ascending order
	keys := make([]int64, 0, len(migrationsMap))

	for k := range migrationsMap {
		keys = append(keys, k)
	}

	sortkeys.Int64s(keys)

	var exists int

	container.DB.QueryRow("SELECT EXISTS ( SELECT 1 FROM information_schema.tables WHERE table_name = 'gofr_migrations' );").Scan(&exists)

	if exists != 1 {
		_, err := container.DB.Exec(createGoFrMigrationsTable)
		if err != nil {
			container.Logger.Error("unable to create gofr_migrations table : %v", err)
			return
		}
	}

	var lastMigration int64

	container.DB.QueryRowContext(context.Background(), `select max(version) from gofr_migrations;`).Scan(&lastMigration)

	for _, value := range keys {
		if value <= lastMigration {
			continue
		}

		var (
			tx  *gofrSql.Tx
			err error
		)

		if migrator.SQL != nil {
			tx, err = container.DB.Begin()
			if err != nil {
				return
			}

			startTime = time.Now()

			_, err := tx.Exec(`INSERT INTO gofr_migrations (app_name, version, start_time, duration, method) VALUES (?, ?, ?, ?, ?);`, container.GetAppName(), value, startTime, nil, "UP")
			if err != nil {
				container.Error("unable to insert into gofr_migrations table :%v", err)

				err := tx.Rollback()
				if err != nil {
					container.Error("unable to rollback :%v", err)

					return
				}
			}

			migrator.SQL = tx.Tx
		}

		err = migrationsMap[value].UP(migrator)
		if err != nil {
			container.Error("unable to run migration :%v", err)

			err := tx.Rollback()

			if err != nil {
				container.Error("unable to rollback :%v", err)

				return
			}
		}

		if migrator.SQL != nil {
			// update gofr migrations table with endtime
			_, err := tx.Exec(`UPDATE gofr_migrations SET duration = ? WHERE app_name = ? AND version = ? AND duration IS NULL AND method = 'UP';`, time.Since(startTime).Milliseconds(), container.GetAppName(), value)
			if err != nil {
				container.Error("unable to insert into gofr migration :%v", err)

				err := tx.Rollback()
				if err != nil {
					container.Error("unable to rollback :%v", err)

					return
				}
			}
		}

		container.Logger.Infof("MIGRATION [%v] ran successfully", value)
	}
}
