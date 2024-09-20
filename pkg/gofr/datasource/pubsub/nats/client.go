package nats

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

//go:generate mockgen -destination=mock_jetstream.go -package=nats github.com/nats-io/nats.go/jetstream JetStream,Stream,Consumer,Msg,MessageBatch

// Config defines the NATS client configuration.
type Config struct {
	Server      string
	CredsFile   string
	Stream      StreamConfig
	Consumer    string
	MaxWait     time.Duration
	MaxPullWait int
	BatchSize   int
}

// StreamConfig holds stream settings for NATS JetStream.
type StreamConfig struct {
	Stream     string
	Subjects   []string
	MaxDeliver int
	MaxWait    time.Duration
}

// Subscription holds subscription information for NATS JetStream.
type Subscription struct {
	Handler MessageHandler
	Ctx     context.Context
	Cancel  context.CancelFunc
}

type MessageHandler func(context.Context, jetstream.Msg) error

// NATSClient represents a client for NATS JetStream operations.
type NATSClient struct {
	Conn          ConnInterface
	JetStream     jetstream.JetStream
	Logger        pubsub.Logger
	Config        *Config
	Metrics       Metrics
	Subscriptions map[string]*Subscription
	subMu         sync.Mutex
}

// CreateTopic creates a new topic (stream) in NATS JetStream.
func (n *NATSClient) CreateTopic(ctx context.Context, name string) error {
	return n.CreateStream(ctx, StreamConfig{
		Stream:   name,
		Subjects: []string{name},
	})
}

// DeleteTopic deletes a topic (stream) in NATS JetStream.
func (n *NATSClient) DeleteTopic(ctx context.Context, name string) error {
	n.Logger.Debugf("deleting topic (stream) %s", name)

	err := n.JetStream.DeleteStream(ctx, name)
	if err != nil {
		if errors.Is(err, jetstream.ErrStreamNotFound) {
			n.Logger.Debugf("stream %s not found, considering delete successful", name)

			return nil // If the stream doesn't exist, we consider it a success
		}

		n.Logger.Errorf("failed to delete stream (topic) %s: %v", name, err)

		return err
	}

	n.Logger.Debugf("successfully deleted topic (stream) %s", name)

	return nil
}

// natsConnWrapper wraps a nats.Conn to implement the ConnInterface.
type natsConnWrapper struct {
	*nats.Conn
}

func (w *natsConnWrapper) Status() nats.Status {
	return w.Conn.Status()
}

func (w *natsConnWrapper) Close() {
	w.Conn.Close()
}

func (w *natsConnWrapper) NatsConn() *nats.Conn {
	return w.Conn
}

// New creates and returns a new NATS client.
func New(conf *Config, logger pubsub.Logger, metrics Metrics) (pubsub.Client, error) {
	if err := ValidateConfigs(conf); err != nil {
		logger.Errorf("could not initialize NATS JetStream: %v", err)
		return nil, err
	}

	logger.Debugf("connecting to NATS server '%s'", conf.Server)

	// Create connection options
	opts := []nats.Option{nats.Name("GoFr NATS Client")}

	// Add credentials if provided
	if conf.CredsFile != "" {
		opts = append(opts, nats.UserCredentials(conf.CredsFile))
	}

	nc, err := nats.Connect(conf.Server, opts...)
	if err != nil {
		logger.Errorf("failed to connect to NATS server at %v: %v", conf.Server, err)
		return nil, err
	}

	// Check connection status
	status := nc.Status()
	if status != nats.CONNECTED {
		logger.Errorf("unexpected NATS connection status: %v", status)
		return nil, ErrConnectionStatus
	}

	js, err := jetstream.New(nc)
	if err != nil {
		logger.Errorf("failed to create JetStream context: %v", err)
		return nil, err
	}

	logger.Logf("connected to NATS server '%s'", conf.Server)

	client := &NATSClient{
		Conn:          &natsConnWrapper{nc},
		JetStream:     js,
		Logger:        logger,
		Config:        conf,
		Metrics:       metrics,
		Subscriptions: make(map[string]*Subscription),
	}

	return &NatsPubSubWrapper{Client: client}, nil
}

// Publish publishes a message to a topic.
func (n *NATSClient) Publish(ctx context.Context, subject string, message []byte) error {
	n.Metrics.IncrementCounter(ctx, "app_pubsub_publish_total_count", "subject", subject)

	if n.JetStream == nil || subject == "" {
		err := ErrJetStreamNotConfigured
		n.Logger.Error(err.Error())

		return err
	}

	_, err := n.JetStream.Publish(ctx, subject, message)
	if err != nil {
		n.Logger.Errorf("failed to publish message to NATS JetStream: %v", err)

		return err
	}

	n.Metrics.IncrementCounter(ctx, "app_pubsub_publish_success_count", "subject", subject)

	return nil
}

