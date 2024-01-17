package redis

import (
	"context"
	"encoding/json"
	"gofr.dev/pkg/gofr/datasource"
	"time"
)

func (r *Redis) HealthCheck() datasource.Health {
	h := datasource.Health{
		Details: make(map[string]interface{}),
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	if cancel != nil {
		r.logger.Error("")
	}

	health := r.InfoMap(ctx).Val()
	if len(health) == 0 {
		h.Status = "DOWN"

		return h
	}

	bytes, err := json.Marshal(health)
	if err != nil {
		r.logger.Error("Failed to Marshal REDIS Stats :%v", err)
	}

	stat := struct {
		Stat map[string]interface{} `json:"stats"`
	}{}

	err = json.Unmarshal(bytes, &stat)
	if err != nil {
		r.logger.Error("Failed to Unmarshal REDIS Stats :%v", err)
	}

	h.Status = "UP"
	h.Details["stats"] = stat

	return h
}
