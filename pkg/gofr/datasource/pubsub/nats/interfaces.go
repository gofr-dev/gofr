package nats

import (
	"context"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

//go:generate mockgen -destination=mock_client.go -package=nats -source=./interfaces.go Client,Subscription,ConnInterface,ConnectionManagerInterface,SubscriptionManagerInterface,StreamManagerInterface

// ConnInterface represents the main Client connection.
type ConnInterface interface {
	Status() nats.Status
	Close()
	NATSConn() *nats.Conn
	JetStream() (jetstream.JetStream, error)
}

// Connector represents the main Client connection.
type Connector interface {
	Connect(string, ...nats.Option) (ConnInterface, error)
}

// JetStreamCreator represents the main Client jStream Client.
type JetStreamCreator interface {
	New(conn ConnInterface) (jetstream.JetStream, error)
}

// JetStreamClient represents the main Client jStream Client.
type JetStreamClient interface {
	Publish(ctx context.Context, subject string, message []byte) error
	Subscribe(ctx context.Context, subject string, handler messageHandler) error
	Close(ctx context.Context) error
	DeleteStream(ctx context.Context, name string) error
	CreateStream(ctx context.Context, cfg StreamConfig) error
	CreateOrUpdateStream(ctx context.Context, cfg jetstream.StreamConfig) (jetstream.Stream, error)
	Health() datasource.Health
}

// ConnectionManagerInterface represents the main Client connection.
type ConnectionManagerInterface interface {
	Connect() error
	Close(ctx context.Context)
	Publish(ctx context.Context, subject string, message []byte, metrics Metrics) error
	Health() datasource.Health
	jetStream() (jetstream.JetStream, error)
	isConnected() bool
}

// SubscriptionManagerInterface represents the main Subscription Manager.
type SubscriptionManagerInterface interface {
	Subscribe(
		ctx context.Context,
		topic string,
		js jetstream.JetStream,
		cfg *Config,
		logger pubsub.Logger,
		metrics Metrics) (*pubsub.Message, error)
	Close()
}

// StreamManagerInterface represents the main Stream Manager.
type StreamManagerInterface interface {
	CreateStream(ctx context.Context, cfg *StreamConfig) error
	DeleteStream(ctx context.Context, name string) error
	CreateOrUpdateStream(ctx context.Context, cfg *jetstream.StreamConfig) (jetstream.Stream, error)
}
