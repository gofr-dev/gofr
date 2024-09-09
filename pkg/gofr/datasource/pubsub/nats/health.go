package nats

import (
	"github.com/nats-io/nats.go"
	"gofr.dev/pkg/gofr/datasource"
)

// Health returns the health of the NATS connection.
func (n *natsClient) Health() datasource.Health {
	health := datasource.Health{
		Details: make(map[string]interface{}),
	}

	health.Status = datasource.StatusUp

	// Check connection status
	if n.conn.Status() != nats.CONNECTED {
		health.Status = datasource.StatusDown
	}

	health.Details["host"] = n.config.Server
	health.Details["backend"] = "NATS"
	health.Details["connection_status"] = n.conn.Status().String()
	health.Details["jetstream_enabled"] = n.js != nil

	// Simple JetStream check
	if n.js != nil {
		_, err := n.js.AccountInfo()
		if err != nil {
			health.Details["jetstream_status"] = "Error: " + err.Error()
		} else {
			health.Details["jetstream_status"] = "OK"
		}
	}

	return health
}
