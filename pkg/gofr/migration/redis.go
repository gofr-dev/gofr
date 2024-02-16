package migration

import (
	"context"
	"encoding/json"
	"time"

	goRedis "github.com/redis/go-redis/v9"

	"gofr.dev/pkg/gofr/container"
)

type migration struct {
	Version   int64
	StartTime time.Time
	Duration  int64
}

type redisCache struct {
	redis

	currentMigration []migration
	used             bool
}

func newRedis(version int64, r redis) redisCache {
	return redisCache{
		redis: r,
		currentMigration: []migration{{
			Version:   version,
			StartTime: time.Now(),
		}},
	}
}

type redis interface {
	Get(ctx context.Context, key string) *goRedis.StringCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *goRedis.StatusCmd
	Del(ctx context.Context, keys ...string) *goRedis.IntCmd
	Rename(ctx context.Context, key, newKey string) *goRedis.StatusCmd
}

func (r redisCache) Get(ctx context.Context, key string) *goRedis.StringCmd {
	r.used = true

	return r.redis.Get(ctx, key)
}

func (r redisCache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *goRedis.StatusCmd {
	r.used = true

	return r.redis.Set(ctx, key, value, expiration)
}

func (r redisCache) Del(ctx context.Context, keys ...string) *goRedis.IntCmd {
	r.used = true

	return r.redis.Del(ctx, keys...)
}

func (r redisCache) Rename(ctx context.Context, key, newKey string) *goRedis.StatusCmd {
	r.used = true

	return r.redis.Rename(ctx, key, newKey)
}

func (r redisCache) redisPostRun(c *container.Container, p goRedis.Pipeliner, currentMigration int64, start time.Time) {
	migBytes, _ := json.Marshal(r.currentMigration)

	p.HSet(context.Background(), "gofr_migrations", "UP", string(migBytes))
}
