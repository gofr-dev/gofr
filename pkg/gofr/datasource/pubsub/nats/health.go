package nats

import (
	"context"

	"github.com/nats-io/nats.go/jetstream"
	"gofr.dev/pkg/gofr/datasource"
)

const (
	natsBackend            = "Client"
	jetStreamStatusOK      = "OK"
	jetStreamStatusError   = "Error"
	jetStreamConnected     = "CONNECTED"
	jetStreamConnecting    = "CONNECTING"
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

	js := c.connManager.JetStream()
	if js != nil {
		health.Details["jetstream_enabled"] = true
		health.Details["jetstream_status"] = getJetStreamStatus(context.Background(), js)
	} else {
		health.Details["jetstream_enabled"] = false
	}

	return health
}

func getJetStreamStatus(ctx context.Context, js jetstream.JetStream) string {
	_, err := js.AccountInfo(ctx)
	if err != nil {
		return jetStreamStatusError + ": " + err.Error()
	}

	return jetStreamStatusOK
}
