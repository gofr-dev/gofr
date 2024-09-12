package nats

import (
	"context"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

type MsgMetadata struct {
	MsgId      string
	Stream     string
	Subject    string
	Sequence   uint64
	Timestamp  time.Time
	Reply      string
	ReplyChain []string
}

// Msg represents a NATS message
type Msg interface {
	Ack() error
	Data() []byte
	Subject() string
	Reply() string
	Metadata() (*jetstream.MsgMetadata, error)
	Headers() nats.Header
	DoubleAck(context.Context) error
	Nak() error
	NakWithDelay(delay time.Duration) error
	InProgress() error
	Term() error
	TermWithReason(reason string) error
}

// Client represents the main NATS JetStream client.
type Client interface {
	Publish(ctx context.Context, stream string, message []byte) error
	Subscribe(ctx context.Context, stream string) (*pubsub.Message, error)
	Close() error
}

type Subscription interface {
	Fetch(batch int, opts ...nats.PullOpt) ([]*nats.Msg, error)
	Drain() error
	Unsubscribe() error
	NextMsg(timeout time.Duration) (*nats.Msg, error)
}

// Connection represents the NATS connection.
type Connection interface {
	Status() nats.Status
	JetStream(opts ...nats.JSOpt) (JetStreamContext, error) // Use NATS' JetStreamContext
	Close()
	Drain() error
}

// JetStreamContext represents the NATS JetStream context.
type JetStreamContext interface {
	PublishMsg(m *nats.Msg, opts ...nats.PubOpt) (*nats.PubAck, error)
	Publish(subj string, data []byte, opts ...nats.PubOpt) (*nats.PubAck, error)
	PublishAsync(subj string, data []byte, opts ...nats.PubOpt) (nats.PubAckFuture, error)
	PublishMsgAsync(m *nats.Msg, opts ...nats.PubOpt) (nats.PubAckFuture, error)
	Subscribe(subj string, cb nats.MsgHandler, opts ...nats.SubOpt) (*nats.Subscription, error)
	SubscribeSync(subj string, opts ...nats.SubOpt) (*nats.Subscription, error)
	ChanSubscribe(subj string, ch chan *nats.Msg, opts ...nats.SubOpt) (*nats.Subscription, error)
	QueueSubscribe(subj, queue string, cb nats.MsgHandler, opts ...nats.SubOpt) (*nats.Subscription, error)
	QueueSubscribeSync(subj, queue string, opts ...nats.SubOpt) (*nats.Subscription, error)
	PullSubscribe(subj, durable string, opts ...nats.SubOpt) (*nats.Subscription, error)
	AddStream(cfg *nats.StreamConfig, opts ...nats.JSOpt) (*nats.StreamInfo, error)
	UpdateStream(cfg *nats.StreamConfig, opts ...nats.JSOpt) (*nats.StreamInfo, error)
	DeleteStream(name string, opts ...nats.JSOpt) error
	StreamInfo(stream string, opts ...nats.JSOpt) (*nats.StreamInfo, error)
	PurgeStream(name string, opts ...nats.JSOpt) error
	StreamsInfo(opts ...nats.JSOpt) <-chan *nats.StreamInfo
	StreamNames(opts ...nats.JSOpt) <-chan string
	GetMsg(name string, seq uint64, opts ...nats.JSOpt) (*nats.RawStreamMsg, error)
	GetLastMsg(name, subject string, opts ...nats.JSOpt) (*nats.RawStreamMsg, error)
	DeleteMsg(name string, seq uint64, opts ...nats.JSOpt) error
	AddConsumer(stream string, cfg *nats.ConsumerConfig, opts ...nats.JSOpt) (*nats.ConsumerInfo, error)
	UpdateConsumer(stream string, cfg *nats.ConsumerConfig, opts ...nats.JSOpt) (*nats.ConsumerInfo, error)
	DeleteConsumer(stream, consumer string, opts ...nats.JSOpt) error
	ConsumerInfo(stream, name string, opts ...nats.JSOpt) (*nats.ConsumerInfo, error)
	ConsumersInfo(stream string, opts ...nats.JSOpt) <-chan *nats.ConsumerInfo
	AccountInfo(opts ...nats.JSOpt) (*nats.AccountInfo, error)
}

// NatsConn interface abstracts the necessary methods from nats.Conn
type NatsConn interface {
	Status() nats.Status
	JetStream(opts ...nats.JSOpt) (nats.JetStreamContext, error)
	Close()
	Drain() error
}
