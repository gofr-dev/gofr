package mqtt

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

type mockLogger struct{}

func (m mockLogger) Infof(format string, args ...any)  {}
func (m mockLogger) Debug(args ...any)                 {}
func (m mockLogger) Debugf(format string, args ...any) {}
func (m mockLogger) Warnf(format string, args ...any)  {}
func (m mockLogger) Errorf(format string, args ...any) {}

type mockMetrics struct{}

func (m mockMetrics) IncrementCounter(ctx context.Context, name string, labels ...string) {}

func TestMQTT_Health(t *testing.T) {
	logger := mockLogger{}
	metrics := mockMetrics{}

	cfg := &Config{
		Protocol:  "tcp",
		Hostname:  "127.0.0.1",
		Port:      23456, // Unused port to avoid connection
		KeepAlive: 5 * time.Second,
	}

	// This should fail to connect but return a valid *MQTT
	m := New(cfg, logger, metrics)
	assert.NotNil(t, m)

	health := m.Health()
	assert.Equal(t, "DOWN", health.Status)
}

func TestMQTT_Query_InvalidTopic(t *testing.T) {
	logger := mockLogger{}
	metrics := mockMetrics{}

	cfg := &Config{
		Protocol:  "tcp",
		Hostname:  "127.0.0.1",
		Port:      23456,
		KeepAlive: 5 * time.Second,
	}

	m := New(cfg, logger, metrics)

	// Since we are not connected, AwaitConnection will fail
	_, err := m.Query(context.Background(), "test/topic")
	assert.ErrorIs(t, err, errClientNotConnected)
}

func TestMQTT_Publish_NotConnected(t *testing.T) {
	logger := mockLogger{}
	metrics := mockMetrics{}

	cfg := &Config{
		Protocol:  "tcp",
		Hostname:  "127.0.0.1",
		Port:      23456,
		KeepAlive: 5 * time.Second,
	}

	m := New(cfg, logger, metrics)

	// Force connection wait timeout to trigger context cancellation
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	defer cancel()

	err := m.Publish(ctx, "test", []byte("message"))
	assert.Error(t, err) // autopaho returns error if context cancelled during publish wait
}

func TestMQTT_Subscribe_NotConnected(t *testing.T) {
	logger := mockLogger{}
	metrics := mockMetrics{}

	cfg := &Config{
		Protocol:  "tcp",
		Hostname:  "127.0.0.1",
		Port:      23456,
		KeepAlive: 5 * time.Second,
	}

	m := New(cfg, logger, metrics)

	ctx := context.Background()

	_, err := m.Subscribe(ctx, "test")
	assert.ErrorIs(t, err, errClientNotConnected)
}

func TestMQTT_Close(t *testing.T) {
	logger := mockLogger{}
	metrics := mockMetrics{}

	cfg := &Config{
		Protocol:     "tcp",
		Hostname:     "127.0.0.1",
		Port:         23456,
		KeepAlive:    5 * time.Second,
		CloseTimeout: 1 * time.Second,
	}

	m := New(cfg, logger, metrics)

	err := m.Close()
	// When connection is not established, disconnect might return a context deadline error or canceled.
	// Since we only want to ensure it doesn't panic and returns, we'll just check it doesn't panic.
	// `autopaho` might return an error when Disconnect is called while it's dialing.
	_ = err
}

func TestMQTT_GetDefaultClient(t *testing.T) {
	logger := mockLogger{}
	metrics := mockMetrics{}

	cfg := &Config{
		Protocol:  "tcp",
		Hostname:  "", // This should trigger getDefaultClient
		Port:      1883,
		KeepAlive: 5 * time.Second,
	}

	m := New(cfg, logger, metrics)
	assert.NotNil(t, m)
	assert.Equal(t, publicBroker, m.config.Hostname)
}

func Test_parseQueryArgs(t *testing.T) {
	timeout, limit := parseQueryArgs(10*time.Second, 5)
	assert.Equal(t, 10*time.Second, timeout)
	assert.Equal(t, 5, limit)

	timeout, limit = parseQueryArgs()
	assert.Equal(t, defaultQueryCollectTimeout, timeout)
	assert.Equal(t, defaultQueryMessageLimit, limit)

	timeout, limit = parseQueryArgs("invalid type", "invalid type")
	assert.Equal(t, defaultQueryCollectTimeout, timeout)
	assert.Equal(t, defaultQueryMessageLimit, limit)
}

func Test_topicMatch(t *testing.T) {
	assert.True(t, topicMatch("test/topic", "test/topic"))
	assert.False(t, topicMatch("test/topic", "test/other"))
}

func TestMQTT_CreateTopic(t *testing.T) {
	logger := mockLogger{}
	metrics := mockMetrics{}

	cfg := &Config{
		Protocol:  "tcp",
		Hostname:  "127.0.0.1",
		Port:      23456,
		KeepAlive: 5 * time.Second,
	}

	m := New(cfg, logger, metrics)

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	defer cancel()

	err := m.CreateTopic(ctx, "new-topic")
	assert.Error(t, err)
}

func TestMQTT_DeleteTopic(t *testing.T) {
	logger := mockLogger{}
	metrics := mockMetrics{}

	m := New(&Config{Hostname: "127.0.0.1"}, logger, metrics)

	err := m.DeleteTopic(context.Background(), "topic")
	assert.NoError(t, err) // DeleteTopic is a no-op
}

func TestMQTT_SubscribeWithFunction(t *testing.T) {
	logger := mockLogger{}
	metrics := mockMetrics{}

	m := New(&Config{Hostname: "127.0.0.1"}, logger, metrics)

	err := m.SubscribeWithFunction("test/topic", func(msg *pubsub.Message) error {
		return nil
	})

	assert.Error(t, err) // should fail because it tries to subscribe
}
