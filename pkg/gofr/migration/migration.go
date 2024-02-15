package migration

import (
	"fmt"

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

	for _, v := range keys {
		tx, err := c.DB.Begin()
		if err != nil {
			tx.Rollback()
		}

		p := c.Redis.TxPipeline()

		datasource := newDatasource(c.Logger, newMysql(v, tx), newRedis(v, p))

		err = migrationsMap[v].UP(datasource)
		if err != nil {
			return
		}
	}
}
