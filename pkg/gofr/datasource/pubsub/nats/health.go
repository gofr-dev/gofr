package nats

import (
	"context"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"gofr.dev/pkg/gofr/datasource"
)

const (
	natsBackend           = "NATS"
	jetstreamStatusOK     = "OK"
	jetstreamStatusError  = "Error"
	jetstreamConnected    = "CONNECTED"
	jetstreamConnecting   = "CONNECTING"
	jetstreamDisconnected = "DISCONNECTED"
)

func (n *NATSClient) Health() datasource.Health {
	health := datasource.Health{
		Status:  datasource.StatusUp,
		Details: make(map[string]interface{}),
	}

	connectionStatus := n.conn.Status()

	switch connectionStatus {
	case nats.CONNECTING:
		health.Status = datasource.StatusUp
		health.Details["connection_status"] = jetstreamConnecting
		n.logger.Logf("NATS health check: Connecting")
	case nats.CONNECTED:
		health.Details["connection_status"] = jetstreamConnected
		n.logger.Logf("NATS health check: Connected")
	case nats.CLOSED, nats.DISCONNECTED, nats.RECONNECTING, nats.DRAINING_PUBS, nats.DRAINING_SUBS:
		health.Status = datasource.StatusDown
		health.Details["connection_status"] = jetstreamDisconnected
		n.logger.Errorf("NATS health check: Disconnected")
	default:
		health.Status = datasource.StatusDown
		health.Details["connection_status"] = connectionStatus.String()
		n.logger.Errorf("NATS health check: Unknown status %v", connectionStatus)
	}

	health.Details["host"] = n.config.Server
	health.Details["backend"] = natsBackend
	health.Details["jetstream_enabled"] = n.js != nil

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if n.js != nil && connectionStatus == nats.CONNECTED {
		status := getJetstreamStatus(ctx, n.js)
		health.Details["jetstream_status"] = status
		if status != jetstreamStatusOK {
			n.logger.Errorf("NATS health check: JetStream error: %v", status)
		} else {
			n.logger.Logf("NATS health check: JetStream enabled")
		}
	} else if n.js == nil {
		n.logger.Logf("NATS health check: JetStream not enabled")
	}

	return health
}

func getJetstreamStatus(ctx context.Context, js jetstream.JetStream) string {
	_, err := js.AccountInfo(ctx)
	if err != nil {
		return jetstreamStatusError + ": " + err.Error()
	}

	return jetstreamStatusOK
}
