package oracle

import "context"

type Conn interface {
    Select(ctx context.Context, dest any, query string, args ...any) error
    Exec(ctx context.Context, query string, args ...any) error
    AsyncInsert(ctx context.Context, query string, wait bool, args ...any) error
    Ping(ctx context.Context) error
    Stats() any
}
