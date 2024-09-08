// Package nats provides a client for interacting with NATS JetStream.
// This package facilitates interaction with NATS JetStream, allowing publishing and subscribing to streams,
// managing consumer groups, and handling messages.
package nats

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

var (
	ErrConsumerNotProvided    = errors.New("consumer name not provided")
	errServerNotProvided      = errors.New("NATS server address not provided")
	errPublisherNotConfigured = errors.New("can't publish message. Publisher not configured or stream is empty")
	errNATSConnectionNotOpen  = errors.New("NATS connection not open")
)

type StreamConfig struct {
	Subject       string
	AckPolicy     nats.AckPolicy
	DeliverPolicy nats.DeliverPolicy
}

type Config struct {
	Server    string
	Stream    StreamConfig
	Consumer  string
	MaxWait   time.Duration
	BatchSize int
}

type natsClient struct {
	conn     *nats.Conn
	js       jetstream.JetStream
	consumer map[string]jetstream.Consumer

	mu *sync.RWMutex

	logger  pubsub.Logger
	config  Config
	metrics Metrics
}

//nolint:revive // We do not want anyone using the client without initialization steps.
func New(conf Config, logger pubsub.Logger, metrics Metrics) Client {
	if err := validateConfigs(conf); err != nil {
		logger.Errorf("could not initialize NATS JetStream, error: %v", err)
		return nil
	}

	logger.Debugf("connecting to NATS server '%s'", conf.Server)

	nc, err := nats.Connect(conf.Server)
	if err != nil {
		logger.Errorf("failed to connect to NATS at %v, error: %v", conf.Server, err)
		return nil
	}

	js, err := jetstream.New(nc)
	if err != nil {
		logger.Errorf("failed to create JetStream context, error: %v", err)
		return nil
	}

	logger.Logf("connected to NATS server '%s'", conf.Server)

	return &natsClient{
		config:   conf,
		conn:     nc,
		js:       js,
		consumer: make(map[string]jetstream.Consumer),
		mu:       &sync.RWMutex{},
		logger:   logger,
		metrics:  metrics,
	}
}

func (n *natsClient) Publish(ctx context.Context, stream string, message []byte) error {
	ctx, span := otel.GetTracerProvider().Tracer("gofr").Start(ctx, "nats-publish")
	defer span.End()

	n.metrics.IncrementCounter(ctx, "app_pubsub_publish_total_count", "stream", stream)

	if n.js == nil || stream == "" {
		return errPublisherNotConfigured
	}

	start := time.Now()
	_, err := n.js.Publish(ctx, stream, message)
	end := time.Since(start)

	if err != nil {
		n.logger.Errorf("failed to publish message to NATS JetStream, error: %v", err)
		return err
	}

	n.logger.Debug(&pubsub.Log{
		Mode:          "PUB",
		CorrelationID: span.SpanContext().TraceID().String(),
		MessageValue:  string(message),
		Topic:         stream,
		Host:          n.config.Server,
		PubSubBackend: "NATS",
		Time:          end.Microseconds(),
	})

	n.metrics.IncrementCounter(ctx, "app_pubsub_publish_success_count", "stream", stream)

	return nil
}

func (n *natsClient) Subscribe(ctx context.Context, stream string) (*pubsub.Message, error) {
	if n.config.Consumer == "" {
		n.logger.Error("cannot subscribe as consumer name is not provided in configs")
		return nil, ErrConsumerNotProvided
	}

	ctx, span := otel.GetTracerProvider().Tracer("gofr").Start(ctx, "nats-subscribe")
	defer span.End()

	n.metrics.IncrementCounter(ctx, "app_pubsub_subscribe_total_count", "stream", stream, "consumer", n.config.Consumer)

	// Lock the consumer map to ensure only one subscriber accesses the consumer at a time
	n.mu.Lock()
	defer n.mu.Unlock()

	var cons jetstream.Consumer
	var ok bool
	if cons, ok = n.consumer[stream]; !ok {
		str, err := n.js.Stream(ctx, stream)
		if err != nil {
			n.logger.Errorf("failed to get stream %s: %v", stream, err)
			return nil, err
		}

		cons, err = str.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
			Name:          n.config.Consumer,
			Durable:       n.config.Consumer,
			AckPolicy:     jetstream.AckPolicy(n.config.Stream.AckPolicy),
			DeliverPolicy: jetstream.DeliverPolicy(n.config.Stream.DeliverPolicy),
		})
		if err != nil {
			n.logger.Errorf("failed to create/update consumer for stream %s: %v", stream, err)
			return nil, err
		}
		n.consumer[stream] = cons
	}

	start := time.Now()

	// Fetch a single message from the stream
	msg, err := cons.Next(jetstream.FetchMaxWait(n.config.MaxWait))
	if err != nil {
		n.logger.Errorf("failed to read message from NATS stream %s: %v", stream, err)
		return nil, err
	}

	m := pubsub.NewMessage(ctx)
	m.Value = msg.Data()
	m.Topic = stream
	m.Committer = newNATSMessage(msg, n.logger)

	end := time.Since(start)

	n.logger.Debug(&pubsub.Log{
		Mode:          "SUB",
		CorrelationID: span.SpanContext().TraceID().String(),
		MessageValue:  string(msg.Data()),
		Topic:         stream,
		Host:          n.config.Server,
		PubSubBackend: "NATS",
		Time:          end.Microseconds(),
	})

	n.metrics.IncrementCounter(
		ctx, "app_pubsub_subscribe_success_count", "stream", stream, "consumer", n.config.Consumer)

	return m, nil
}

func validateConfigs(conf Config) error {
	if conf.Server == "" {
		return errServerNotProvided
	}
	// Add more config validations as needed
	return nil
}

func (n *natsClient) Close() error {
	if n.conn != nil {
		n.logger.Debug("NATS connection closing, draining messages..")
		// Drain the connection to ensure all messages are processed
		return n.conn.Drain()
	}
	return errNATSConnectionNotOpen
}
