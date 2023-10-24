package errors

// HealthCheckFailed is used when the health check for a dependency is failing
type HealthCheckFailed struct {
	Dependency string `json:"dependency"`
	Reason     string `json:"reason"`
	Err        error  `json:"err"`
}

// Error returns a formatted message that includes information regarding failed health check
func (h HealthCheckFailed) Error() string {
	msg := "Health check failed for " + h.Dependency
	if h.Reason != "" {
		msg += " Reason: " + h.Reason
	}

	if h.Err != nil {
		msg += " Error: " + h.Err.Error()
	}

	return msg
}
