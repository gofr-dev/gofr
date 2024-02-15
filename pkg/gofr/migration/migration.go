package migration

import (
	"fmt"
	"gofr.dev/pkg/gofr/datasource/sql"
	"time"

	"github.com/gogo/protobuf/sortkeys"

	"gofr.dev/pkg/gofr/container"
)

type MigrateFunc func(d Datasource) error

type Migrate struct {
	UP MigrateFunc
}

func Run(migrationsMap map[int64]Migrate, c *container.Container) {
	invalidKeys := ""

	// Sort migrations by version
	keys := make([]int64, 0, len(migrationsMap))

	for k, v := range migrationsMap {
		if v.UP == nil {
			invalidKeys += fmt.Sprintf("%v,", k)

			continue
		}

		keys = append(keys, k)
	}

	if len(invalidKeys) > 0 {
		c.Logger.Errorf("Run Failed! UP not defined for the following keys: %v", invalidKeys[0:len(invalidKeys)-1])

		return
	}

	sortkeys.Int64s(keys)

	var lastMigration int64

	if c.DB != nil {
		err := ensureSQLMigrationTableExists(c)
		if err != nil {
			c.Logger.Errorf("unable to verify sql migration table due to : %v", err)
			return
		}

		lastMigration = getLastMigration(c)
	}

	for _, v := range keys {
		if v <= lastMigration {
			continue
		}

		start := time.Now()

		tx, err := c.DB.Begin()
		if err != nil {
			rollbackAndLog(c, tx)
			return
		}

		p := c.Redis.TxPipeline()

		sql := newMysql(v, tx)

		datasource := newDatasource(c.Logger, sql, newRedis(v, p))

		err = migrationsMap[v].UP(datasource)
		if err != nil {
			rollbackAndLog(c, tx)
			return
		}

		sqlPostRun(c, tx, v, start, sql.used)
	}
}

func sqlPostRun(c *container.Container, tx *sql.Tx, currentMigration int64, start time.Time, used bool) {
	if !used {
		rollbackAndLog(c, tx)
		return
	}

	err := insertMigrationRecord(tx, currentMigration, start)
	if err != nil {
		rollbackAndLog(c, tx)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		c.Logger.Error("unable to commit transaction: %v", err)
	}
}
