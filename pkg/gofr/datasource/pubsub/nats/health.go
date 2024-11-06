package nats

import (
	"context"

	"gofr.dev/pkg/gofr/datasource"
)

const (
	natsBackend            = "Client"
	jetStreamStatusOK      = "OK"
	jetStreamStatusError   = "Error"
	jetStreamConnected     = "CONNECTED"
	jetStreamDisconnecting = "DISCONNECTED"
)

// Health checks the health of the NATS connection.
func (c *Client) Health() datasource.Health {
	if c.connManager == nil {
		return datasource.Health{
			Status: datasource.StatusDown,
		}
	}

	health := c.connManager.Health()
	health.Details["backend"] = natsBackend

	js, err := c.connManager.jetStream()
	if err != nil {
		health.Details["jetstream_enabled"] = false
		health.Details["jetstream_status"] = jetStreamStatusError + ": " + err.Error()

		return health
	}

	// Call AccountInfo() to get jStream status
	jetStreamStatus, err := GetJetStreamStatus(context.Background(), js)
	if err != nil {
		jetStreamStatus = jetStreamStatusError + ": " + err.Error()
	}

	health.Details["jetstream_enabled"] = true
	health.Details["jetstream_status"] = jetStreamStatus

	return health
}
