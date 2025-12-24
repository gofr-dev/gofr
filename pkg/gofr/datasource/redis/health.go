// Package redis provides a client for interacting with Redis key-value stores.This package allows creating and
// managing Redis clients, executing Redis commands, and handling connections to Redis databases.
package redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"gofr.dev/pkg/gofr/datasource"
)

const (
	healthCheckTimeout = 1 * time.Second
)

// HealthCheck returns the health status of the Redis connection.
func (r *Redis) HealthCheck() datasource.Health {
	h := datasource.Health{
		Details: make(map[string]any),
	}

	h.Details["host"] = r.config.HostName + ":" + strconv.Itoa(r.config.Port)

	ctx, cancel := context.WithTimeout(context.Background(), healthCheckTimeout)
	defer cancel()

	if r.Client == nil {
		h.Status = datasource.StatusDown
		h.Details["error"] = "redis not connected"

		return h
	}

	info, err := r.Client.InfoMap(ctx, "Stats").Result()
	if err != nil {
		h.Status = datasource.StatusDown
		h.Details["error"] = err.Error()

		return h
	}

	h.Status = datasource.StatusUp
	h.Details["stats"] = info["Stats"]

	return h
}

// Health returns the health status of the Redis PubSub connection.
func (ps *PubSub) Health() datasource.Health {
	res := datasource.Health{
		Status: datasource.StatusDown,
		Details: map[string]any{
			"backend": "REDIS",
		},
	}

	addr := fmt.Sprintf("%s:%d", ps.config.HostName, ps.config.Port)
	res.Details["host"] = addr

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

	res.Status = datasource.StatusUp

	return res
}
