package nats

import (
	"context"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"gofr.dev/pkg/gofr/health"
)

type ConnInterface interface {
	Status() nats.Status
	Close()
	NatsConn() *nats.Conn
}

// Client represents the main NATS JetStream client.
type Client interface {
	Publish(ctx context.Context, subject string, message []byte) error
	Subscribe(ctx context.Context, subject string, handler MessageHandler) error
	Close(ctx context.Context) error
	DeleteStream(ctx context.Context, name string) error
	CreateStream(ctx context.Context, cfg StreamConfig) error
	CreateOrUpdateStream(ctx context.Context, cfg jetstream.StreamConfig) (jetstream.Stream, error)
	Health() health.Health
}

// MessageHandler represents the function signature for handling messages.
// type MessageHandler func(*gofr.Context, jetstream.Msg) error
type MessageHandler func(context.Context, jetstream.Msg) error
