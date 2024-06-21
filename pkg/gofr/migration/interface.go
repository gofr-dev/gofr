package migration

import (
	"context"
	"database/sql"
	"time"

	goRedis "github.com/redis/go-redis/v9"
)

type Decorator interface {
	Apply(m migrator) migrator
}

type Redis interface {
	Get(ctx context.Context, key string) *goRedis.StringCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *goRedis.StatusCmd
	Del(ctx context.Context, keys ...string) *goRedis.IntCmd
	Rename(ctx context.Context, key, newKey string) *goRedis.StatusCmd
}

type SQL interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	Exec(query string, args ...interface{}) (sql.Result, error)
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

type PubSub interface {
	CreateTopic(context context.Context, name string) error
	DeleteTopic(context context.Context, name string) error
}
