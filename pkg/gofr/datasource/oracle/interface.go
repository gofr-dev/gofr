package oracle

import "context"

type Connection interface {
	Select(ctx context.Context, dest any, query string, args ...any) error
	Exec(ctx context.Context, query string, args ...any) error
	Ping(ctx context.Context) error
}

type Txn interface {
	ExecContext(ctx context.Context, query string, args ...any) error
	SelectContext(ctx context.Context, dest any, query string, args ...any) error
	Commit() error
	Rollback() error
}
