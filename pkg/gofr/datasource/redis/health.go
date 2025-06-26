// Package redis provides a client for interacting with Redis key-value stores.This package allows creating and
// managing Redis clients, executing Redis commands, and handling connections to Redis databases.
package redis

import (
	"context"
	"strconv"
	"time"

	"gofr.dev/pkg/gofr/datasource"
)

func (r *Redis) HealthCheck() datasource.Health {
	h := datasource.Health{
		Details: make(map[string]any),
	}

	h.Details["host"] = r.config.HostName + ":" + strconv.Itoa(r.config.Port)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if r.Client == nil {
		h.Status = datasource.StatusDown
		h.Details["error"] = "redis not connected"

		return h
	}

	info, err := r.InfoMap(ctx, "Stats").Result()
	if err != nil {
		h.Status = datasource.StatusDown
		h.Details["error"] = err.Error()

		return h
	}

	h.Status = datasource.StatusUp
	h.Details["stats"] = info["Stats"]

	return h
}
