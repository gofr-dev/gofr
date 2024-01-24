package container

import "context"

type HealthChecker interface {
	HealthCheck() interface{}
}

type Readiness interface {
	Ready(ctx context.Context) interface{}
}

type Health struct {
	Status     string                 `json:"status,omitempty"`
	Services   []ServiceHealth        `json:"services,omitempty"`
	Datasource map[string]interface{} `json:"datasource,omitempty"`
}

const (
	StatusUp       = "UP"
	StatusDown     = "DOWN"
	StatusDegraded = "DEGRADED"
)
