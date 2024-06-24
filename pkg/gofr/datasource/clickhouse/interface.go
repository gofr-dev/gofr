package clickhouse

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type Conn interface {
	Select(ctx context.Context, dest any, query string, args ...any) error
	Exec(ctx context.Context, query string, args ...any) error
	AsyncInsert(ctx context.Context, query string, wait bool, args ...any) error
	Ping(context.Context) error
	Stats() driver.Stats
}
