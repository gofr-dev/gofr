package migration

import (
	"context"
	"database/sql"
	"time"

	goRedis "github.com/redis/go-redis/v9"

	"gofr.dev/pkg/gofr/container"
)

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

type Clickhouse interface {
	Exec(ctx context.Context, query string, args ...any) error
	Select(ctx context.Context, dest any, query string, args ...any) error
	AsyncInsert(ctx context.Context, query string, wait bool, args ...any) error

	HealthCheck(ctx context.Context) (any, error)
}

// keeping the migrator interface unexported as, right now it is not being implemented directly, by the externalDB drivers.
// keeping the implementations for externalDB at one place such that if any change in migration logic, we would change directly here.
type migrator interface {
	checkAndCreateMigrationTable(c *container.Container) error
	getLastMigration(c *container.Container) int64

	beginTransaction(c *container.Container) transactionData

	commitMigration(c *container.Container, data transactionData) error
	rollback(c *container.Container, data transactionData)
}
