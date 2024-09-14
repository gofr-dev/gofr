package nats

import (
	"context"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"gofr.dev/pkg/gofr"
)

type ConnInterface interface {
	Status() nats.Status
	Close()
}

// Client represents the main NATS JetStream client.
type Client interface {
	Publish(ctx context.Context, subject string, message []byte) error
	Subscribe(ctx context.Context, subject string, handler MessageHandler) error
	Close(ctx context.Context) error
	DeleteStream(ctx context.Context, name string) error
	CreateStream(ctx context.Context, cfg StreamConfig) error
	CreateOrUpdateStream(ctx context.Context, cfg jetstream.StreamConfig) (jetstream.Stream, error)
}

// MessageHandler represents the function signature for handling messages.
type MessageHandler func(*gofr.Context, jetstream.Msg) error

// StreamConfig holds stream settings for NATS JetStream.
type StreamConfig struct {
	Stream        string
	Subject       string
	AckPolicy     nats.AckPolicy
	DeliverPolicy nats.DeliverPolicy
	MaxDeliver    int
}
