package datasource

const (
	StatusUp   = "UP"
	StatusDown = "DOWN"
)

type Health struct {
	Status  string         `json:"status,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}
