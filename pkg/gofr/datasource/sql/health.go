package sql

import (
	"context"
	"time"

	"gofr.dev/pkg/gofr/datasource"
)

func (d *DB) HealthCheck() datasource.Health {
	h := datasource.Health{
		Details: make(map[string]interface{}),
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := d.PingContext(ctx)
	if err != nil {
		h.Status = "DOWN"

		return h
	}

	h.Status = "UP"
	h.Details["stats"] = d.Stats()

	return h
}
