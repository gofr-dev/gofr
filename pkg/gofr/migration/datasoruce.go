package migration

import (
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/datasource/redis"
)

type Datasource struct {
	Logger

	migrationVersion int64

	DB    sqlDB
	Redis *redis.Redis
}

func newDatasource(c *container.Container) Datasource {
	d := Datasource{Logger: c.Logger}

	if c.DB.DB != nil {
		d.DB = newMysql(c)
	}

	d.Redis.Client = c.Redis.Client

	return d
}
