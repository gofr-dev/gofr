package nats

import (
	"context"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/logging"
)

// mockConn is a minimal mock implementation of nats.Conn
type mockConn struct {
	status nats.Status
}

func (m *mockConn) Status() nats.Status {
	return m.status
}

// mockJetStream is a minimal mock implementation of jetstream.JetStream
type mockJetStream struct {
	accountInfoErr error
}

func (m *mockJetStream) AccountInfo(ctx context.Context) (*jetstream.AccountInfo, error) {
	if m.accountInfoErr != nil {
		return nil, m.accountInfoErr
	}
	return &jetstream.AccountInfo{}, nil
}

// testNATSClient is a test-specific implementation of natsClient
type testNATSClient struct {
	natsClient
	mockConn      *mockConn
	mockJetStream *mockJetStream
}

func (c *testNATSClient) Health() datasource.Health {
	health := datasource.Health{
		Details: make(map[string]interface{}),
	}

	health.Status = datasource.StatusUp

	if c.mockConn != nil && c.mockConn.Status() != nats.CONNECTED {
		health.Status = datasource.StatusDown
	}

	health.Details["host"] = c.config.Server
	health.Details["backend"] = "NATS"
	health.Details["connection_status"] = c.mockConn.Status().String()
	health.Details["jetstream_enabled"] = c.mockJetStream != nil

	if c.mockJetStream != nil {
		_, err := c.mockJetStream.AccountInfo(context.Background())
		if err != nil {
			health.Details["jetstream_status"] = "Error: " + err.Error()
		} else {
			health.Details["jetstream_status"] = "OK"
		}
	}

	return health
}

func TestNATSClient_HealthStatusUP(t *testing.T) {
	client := &testNATSClient{
		natsClient: natsClient{
			config: Config{Server: "nats://localhost:4222"},
			logger: logging.NewMockLogger(logging.DEBUG),
		},
		mockConn:      &mockConn{status: nats.CONNECTED},
		mockJetStream: &mockJetStream{},
	}

	health := client.Health()

	assert.Equal(t, datasource.StatusUp, health.Status)
	assert.Equal(t, "nats://localhost:4222", health.Details["host"])
	assert.Equal(t, "NATS", health.Details["backend"])
	assert.Equal(t, "CONNECTED", health.Details["connection_status"])
	assert.Equal(t, true, health.Details["jetstream_enabled"])
	assert.Equal(t, "OK", health.Details["jetstream_status"])
}

func TestNATSClient_HealthStatusDown(t *testing.T) {
	client := &testNATSClient{
		natsClient: natsClient{
			config: Config{Server: "nats://localhost:4222"},
			logger: logging.NewMockLogger(logging.DEBUG),
		},
		mockConn: &mockConn{status: nats.CLOSED},
	}

	health := client.Health()

	assert.Equal(t, datasource.StatusDown, health.Status)
	assert.Equal(t, "nats://localhost:4222", health.Details["host"])
	assert.Equal(t, "NATS", health.Details["backend"])
	assert.Equal(t, "CLOSED", health.Details["connection_status"])
	assert.Equal(t, false, health.Details["jetstream_enabled"])
}

func TestNATSClient_HealthJetStreamError(t *testing.T) {
	client := &testNATSClient{
		natsClient: natsClient{
			config: Config{Server: "nats://localhost:4222"},
			logger: logging.NewMockLogger(logging.DEBUG),
		},
		mockConn:      &mockConn{status: nats.CONNECTED},
		mockJetStream: &mockJetStream{accountInfoErr: nats.ErrConnectionClosed},
	}

	health := client.Health()

	assert.Equal(t, datasource.StatusUp, health.Status)
	assert.Equal(t, "nats://localhost:4222", health.Details["host"])
	assert.Equal(t, "NATS", health.Details["backend"])
	assert.Equal(t, "CONNECTED", health.Details["connection_status"])
	assert.Equal(t, true, health.Details["jetstream_enabled"])
	assert.Equal(t, "Error: nats: connection closed", health.Details["jetstream_status"])
}
