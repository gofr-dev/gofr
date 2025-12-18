package redis

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"gofr.dev/pkg/gofr/datasource"
)

var (
	// redisInternalLoggerOnce ensures we set the go-redis internal logger once.
	//
	// Not specifically because we have two Redis clients (db + PubSub), but because go-redis itself has a single
	// global internal logger. We create Redis clients in multiple places (regular Redis datasource, and Redis PubSub
	// via NewPubSub). Both paths call redis.SetLogger(...) to route go-redis internal messages through GoFr logging.
	// Since that logger is process-wide, we guard it with sync.Once so it’s set once, regardless of how many clients
	// (db + pubsub, or multiple containers/tests) get created.
	redisInternalLoggerOnce sync.Once //nolint:gochecknoglobals // This is a package-level singleton for logger setup
)

// redactRedisHostname redacts any credentials from a Redis hostname that may be in URI format.
// If the hostname is a URI like redis://user:pass@host:port, credentials are replaced with [REDACTED].
// Plain hostnames are returned unchanged.
func redactRedisHostname(hostname string) string {
	if idx := strings.Index(hostname, "://"); idx != -1 {
		scheme := hostname[:idx]
		rest := hostname[idx+3:]

		if atIdx := strings.Index(rest, "@"); atIdx != -1 {
			return scheme + "://[REDACTED]@" + rest[atIdx+1:]
		}
	}

	return hostname
}

// gofrRedisLogger wraps go-redis internal logs (connection pool/retry/protocol messages)
// and routes them through GoFr's logger, so they use the same formatting and can be asserted in tests.
type gofrRedisLogger struct {
	logger datasource.Logger
}

// Printf implements redis.Logger interface.
func (l *gofrRedisLogger) Printf(_ context.Context, format string, v ...any) {
	if l.logger == nil {
		return
	}

	msg := fmt.Sprintf(format, v...)
	// Log through Gofr logger as DEBUG level.
	// Connection pool retry attempts are logged here, while actual connection failures
	// are logged by GoFr at ERROR level in NewClient/retryConnect.
	l.logger.Debugf("%s", msg)
}
