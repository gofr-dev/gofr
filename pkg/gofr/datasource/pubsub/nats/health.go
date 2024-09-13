package nats

import (
	"github.com/nats-io/nats.go"
	"gofr.dev/pkg/gofr/datasource"
)

const (
	natsBackend               = "NATS"
	jetstreamStatusOK         = "OK"
	jetstreamStatusError      = "Error"
	jetstreamConnectionError  = "Error: nats: connection closed"
	jetstreamConnectionClosed = "CLOSED"
	jetstreamConnectionOK     = "Connection OK"
	jetstreamConnected        = "CONNECTED"
	jetstreamDisconnected     = "DISCONNECTED"
	jetstreamError            = "Error: jetstream error"
)

func (n *NatsClient) Health() datasource.Health {
	health := datasource.Health{
		Details: make(map[string]interface{}),
	}

	health.Status = datasource.StatusUp

	connectionStatus := n.conn.Status()
	health.Details["connection_status"] = connectionStatus.String()

	if connectionStatus != nats.CONNECTED {
		health.Status = datasource.StatusDown
	}

	health.Details["host"] = n.config.Server
	health.Details["backend"] = natsBackend
	health.Details["jetstream_enabled"] = n.js != nil

	// Only check JetStream if the connection is CONNECTED
	if connectionStatus == nats.CONNECTED && n.js != nil {
		_, err := n.js.AccountInfo()
		if err != nil {
			health.Details["jetstream_status"] = jetstreamStatusError + err.Error()
		} else {
			health.Details["jetstream_status"] = jetstreamStatusOK
		}
	}

	return health
}
