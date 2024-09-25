package nats

import (
	"context"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"gofr.dev/pkg/gofr/datasource"
)

//go:generate mockgen -destination=mock_client.go -package=nats -source=./interfaces.go Client,Subscription,ConnInterface

// connInterface represents the main client connection.
type connInterface interface {
	Status() nats.Status
	Close()
	NatsConn() *nats.Conn
}

// natsConnector represents the main client connection.
type natsConnector interface {
	Connect(string, ...nats.Option) (connInterface, error)
}

// jetStreamCreator represents the main client JetStream client.
type jetStreamCreator interface {
	New(*nats.Conn) (jetstream.JetStream, error)
}

// jetStreamClient represents the main client JetStream client.
type jetStreamClient interface {
	Publish(ctx context.Context, subject string, message []byte) error
	Subscribe(ctx context.Context, subject string, handler messageHandler) error
	Close(ctx context.Context) error
	DeleteStream(ctx context.Context, name string) error
	CreateStream(ctx context.Context, cfg StreamConfig) error
	CreateOrUpdateStream(ctx context.Context, cfg jetstream.StreamConfig) (jetstream.Stream, error)
	Health() datasource.Health
}
