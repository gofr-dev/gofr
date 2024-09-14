package nats

import (
	"context"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

// ConnInterface represents the methods of nats.Conn that we use
type ConnInterface interface {
	JetStream(...nats.JSOpt) (jetstream.JetStream, error)
	Status() nats.Status
	Drain() error
}

// Client represents the main NATS JetStream client.
type Client interface {
	Publish(ctx context.Context, stream string, message []byte) error
	Subscribe(ctx context.Context, stream string) (*pubsub.Message, error)
	Close() error
}

// Subscription represents a NATS subscription.
type Subscription interface {
	Fetch(batch int, opts ...nats.PullOpt) ([]*nats.Msg, error)
	Drain() error
	Unsubscribe() error
	NextMsg(timeout time.Duration) (*nats.Msg, error)
}
