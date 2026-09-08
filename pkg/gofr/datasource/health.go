package datasource

const (
	StatusUp   = "UP"
	StatusDown = "DOWN"
	// StatusDegraded is the aggregate an application reports when it is itself serving but at
	// least one of its dependencies is down. It is never the status of an individual datasource.
	StatusDegraded = "DEGRADED"
)

type Health struct {
	Status  string         `json:"status,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}
