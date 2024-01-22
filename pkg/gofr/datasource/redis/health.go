package redis

import (
	"context"
	"time"

	"gofr.dev/pkg/gofr/datasource"
)

func (r *Redis) HealthCheck() datasource.Health {
	h := datasource.Health{
		Details: make(map[string]interface{}),
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	info, err := r.InfoMap(ctx, "Stats").Result()
	if err != nil {
		h.Status = datasource.StatusDown
		h.Details["error"] = err.Error()

		return h
	}

	h.Status = datasource.StatusUp
	h.Details["stats"] = info

	return h
}
