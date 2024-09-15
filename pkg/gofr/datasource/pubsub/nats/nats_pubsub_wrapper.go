package nats

import (
	"context"

	"github.com/nats-io/nats.go"
	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/health"
)

// natsPubSubWrapper adapts NATSClient to pubsub.Client.
type natsPubSubWrapper struct {
	client *NATSClient
}

// Publish publishes a message to a topic.
func (w *natsPubSubWrapper) Publish(ctx context.Context, topic string, message []byte) error {
	return w.client.Publish(ctx, topic, message)
}

// Subscribe subscribes to a topic.
func (w *natsPubSubWrapper) Subscribe(ctx context.Context, topic string) (*pubsub.Message, error) {
	return w.client.Subscribe(ctx, topic)
}

// CreateTopic creates a new topic (stream) in NATS JetStream.
func (w *natsPubSubWrapper) CreateTopic(ctx context.Context, name string) error {
	return w.client.CreateTopic(ctx, name)
}

// DeleteTopic deletes a topic (stream) in NATS JetStream.
func (w *natsPubSubWrapper) DeleteTopic(ctx context.Context, name string) error {
	return w.client.DeleteTopic(ctx, name)
}

// Close closes the NATS client.
func (w *natsPubSubWrapper) Close() error {
	return w.client.Close()
}

// Health returns the health status of the NATS client.
func (w *natsPubSubWrapper) Health() health.Health {
	// Implement health check
	status := health.StatusUp
	if w.client.Conn.Status() != nats.CONNECTED {
		status = health.StatusDown
	}

	return health.Health{
		Status: status,
		Details: map[string]interface{}{
			"server": w.client.Config.Server,
		},
	}
}