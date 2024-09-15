package health

// Status represents the status of a service.
type Status string

const (
	StatusUp   Status = "up"
	StatusDown Status = "down"
)

// Health represents the health of a service.
type Health struct {
	Status  Status
	Details map[string]interface{}
}
