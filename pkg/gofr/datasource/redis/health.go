package redis

import (
	"context"
	"fmt"

	"gofr.dev/pkg/gofr/datasource"
)

// Health returns the health status of the Redis PubSub connection.
func (ps *PubSub) Health() datasource.Health {
	res := datasource.Health{
		Status: "DOWN",
		Details: map[string]any{
			"backend": "REDIS",
		},
	}

	addr := fmt.Sprintf("%s:%d", ps.config.HostName, ps.config.Port)
	res.Details["addr"] = addr

	mode := ps.config.PubSubMode
	if mode == "" {
		mode = modeStreams
	}

	res.Details["mode"] = mode

	ctx, cancel := context.WithTimeout(context.Background(), defaultRetryTimeout)
	defer cancel()

	if err := ps.client.Ping(ctx).Err(); err != nil {
		ps.logger.Errorf("PubSub health check failed: %v", err)
		return res
	}

	res.Status = "UP"

	return res
}
