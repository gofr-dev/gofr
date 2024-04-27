package container

import (
	"context"
	"database/sql"

	"github.com/redis/go-redis/v9"

	"gofr.dev/pkg/gofr/datasource"
	gofrSQL "gofr.dev/pkg/gofr/datasource/sql"
)

type DB interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	Exec(query string, args ...interface{}) (sql.Result, error)
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	Prepare(query string) (*sql.Stmt, error)
	Begin() (*gofrSQL.Tx, error)
	Select(ctx context.Context, data interface{}, query string, args ...interface{})
	HealthCheck() *datasource.Health
	Dialect() string
}

type Redis interface {
	redis.Cmdable
	redis.HashCmdable
	HealthCheck() datasource.Health
}
