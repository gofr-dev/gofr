package container

import "context"

const (
	StatusUp       = "UP"
	StatusDown     = "DOWN"
	StatusDegraded = "DEGRADED"
)

type HealthChecker interface {
	HealthCheck() interface{}
}

type Readiness interface {
	Ready(ctx context.Context) interface{}
}

type Health struct {
	Status      string                 `json:"status,omitempty"`
	Services    []ServiceHealth        `json:"services,omitempty"`
	Datasources map[string]interface{} `json:"datasource,omitempty"`
}

type ServiceHealth struct {
	Name   string `json:"name,omitempty"`
	Status string `json:"status,omitempty"`
}
