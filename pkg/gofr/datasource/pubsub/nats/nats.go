package nats

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

// Errors for NATS operations
var (
	ErrConsumerNotProvided    = errors.New("consumer name not provided")
	errServerNotProvided      = errors.New("NATS server address not provided")
	errPublisherNotConfigured = errors.New("can't publish message: publisher not configured or stream is empty")
	errNATSConnectionNotOpen  = errors.New("NATS connection not open")
)

// Config defines the NATS client configuration.
type Config struct {
	Server    string
	Stream    StreamConfig
	Consumer  string
	MaxWait   time.Duration
	BatchSize int
}

// StreamConfig holds stream settings for NATS JetStream.
type StreamConfig struct {
	Subject       string
	AckPolicy     nats.AckPolicy
	DeliverPolicy nats.DeliverPolicy
}

// natsClient represents a client for NATS JetStream operations.
type natsClient struct {
	conn     Connection
	js       JetStreamContext
	consumer map[string]jetstream.Consumer
	mu       *sync.RWMutex
	logger   pubsub.Logger
	config   Config
	metrics  Metrics
}

type natsConnection struct {
	*nats.Conn
}

func (nc *natsConnection) Status() nats.Status {
	return nc.Conn.Status()
}

func (nc *natsConnection) JetStream(opts ...nats.JSOpt) (nats.JetStreamContext, error) {
	return nc.Conn.JetStream(opts...)
}

// New initializes a new NATS JetStream client.
func New(conf Config, logger pubsub.Logger, metrics Metrics) (*natsClient, error) {
	if err := validateConfigs(conf); err != nil {
		logger.Errorf("could not initialize NATS JetStream: %v", err)
		return nil, err
	}

	nc, err := nats.Connect(conf.Server)
	if err != nil {
		logger.Errorf("failed to connect to NATS server at %v: %v", conf.Server, err)
		return nil, err
	}

	conn := &natsConnection{Conn: nc}

	natsJS, err := conn.JetStream()
	if err != nil {
		logger.Errorf("failed to create JetStream context: %v", err)
		return nil, err
	}

	// Wrap nats.JetStreamContext with custom wrapper
	js := newJetStreamContextWrapper(natsJS)

	return &natsClient{
		conn:    conn,
		js:      js,
		mu:      &sync.RWMutex{},
		logger:  logger,
		metrics: metrics,
	}, nil
}

// Publish sends a message to the specified NATS JetStream stream.
func (n *natsClient) Publish(ctx context.Context, stream string, message []byte) error {
	if stream == "" {
		return errPublisherNotConfigured
	}

	_, err := n.js.Publish(stream, message)
	if err != nil {
		n.logger.Errorf("failed to publish message: %v", err)
		return err
	}

	return nil
}

// Subscribe listens for messages on the specified stream and returns the next available message.
func (n *natsClient) Subscribe(ctx context.Context, stream string) (*pubsub.Message, error) {
	if n.config.Consumer == "" {
		n.logger.Error("consumer name not provided")
		return nil, ErrConsumerNotProvided
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	// Define consumer configuration
	consCfg := &nats.ConsumerConfig{
		Durable:       n.config.Consumer,
		AckPolicy:     n.config.Stream.AckPolicy,
		DeliverPolicy: n.config.Stream.DeliverPolicy,
	}

	// Ensure the consumer is created if it doesn't exist
	_, err := n.js.AddConsumer(stream, consCfg)
	if err != nil {
		n.logger.Errorf("failed to create or attach consumer: %v", err)
		return nil, err
	}

	// Fetch the next message
	sub, err := n.js.PullSubscribe(stream, n.config.Consumer, nats.PullMaxWaiting(128))
	if err != nil {
		return nil, err
	}

	messages, err := sub.Fetch(1, nats.MaxWait(n.config.MaxWait))
	if err != nil {
		return nil, err
	}

	if len(messages) == 0 {
		return nil, errors.New("no messages received")
	}

	msg := messages[0]

	// Acknowledge the message if needed
	if err := msg.Ack(); err != nil {
		n.logger.Errorf("failed to acknowledge message: %v", err)
		return nil, err
	}

	return &pubsub.Message{
		Value: msg.Data,
		Topic: stream,
	}, nil
}

// Close gracefully closes the NATS connection.
func (n *natsClient) Close() error {
	if n.conn != nil {
		return n.conn.Drain()
	}
	return errNATSConnectionNotOpen
}

// validateConfigs ensures that the necessary NATS configuration values are provided.
func validateConfigs(conf Config) error {
	if conf.Server == "" {
		return errServerNotProvided
	}
	return nil
}
