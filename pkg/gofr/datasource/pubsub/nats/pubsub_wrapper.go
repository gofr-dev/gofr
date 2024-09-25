package nats

import (
	"context"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

// PubSubWrapper adapts NATS to pubsub.Client.
type PubSubWrapper struct {
	Client *NATS
}

// Publish publishes a message to a topic.
func (w *PubSubWrapper) Publish(ctx context.Context, topic string, message []byte) error {
	return w.Client.Publish(ctx, topic, message)
}

// Subscribe subscribes to a topic.
func (w *PubSubWrapper) Subscribe(ctx context.Context, topic string) (*pubsub.Message, error) {
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
func (w *PubSubWrapper) CreateTopic(ctx context.Context, name string) error {
	return w.Client.CreateTopic(ctx, name)
}

// DeleteTopic deletes a topic (stream) in NATS JetStream.
func (w *PubSubWrapper) DeleteTopic(ctx context.Context, name string) error {
	return w.Client.DeleteTopic(ctx, name)
}

// Close closes the NATS client.
func (w *PubSubWrapper) Close() error {
	return w.Client.Close()
}

// Health returns the health status of the NATS client.
func (w *PubSubWrapper) Health() datasource.Health {
	status := datasource.StatusUp
	if w.Client.Conn.Status() != nats.CONNECTED {
		status = datasource.StatusDown
	}

	return datasource.Health{
		Status: status,
		Details: map[string]interface{}{
			"server": w.Client.Config.Server,
		},
	}
}