// Subscribe subscribes to a topic.
func (n *NATSClient) Subscribe(ctx context.Context, topic string, handler MessageHandler) error {
	if n.Config.Consumer == "" {
		n.Logger.Error("consumer name not provided")
		return ErrConsumerNotProvided
	}

	// Create a unique consumer name for each topic
	consumerName := fmt.Sprintf("%s_%s", n.Config.Consumer, topic)

	// Create or update the consumer
	cons, err := n.JetStream.CreateOrUpdateConsumer(ctx, n.Config.Stream.Stream, jetstream.ConsumerConfig{
		Durable:       consumerName,
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: topic,
		MaxDeliver:    n.Config.Stream.MaxDeliver,
		DeliverPolicy: jetstream.DeliverNewPolicy,
		AckWait:       30 * time.Second,
	})
	if err != nil {
		n.Logger.Errorf("failed to create or update consumer: %v", err)
		return err
	}

	// Start fetching messages
	go n.startConsuming(ctx, cons, handler)

	return nil
}

func (n *NATSClient) startConsuming(ctx context.Context, cons jetstream.Consumer, handler MessageHandler) {
	for {
		if err := n.fetchAndProcessMessages(ctx, cons, handler); err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}

			n.HandleFetchError(err)
		}
	}
}

func (n *NATSClient) fetchAndProcessMessages(ctx context.Context, cons jetstream.Consumer, handler MessageHandler) error {
	msgs, err := cons.Fetch(n.Config.BatchSize, jetstream.FetchMaxWait(n.Config.MaxWait))
	if err != nil {
		return err
	}

	n.processMessages(ctx, msgs, handler)

	return msgs.Error()
}

// processMessages processes messages from a consumer.
func (n *NATSClient) processMessages(ctx context.Context, msgs jetstream.MessageBatch, handler MessageHandler) {
	for msg := range msgs.Messages() {
		if err := n.HandleMessage(ctx, msg, handler); err != nil {
			n.Logger.Errorf("error handling message: %v", err)
		}
	}
}

// HandleMessage handles a message from a consumer.
func (n *NATSClient) HandleMessage(ctx context.Context, msg jetstream.Msg, handler MessageHandler) error {
	if err := handler(ctx, msg); err != nil {
		n.Logger.Errorf("error handling message: %v", err)
		return n.NakMessage(msg)
	}

	return nil
}

// NakMessage naks a message from a consumer.
func (n *NATSClient) NakMessage(msg jetstream.Msg) error {
	if err := msg.Nak(); err != nil {
		n.Logger.Errorf("Failed to NAK message: %v", err)

		return err
	}

	return nil
}

// HandleFetchError handles fetch errors.
func (n *NATSClient) HandleFetchError(err error) {
	n.Logger.Errorf("failed to fetch messages: %v", err)
	time.Sleep(time.Second) // Backoff on error
}

// Close closes the NATS client.
func (n *NATSClient) Close() error {
	n.subMu.Lock()
	for _, sub := range n.Subscriptions {
		sub.Cancel()
	}

	n.Subscriptions = make(map[string]*Subscription)
	n.subMu.Unlock()

	if n.Conn != nil {
		n.Conn.Close()
	}

	return nil
}

// DeleteStream deletes a stream in NATS JetStream.
func (n *NATSClient) DeleteStream(ctx context.Context, name string) error {
	err := n.JetStream.DeleteStream(ctx, name)
	if err != nil {
		n.Logger.Errorf("failed to delete stream: %v", err)

		return err
	}

	return nil
}

// CreateStream creates a stream in NATS JetStream.
func (n *NATSClient) CreateStream(ctx context.Context, cfg StreamConfig) error {
	n.Logger.Debugf("creating stream %s", cfg.Stream)
	jsCfg := jetstream.StreamConfig{
		Name:     cfg.Stream,
		Subjects: cfg.Subjects,
	}

	_, err := n.JetStream.CreateStream(ctx, jsCfg)
	if err != nil {
		n.Logger.Errorf("failed to create stream: %v", err)

		return err
	}

	return nil
}

// CreateOrUpdateStream creates or updates a stream in NATS JetStream.
func (n *NATSClient) CreateOrUpdateStream(ctx context.Context, cfg *jetstream.StreamConfig) (jetstream.Stream, error) {
	n.Logger.Debugf("creating or updating stream %s", cfg.Name)

	stream, err := n.JetStream.CreateOrUpdateStream(ctx, *cfg)
	if err != nil {
		n.Logger.Errorf("failed to create or update stream: %v", err)

		return nil, err
	}

	return stream, nil
}

// ValidateConfigs validates the configuration for NATS JetStream.
func ValidateConfigs(conf *Config) error {
	err := error(nil)

	if conf.Server == "" {
		err = ErrServerNotProvided
	}

	// check if subjects are provided
	if err == nil && len(conf.Stream.Subjects) == 0 {
		err = ErrSubjectsNotProvided
	}

	return err
}
