package nats

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

var natsConnect = nats.Connect

var jetStreamCreate = func(conn *nats.Conn, opts ...nats.JSOpt) (JetStreamContext, error) {
	js, err := conn.JetStream(opts...)
	if err != nil {
		return nil, err
	}
	return newJetStreamContextWrapper(js), nil
}

// Errors for NATS operations
var (
	ErrConsumerNotProvided    = errors.New("consumer name not provided")
	errServerNotProvided      = errors.New("NATS server address not provided")
	errPublisherNotConfigured = errors.New("can't publish message: publisher not configured or stream is empty")
	errStreamNotProvided      = errors.New("stream name not provided")
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
	conn      Connection
	js        JetStreamContext
	mu        *sync.RWMutex
	logger    pubsub.Logger
	config    Config
	metrics   Metrics
	fetchFunc func(*nats.Subscription, int, ...nats.PullOpt) ([]*nats.Msg, error)
}

type natsConnection struct {
	*nats.Conn
}

func (nc *natsConnection) Status() nats.Status {
	return nc.Conn.Status()
}

func (nc *natsConnection) JetStream(opts ...nats.JSOpt) (JetStreamContext, error) {
	js, err := nc.Conn.JetStream(opts...)
	if err != nil {
		return nil, err
	}
	return newJetStreamContextWrapper(js), nil
}

// New initializes a new NATS JetStream client.
func New(conf Config, logger pubsub.Logger, metrics Metrics) (*natsClient, error) {
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

	conn := &natsConnection{Conn: nc}

	js, err := jetStreamCreate(nc)
	if err != nil {
		logger.Errorf("failed to create JetStream context: %v", err)
		return nil, err
	}

	logger.Logf("connected to NATS server '%s'", conf.Server)

	return &natsClient{
		conn:    conn,
		js:      js,
		mu:      &sync.RWMutex{},
		logger:  logger,
		config:  conf,
		metrics: metrics,
	}, nil
}

func (n *natsClient) Publish(ctx context.Context, stream string, message []byte) error {
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

func (n *natsClient) Subscribe(ctx context.Context, stream string) (*pubsub.Message, error) {
	if n.config.Consumer == "" {
		n.logger.Error("consumer name not provided")
		return nil, ErrConsumerNotProvided
	}

	ctx, span := otel.GetTracerProvider().Tracer("gofr").Start(ctx, "nats-subscribe")
	defer span.End()

	n.metrics.IncrementCounter(ctx, "app_pubsub_subscribe_total_count", "stream", stream, "consumer", n.config.Consumer)

	n.mu.Lock()
	defer n.mu.Unlock()

	start := time.Now()

	sub, err := n.js.PullSubscribe(stream, n.config.Consumer, nats.PullMaxWaiting(128))
	if err != nil {
		errMsg := fmt.Sprintf("failed to create or attach consumer: %v", err)
		n.logger.Error(errMsg)
		return nil, errors.New(errMsg)
	}

	var msgs []*nats.Msg
	if n.fetchFunc != nil {
		msgs, err = n.fetchFunc(sub, 1, nats.MaxWait(n.config.MaxWait))
	} else {
		msgs, err = sub.Fetch(1, nats.MaxWait(n.config.MaxWait))
	}

	if len(msgs) == 0 {
		return nil, errors.New("no messages received")
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

func (n *natsClient) Close() error {
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

func (n *natsClient) DeleteStream(ctx context.Context, name string) error {
	return n.js.DeleteStream(name)
}

func (n *natsClient) CreateStream(ctx context.Context, name string) error {
	_, err := n.js.AddStream(&nats.StreamConfig{
		Name:     name,
		Subjects: []string{name},
	})
	return err
}

func validateConfigs(conf Config) error {
	if conf.Server == "" {
		return errServerNotProvided
	}
	if conf.Stream.Subject == "" {
		return errStreamNotProvided
	}
	return nil
}
