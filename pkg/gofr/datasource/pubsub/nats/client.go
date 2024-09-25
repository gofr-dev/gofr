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

// Config defines the client client configuration.
type Config struct {
	Server      string
	CredsFile   string
	Stream      StreamConfig
	Consumer    string
	MaxWait     time.Duration
	MaxPullWait int
	BatchSize   int
}

// StreamConfig holds stream settings for client JetStream.
type StreamConfig struct {
	Stream     string
	Subjects   []string
	MaxDeliver int
	MaxWait    time.Duration
}

// subscription holds subscription information for client JetStream.
type subscription struct {
	handler messageHandler
	ctx     context.Context
	cancel  context.CancelFunc
}

type messageHandler func(context.Context, jetstream.Msg) error

// client represents a client for client JetStream operations.
type client struct {
	Conn          connInterface
	JetStream     jetstream.JetStream
	Logger        pubsub.Logger
	Config        *Config
	Metrics       Metrics
	Subscriptions map[string]*subscription
	subMu         sync.Mutex
}

// CreateTopic creates a new topic (stream) in client JetStream.
func (n *client) CreateTopic(ctx context.Context, name string) error {
	return n.CreateStream(ctx, StreamConfig{
		Stream:   name,
		Subjects: []string{name},
	})
}

// DeleteTopic deletes a topic (stream) in client JetStream.
func (n *client) DeleteTopic(ctx context.Context, name string) error {
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

// natsConnWrapper wraps a nats.Conn to implement the connInterface.
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

// New creates and returns a new client client.
func New(conf *Config, logger pubsub.Logger, metrics Metrics) (pubsub.Client, error) {
	if err := ValidateConfigs(conf); err != nil {
		logger.Errorf("could not initialize client JetStream: %v", err)
		return nil, err
	}

	logger.Debugf("connecting to client server '%s'", conf.Server)

	// Create connection options
	opts := []nats.Option{nats.Name("GoFr client jetStreamClient")}

	// Add credentials if provided
	if conf.CredsFile != "" {
		opts = append(opts, nats.UserCredentials(conf.CredsFile))
	}

	nc, err := nats.Connect(conf.Server, opts...)
	if err != nil {
		logger.Errorf("failed to connect to client server at %v: %v", conf.Server, err)
		return nil, err
	}

	// Check connection status
	status := nc.Status()
	if status != nats.CONNECTED {
		logger.Errorf("unexpected client connection status: %v", status)
		return nil, errConnectionStatus
	}

	js, err := jetstream.New(nc)
	if err != nil {
		logger.Errorf("failed to create JetStream context: %v", err)
		return nil, err
	}

	logger.Logf("connected to client server '%s'", conf.Server)

	client := &client{
		Conn:          &natsConnWrapper{nc},
		JetStream:     js,
		Logger:        logger,
		Config:        conf,
		Metrics:       metrics,
		Subscriptions: make(map[string]*subscription),
	}

	return &PubSubWrapper{Client: client}, nil
}

// Publish publishes a message to a topic.
func (n *client) Publish(ctx context.Context, subject string, message []byte) error {
	n.Metrics.IncrementCounter(ctx, "app_pubsub_publish_total_count", "subject", subject)

	if n.JetStream == nil || subject == "" {
		err := errJetStreamNotConfigured
		n.Logger.Error(err.Error())

		return err
	}

	_, err := n.JetStream.Publish(ctx, subject, message)
	if err != nil {
		n.Logger.Errorf("failed to publish message to client JetStream: %v", err)

		return err
	}

	n.Metrics.IncrementCounter(ctx, "app_pubsub_publish_success_count", "subject", subject)

	return nil
}

// Subscribe subscribes to a topic.
func (n *client) Subscribe(ctx context.Context, topic string, handler messageHandler) error {
	if n.Config.Consumer == "" {
		n.Logger.Error("consumer name not provided")
		return errConsumerNotProvided
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

func (n *client) startConsuming(ctx context.Context, cons jetstream.Consumer, handler messageHandler) {
	for {
		if err := n.fetchAndProcessMessages(ctx, cons, handler); err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}

			n.HandleFetchError(err)
		}
	}
}

func (n *client) fetchAndProcessMessages(ctx context.Context, cons jetstream.Consumer, handler messageHandler) error {
	msgs, err := cons.Fetch(n.Config.BatchSize, jetstream.FetchMaxWait(n.Config.MaxWait))
	if err != nil {
		return err
	}

	n.processMessages(ctx, msgs, handler)

	return msgs.Error()
}

// processMessages processes messages from a consumer.
func (n *client) processMessages(ctx context.Context, msgs jetstream.MessageBatch, handler messageHandler) {
	for msg := range msgs.Messages() {
		if err := n.HandleMessage(ctx, msg, handler); err != nil {
			n.Logger.Errorf("error handling message: %v", err)
		}
	}
}

// HandleMessage handles a message from a consumer.
func (n *client) HandleMessage(ctx context.Context, msg jetstream.Msg, handler messageHandler) error {
	if err := handler(ctx, msg); err != nil {
		n.Logger.Errorf("error handling message: %v", err)
		return n.NakMessage(msg)
	}

	return nil
}

// NakMessage naks a message from a consumer.
func (n *client) NakMessage(msg jetstream.Msg) error {
	if err := msg.Nak(); err != nil {
		n.Logger.Errorf("failed to NAK message: %v", err)

		return err
	}

	return nil
}

// HandleFetchError handles fetch errors.
func (n *client) HandleFetchError(err error) {
	n.Logger.Errorf("failed to fetch messages: %v", err)
	time.Sleep(time.Second) // Backoff on error
}

// Close closes the client client.
func (n *client) Close() error {
	n.subMu.Lock()
	for _, sub := range n.Subscriptions {
		sub.cancel()
	}

	n.Subscriptions = make(map[string]*subscription)
	n.subMu.Unlock()

	if n.Conn != nil {
		n.Conn.Close()
	}

	return nil
}

// DeleteStream deletes a stream in client JetStream.
func (n *client) DeleteStream(ctx context.Context, name string) error {
	err := n.JetStream.DeleteStream(ctx, name)
	if err != nil {
		n.Logger.Errorf("failed to delete stream: %v", err)

		return err
	}

	return nil
}

// CreateStream creates a stream in client JetStream.
func (n *client) CreateStream(ctx context.Context, cfg StreamConfig) error {
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

// CreateOrUpdateStream creates or updates a stream in client JetStream.
func (n *client) CreateOrUpdateStream(ctx context.Context, cfg *jetstream.StreamConfig) (jetstream.Stream, error) {
	n.Logger.Debugf("creating or updating stream %s", cfg.Name)

	stream, err := n.JetStream.CreateOrUpdateStream(ctx, *cfg)
	if err != nil {
		n.Logger.Errorf("failed to create or update stream: %v", err)

		return nil, err
	}

	return stream, nil
}

// ValidateConfigs validates the configuration for client JetStream.
func ValidateConfigs(conf *Config) error {
	err := error(nil)

	if conf.Server == "" {
		err = errServerNotProvided
	}

	// check if subjects are provided
	if err == nil && len(conf.Stream.Subjects) == 0 {
		err = errSubjectsNotProvided
	}

	return err
}
