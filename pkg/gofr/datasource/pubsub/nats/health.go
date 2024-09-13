package nats

import (
	"github.com/nats-io/nats.go"
	"gofr.dev/pkg/gofr/datasource"
)

const (
	natsBackend           = "NATS"
	jetstreamStatusOK     = "OK"
	jetstreamStatusError  = "Error"
	jetstreamConnected    = "CONNECTED"
	jetstreamDisconnected = "DISCONNECTED"
)

func (n *NATSClient) Health() datasource.Health {
	health := datasource.Health{
		Status:  datasource.StatusUp,
		Details: make(map[string]interface{}),
	}

	connectionStatus := n.conn.Status()

	switch connectionStatus {
	case nats.CONNECTED:
		health.Details["connection_status"] = jetstreamConnected
	case nats.CLOSED, nats.DISCONNECTED, nats.RECONNECTING, nats.DRAINING_PUBS, nats.DRAINING_SUBS:
		health.Status = datasource.StatusDown
		health.Details["connection_status"] = jetstreamDisconnected
	default:
		health.Status = datasource.StatusDown
		health.Details["connection_status"] = connectionStatus.String()
	}

	health.Details["host"] = n.config.Server
	health.Details["backend"] = natsBackend
	health.Details["jetstream_enabled"] = n.js != nil

	if n.js != nil && connectionStatus == nats.CONNECTED {
		health.Details["jetstream_status"] = getJetstreamStatus(n.js)
	}

	return health
}

func getJetstreamStatus(js JetStreamContext) string {
	_, err := js.AccountInfo()
	if err != nil {
		return jetstreamStatusError + ": " + err.Error()
	}
	return jetstreamStatusOK
}
