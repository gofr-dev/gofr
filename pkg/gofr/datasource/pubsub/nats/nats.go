package nats

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

// Config defines the NATS client configuration.
type Config struct {
	Server      string
	Stream      StreamConfig
	Consumer    string
	MaxWait     time.Duration
	MaxPullWait int
	BatchSize   int
}

// StreamConfig holds stream settings for NATS JetStream.
type StreamConfig struct {
	Subject       string
	AckPolicy     nats.AckPolicy
	DeliverPolicy nats.DeliverPolicy
}

// NatsClient represents a client for NATS JetStream operations.
type NatsClient struct {
	conn      Connection
	js        JetStreamContext
	mu        *sync.RWMutex
	logger    pubsub.Logger
	config    *Config
	metrics   Metrics
	fetchFunc func(*nats.Subscription, int, ...nats.PullOpt) ([]*nats.Msg, error)
}

// Update the natsConnection struct to use this interface.
type natsConnection struct {
	NatsConn
}

func (nc *natsConnection) JetStream(opts ...nats.JSOpt) (JetStreamContext, error) {
	js, err := nc.NatsConn.JetStream(opts...)
	if err != nil {
		return nil, err
	}

	return newJetStreamContextWrapper(js), nil
}

// New initializes a new NATS JetStream client.
func New(conf *Config,
	logger pubsub.Logger,
	metrics Metrics,
	natsConnect func(string, ...nats.Option) (*nats.Conn, error),
	jetStreamCreate func(conn *nats.Conn, opts ...nats.JSOpt) (JetStreamContext, error),
) (*NatsClient, error) {
	if err := validateConfigs(conf); err != nil {
		logger.Errorf("could not initialize NATS JetStream: %v", err)
		return nil, err
	}

	logger.Debugf("connecting to NATS server '%s'", conf.Server)

	nc, err := natsConnect(conf.Server)
	if err != nil {
		logger.Errorf("failed to connect to NATS server at %v: %v", conf.Server, err)
		return nil, err
	}

	conn := &natsConnection{NatsConn: nc}

	js, err := jetStreamCreate(nc)
	if err != nil {
		logger.Errorf("failed to create JetStream context: %v", err)
		return nil, err
	}

	logger.Logf("connected to NATS server '%s'", conf.Server)

	return &NatsClient{
		conn:    conn,
		js:      js,
		mu:      &sync.RWMutex{},
		logger:  logger,
		config:  conf,
		metrics: metrics,
	}, nil
}

func (n *NatsClient) Publish(ctx context.Context, stream string, message []byte) error {
	ctx, span := otel.GetTracerProvider().Tracer("gofr").Start(ctx, "nats-publish")
	defer span.End()

	n.metrics.IncrementCounter(ctx, "app_pubsub_publish_total_count", "stream", stream)

	if n.js == nil || stream == "" {
		n.logger.Error(errPublisherNotConfigured.Error())
		return errPublisherNotConfigured
	}

	start := time.Now()
	_, err := n.js.Publish(stream, message)
	end := time.Since(start)

	if err != nil {
		n.logger.Errorf("failed to publish message to NATS JetStream: %v", err)
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

func (n *NatsClient) Subscribe(ctx context.Context, stream string) (*pubsub.Message, error) {
	if n.config.Consumer == "" {
		n.logger.Error("consumer name not provided")
		return nil, ErrConsumerNotProvided
	}

	ctx, span := otel.GetTracerProvider().Tracer("gofr").Start(ctx, "nats-subscribe")
	defer span.End()

	n.metrics.IncrementCounter(ctx,
		"app_pubsub_subscribe_total_count", "stream", stream, "consumer", n.config.Consumer)

	n.mu.Lock()
	defer n.mu.Unlock()

	start := time.Now()

	sub, err := n.js.PullSubscribe(stream, n.config.Consumer, nats.PullMaxWaiting(n.config.MaxPullWait))
	if err != nil {
		n.logger.Error(err.Error())
		return nil, errSubscribe
	}

	var msgs []*nats.Msg
	if n.fetchFunc != nil {
		msgs, err = n.fetchFunc(sub, 1, nats.MaxWait(n.config.MaxWait))
	} else {
		msgs, err = sub.Fetch(1, nats.MaxWait(n.config.MaxWait))
	}

	if err != nil {
		n.logger.Errorf("failed to fetch messages: %v", err)
		return nil, err
	}

	if len(msgs) == 0 {
		return nil, ErrNoMessagesReceived
	}

	msg := msgs[0]

	m := pubsub.NewMessage(ctx)
	m.Value = msg.Data
	m.Topic = stream
	m.Committer = newNATSMessageWrapper(msg, n.logger)

	end := time.Since(start)

	n.logger.Debug(&pubsub.Log{
		Mode:          "SUB",
		CorrelationID: span.SpanContext().TraceID().String(),
		MessageValue:  string(msg.Data),
		Topic:         stream,
		Host:          n.config.Server,
		PubSubBackend: "NATS",
		Time:          end.Microseconds(),
	})

	n.metrics.IncrementCounter(ctx, "app_pubsub_subscribe_success_count", "stream", stream, "consumer", n.config.Consumer)

	return m, nil
}

func (n *NatsClient) Close() error {
	var err error

	if n.js != nil {
		if e := n.js.DeleteStream(n.config.Stream.Subject); e != nil {
			err = errors.Join(err, e)
		}
	}

	if n.conn != nil {
		err = errors.Join(err, n.conn.Drain())
	}

	return err
}

func (n *NatsClient) DeleteStream(_ context.Context, name string) error {
	return n.js.DeleteStream(name)
}

func (n *NatsClient) CreateStream(_ context.Context, name string) error {
	_, err := n.js.AddStream(&nats.StreamConfig{
		Name:     name,
		Subjects: []string{name},
	})

	return err
}

func NewNATSClient(conf *Config, logger pubsub.Logger, metrics Metrics) (*NatsClient, error) {
	return New(conf, logger, metrics, nats.Connect, defaultJetStreamCreate)
}

func defaultJetStreamCreate(conn *nats.Conn, opts ...nats.JSOpt) (JetStreamContext, error) {
	js, err := conn.JetStream(opts...)
	if err != nil {
		return nil, err
	}

	return newJetStreamContextWrapper(js), nil
}

func validateConfigs(conf *Config) error {
	err := error(nil)

	if conf.Server == "" {
		err = errServerNotProvided
	}

	if err == nil && conf.Stream.Subject == "" {
		err = errStreamNotProvided
	}

	return err
}
