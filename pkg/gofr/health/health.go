package health

// Status represents the status of a service.
type Status string

const (
	StatusUp   Status = "UP"
	StatusDown Status = "DOWN"
)

// Health represents the health of a service.
type Health struct {
	Status  Status
	Details map[string]interface{}
}
