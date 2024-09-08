package nats

import (
	"context"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

//go:generate go run go.uber.org/mock/mockgen -source=interfaces.go -destination=mock_interfaces.go -package=nats

// Consumer represents a NATS JetStream consumer
type Consumer interface {
	FetchMessage(ctx context.Context) (*nats.Msg, error)
	Consume(ctx context.Context, handler func(*nats.Msg)) error
	GetInfo() (*nats.ConsumerInfo, error)
}

// Publisher represents a NATS JetStream publisher
type Publisher interface {
	Publish(ctx context.Context, subject string, data []byte) (*nats.PubAck, error)
	PublishAsync(subject string, data []byte) (nats.PubAckFuture, error)
}

// StreamManager represents NATS JetStream management operations
type StreamManager interface {
	AddStream(config *nats.StreamConfig) (*nats.StreamInfo, error)
	UpdateStream(config *nats.StreamConfig) (*nats.StreamInfo, error)
	DeleteStream(name string) error
	StreamInfo(name string) (*nats.StreamInfo, error)
}

// JetStreamContext represents the main NATS JetStream context
type JetStreamContext interface {
	Consumer
	Publisher
	StreamManager
	AccountInfo(ctx context.Context) (*jetstream.AccountInfo, error)
}

// Connection represents the NATS connection
type Connection interface {
	Status() nats.Status
	JetStream(opts ...nats.JSOpt) (JetStreamContext, error)
	Close()
}

// Client represents the main NATS JetStream client
type Client interface {
	Publish(ctx context.Context, stream string, message []byte) error
	Subscribe(ctx context.Context, stream string) (*pubsub.Message, error)
	Close() error
}
