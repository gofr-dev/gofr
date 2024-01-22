package datasource

const (
	StatusUp   = "UP"
	StatusDown = "DOWN"
)

type Health struct {
	Status  string                 `json:"status,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}
