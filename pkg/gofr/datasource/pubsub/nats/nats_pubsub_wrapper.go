package nats

import (
	"context"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/health"
)

// NatsPubSubWrapper adapts NATSClient to pubsub.Client.
type NatsPubSubWrapper struct {
	Client *NATSClient
}

// Publish publishes a message to a topic.
func (w *NatsPubSubWrapper) Publish(ctx context.Context, topic string, message []byte) error {
	return w.Client.Publish(ctx, topic, message)
}

// Subscribe subscribes to a topic.
func (w *NatsPubSubWrapper) Subscribe(ctx context.Context, topic string) (*pubsub.Message, error) {
	msgChan := make(chan *pubsub.Message)

	err := w.Client.Subscribe(ctx, topic, func(ctx context.Context, msg jetstream.Msg) error {
		select {
		case msgChan <- &pubsub.Message{
			Topic:     topic,
			Value:     msg.Data(),
			Committer: &natsCommitter{msg: msg},
		}:
		case <-ctx.Done():
			return ctx.Err()
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	select {
	case msg := <-msgChan:
		return msg, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// CreateTopic creates a new topic (stream) in NATS JetStream.
func (w *NatsPubSubWrapper) CreateTopic(ctx context.Context, name string) error {
	return w.Client.CreateTopic(ctx, name)
}

// DeleteTopic deletes a topic (stream) in NATS JetStream.
func (w *NatsPubSubWrapper) DeleteTopic(ctx context.Context, name string) error {
	return w.Client.DeleteTopic(ctx, name)
}

// Close closes the NATS client.
func (w *NatsPubSubWrapper) Close() error {
	return w.Client.Close()
}

// Health returns the health status of the NATS client.
func (w *NatsPubSubWrapper) Health() health.Health {
	status := health.StatusUp
	if w.Client.Conn.Status() != nats.CONNECTED {
		status = health.StatusDown
	}

	return health.Health{
		Status: status,
		Details: map[string]interface{}{
			"server": w.Client.Config.Server,
		},
	}
}
