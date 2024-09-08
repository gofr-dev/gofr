package nats

import (
	"context"

	"github.com/nats-io/nats.go"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

// Consumer represents a NATS JetStream consumer
type Consumer interface {
	// FetchMessage fetches a single message from the stream
	FetchMessage(ctx context.Context) (*nats.Msg, error)
	// Consume starts consuming messages from the stream
	Consume(ctx context.Context, handler func(*nats.Msg)) error
	// GetInfo returns information about the consumer
	GetInfo() (*nats.ConsumerInfo, error)
}

// Publisher represents a NATS JetStream publisher
type Publisher interface {
	// Publish publishes a message to the stream
	Publish(ctx context.Context, subject string, data []byte) (*nats.PubAck, error)
	// PublishAsync publishes a message to the stream asynchronously
	PublishAsync(subject string, data []byte) (nats.PubAckFuture, error)
}

// StreamManager represents NATS JetStream management operations
type StreamManager interface {
	// AddStream adds a new stream
	AddStream(config *nats.StreamConfig) (*nats.StreamInfo, error)
	// UpdateStream updates an existing stream
	UpdateStream(config *nats.StreamConfig) (*nats.StreamInfo, error)
	// DeleteStream deletes a stream
	DeleteStream(name string) error
	// StreamInfo gets information about a stream
	StreamInfo(name string) (*nats.StreamInfo, error)
}

// JetStreamContext represents the main NATS JetStream context
type JetStreamContext interface {
	Consumer
	Publisher
	StreamManager
}

// Connection represents the NATS connection
type Connection interface {
	// JetStream returns a JetStreamContext
	JetStream(opts ...nats.JSOpt) (JetStreamContext, error)
	// Close closes the connection
	Close()
}

// Client represents the main NATS JetStream client
type Client interface {
	// Publish publishes a message to a stream
	Publish(ctx context.Context, stream string, message []byte) error
	// Subscribe subscribes to a stream and returns a message
	Subscribe(ctx context.Context, stream string) (*pubsub.Message, error)
	// Close closes the client connection
	Close() error
}
