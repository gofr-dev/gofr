package migration

import (
	"context"
	"time"

	goRedis "github.com/redis/go-redis/v9"
)

type Datasource struct {
	Logger

	DB    sqlDB
	Redis redis
}

func newDatasource(l Logger, db sqlDB, r redis) Datasource {
	return Datasource{
		Logger: l,
		DB:     db,
		Redis:  r,
	}
}

type redisCache struct {
	migrationVersion int64

	redis
}

func newRedis(version int64, r redis) redisCache {
	return redisCache{
		migrationVersion: version,
		redis:            r,
	}
}

type redis interface {
	Get(ctx context.Context, key string) *goRedis.StringCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *goRedis.StatusCmd
	Del(ctx context.Context, keys ...string) *goRedis.IntCmd
	Rename(ctx context.Context, key, newkey string) *goRedis.StatusCmd
}
