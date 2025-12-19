package redis

import (
	"context"
	"fmt"
	"gofr.dev/pkg/gofr/datasource"
	"strings"
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
