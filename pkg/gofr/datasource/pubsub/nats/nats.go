package nats

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/health"
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
	Stream     string
	Subjects   []string
	MaxDeliver int
}

type Subscription struct {
	Sub     *nats.Subscription
	Handler MessageHandler
	Ctx     context.Context
	Cancel  context.CancelFunc
}

type natsConnWrapper struct {
	*nats.Conn
}

func (w *natsConnWrapper) NatsConn() *nats.Conn {
	return w.Conn
}

// NATSClient represents a client for NATS JetStream operations.
type NATSClient struct {
	Conn          ConnInterface
	Js            jetstream.JetStream
	mu            *sync.RWMutex
	Logger        pubsub.Logger
	Config        *Config
	Metrics       Metrics
	Subscriptions map[string]*Subscription
	subMu         sync.Mutex
}

func (n *NATSClient) CreateTopic(ctx context.Context, name string) error {
	return n.CreateStream(ctx, StreamConfig{
		Stream:   name,
		Subjects: []string{name},
	})
}

func (n *NATSClient) DeleteTopic(ctx context.Context, name string) error {
	n.Logger.Debugf("Deleting topic (stream) %s", name)

	err := n.Js.DeleteStream(ctx, name)
	if err != nil {
		if errors.Is(err, jetstream.ErrStreamNotFound) {
			n.Logger.Debugf("Stream %s not found, considering delete successful", name)
			return nil // If the stream doesn't exist, we consider it a success
		}
		n.Logger.Errorf("failed to delete stream (topic) %s: %v", name, err)
		return err
	}

	n.Logger.Debugf("Successfully deleted topic (stream) %s", name)
	return nil
}

func NewNATSClient(
	conf *Config,
	logger pubsub.Logger,
	metrics Metrics,
	natsConnect func(string, ...nats.Option) (ConnInterface, error),
	jetstreamNew func(*nats.Conn) (jetstream.JetStream, error),
) (*NATSClient, error) {
	if err := ValidateConfigs(conf); err != nil {
		logger.Errorf("could not initialize NATS JetStream: %v", err)
		return nil, err
	}

	logger.Debugf("connecting to NATS server '%s'", conf.Server)

	nc, err := natsConnect(conf.Server)
	if err != nil {
		logger.Errorf("failed to connect to NATS server at %v: %v", conf.Server, err)
		return nil, err
	}

	// Check connection status
	status := nc.Status()
	if status != nats.CONNECTED {
		logger.Errorf("unexpected NATS connection status: %v", status)
		return nil, fmt.Errorf("unexpected NATS connection status: %v", status)
	}

	js, err := jetstreamNew(nc.NatsConn())
	if err != nil {
		logger.Errorf("failed to create JetStream context: %v", err)
		return nil, err
	}

	logger.Logf("connected to NATS server '%s'", conf.Server)

	return &NATSClient{
		Conn:          nc,
		Js:            js,
		Logger:        logger,
		Config:        conf,
		Metrics:       metrics,
		Subscriptions: make(map[string]*Subscription),
	}, nil
}

func New(conf *Config, logger pubsub.Logger, metrics Metrics) (pubsub.Client, error) {
	// Wrapper function for nats.Connect
	natsConnectWrapper := func(url string, options ...nats.Option) (ConnInterface, error) {
		conn, err := nats.Connect(url, options...)
		if err != nil {
			return nil, err
		}
		return &natsConnWrapper{Conn: conn}, nil
	}

	// Wrapper function for jetstream.New
	jetstreamNewWrapper := func(nc *nats.Conn) (jetstream.JetStream, error) {
		return jetstream.New(nc)
	}

	// Create the NATSClient using the wrapper functions
	client, err := NewNATSClient(conf, logger, metrics, natsConnectWrapper, jetstreamNewWrapper)
	if err != nil {
		return nil, err
	}

	return &natsPubSubWrapper{client: client}, nil
}

// natsPubSubWrapper adapts NATSClient to pubsub.Client
type natsPubSubWrapper struct {
	client *NATSClient
}

func (w *natsPubSubWrapper) Publish(ctx context.Context, topic string, message []byte) error {
	return w.client.Publish(ctx, topic, message)
}

func (w *natsPubSubWrapper) Subscribe(ctx context.Context, topic string) (*pubsub.Message, error) {
	return w.client.Subscribe(ctx, topic)
}

func (w *natsPubSubWrapper) CreateTopic(ctx context.Context, name string) error {
	return w.client.CreateTopic(ctx, name)
}

func (w *natsPubSubWrapper) DeleteTopic(ctx context.Context, name string) error {
	return w.client.DeleteTopic(ctx, name)
}

func (w *natsPubSubWrapper) Close() error {
	return w.client.Close()
}

func (w *natsPubSubWrapper) Health() health.Health {
	// Implement health check
	status := health.StatusUp
	if w.client.Conn.Status() != nats.CONNECTED {
		status = health.StatusDown
	}
	return health.Health{
		Status: status,
		Details: map[string]interface{}{
			"server": w.client.Config.Server,
		},
	}
}

func (n *NATSClient) Publish(ctx context.Context, subject string, message []byte) error {
	n.Metrics.IncrementCounter(ctx, "app_pubsub_publish_total_count", "subject", subject)

	if n.Js == nil || subject == "" {
		err := errors.New("JetStream is not configured or subject is empty")
		n.Logger.Error(err.Error())
		return err
	}

	_, err := n.Js.Publish(ctx, subject, message)
	if err != nil {
		n.Logger.Errorf("failed to publish message to NATS JetStream: %v", err)
		return err
	}

	n.Metrics.IncrementCounter(ctx, "app_pubsub_publish_success_count", "subject", subject)

	return nil
}

func (n *NATSClient) Subscribe(ctx context.Context, topic string) (*pubsub.Message, error) {
	msgChan := make(chan *pubsub.Message)
	errChan := make(chan error, 1)

	go func() {
		err := n.subscribeInternal(ctx, topic, func(msg jetstream.Msg) {
			pubsubMsg := &pubsub.Message{
				Topic:     topic,
				Value:     msg.Data(),
				Committer: n.createCommitter(msg),
			}
			select {
			case msgChan <- pubsubMsg:
			case <-ctx.Done():
				return
			}
		})
		if err != nil {
			errChan <- err
		}
	}()

	select {
	case msg := <-msgChan:
		return msg, nil
	case err := <-errChan:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// createCommitter returns a Committer for the given NATS message
func (n *NATSClient) createCommitter(msg jetstream.Msg) pubsub.Committer {
	return &natsCommitter{msg: msg}
}

// natsCommitter implements the pubsub.Committer interface for NATS messages
type natsCommitter struct {
	msg jetstream.Msg
}

func (c *natsCommitter) Commit() {
	// return c.msg.Ack()
	err := c.msg.Ack()
	if err != nil {
		c.msg.Nak()
		log.Println("Error committing message:", err)
		return
	}
	return
}

func (c *natsCommitter) Rollback() error {
	return c.msg.Nak()
}

func (n *NATSClient) subscribeInternal(ctx context.Context, subject string, handler func(jetstream.Msg)) error {
	if n.Config.Consumer == "" {
		n.Logger.Error("consumer name not provided")
		return errors.New("consumer name not provided")
	}

	// Create or update the stream
	_, err := n.Js.CreateStream(ctx, jetstream.StreamConfig{
		Name:     n.Config.Stream.Stream,
		Subjects: n.Config.Stream.Subjects,
	})
	if err != nil {
		n.Logger.Errorf("failed to create or update stream: %v", err)
		return err
	}

	log.Println("Filter Subject", subject)

	// Create or update the consumer
	cons, err := n.Js.CreateOrUpdateConsumer(ctx, n.Config.Stream.Stream, jetstream.ConsumerConfig{
		Durable:       n.Config.Consumer,
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: subject,
		MaxDeliver:    n.Config.Stream.MaxDeliver,
	})
	if err != nil {
		n.Logger.Errorf("failed to create or update consumer: %v", err)
		return err
	}

	// Start fetching messages
	go n.startConsuming(ctx, cons, handler)

	return nil
}

func (n *NATSClient) startConsuming(ctx context.Context, cons jetstream.Consumer, handler func(jetstream.Msg)) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		msgs, err := cons.Fetch(n.Config.BatchSize, jetstream.FetchMaxWait(n.Config.MaxWait))
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			n.Logger.Errorf("failed to fetch messages: %v", err)
			time.Sleep(time.Second) // Backoff on error
			continue
		}

		for msg := range msgs.Messages() {
			handler(msg)
		}

		if msgs.Error() != nil {
			n.Logger.Errorf("error fetching messages: %v", msgs.Error())
		}
	}
}

func (n *NATSClient) processMessage(ctx context.Context, msg jetstream.Msg, handler MessageHandler) error {
	if err := handler(ctx, msg); err != nil {
		n.Logger.Errorf("failed to process message: %v", err)
		return n.nakMessage(msg)
	}

	return n.ackMessage(msg)
}

func (n *NATSClient) nakMessage(msg jetstream.Msg) error {
	if err := msg.Nak(); err != nil {
		n.Logger.Errorf("failed to nak message: %v", err)
		return err
	}
	return nil
}

func (n *NATSClient) ackMessage(msg jetstream.Msg) error {
	if err := msg.Ack(); err != nil {
		n.Logger.Errorf("failed to ack message: %v", err)
		return err
	}
	return nil
}

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

func (n *NATSClient) DeleteStream(ctx context.Context, name string) error {
	err := n.Js.DeleteStream(ctx, name)
	if err != nil {
		n.Logger.Errorf("failed to delete stream: %v", err)
		return err
	}

	return nil
}

func (n *NATSClient) CreateStream(ctx context.Context, cfg StreamConfig) error {
	n.Logger.Debugf("Creating stream %s", cfg.Stream)
	jsCfg := jetstream.StreamConfig{
		Name:     cfg.Stream,
		Subjects: cfg.Subjects,
	}
	_, err := n.Js.CreateStream(ctx, jsCfg)
	if err != nil {
		n.Logger.Errorf("failed to create stream: %v", err)
		return err
	}

	return nil
}

func (n *NATSClient) CreateOrUpdateStream(ctx context.Context, cfg jetstream.StreamConfig) (jetstream.Stream, error) {
	stream, err := n.Js.CreateOrUpdateStream(ctx, cfg)
	if err != nil {
		n.Logger.Errorf("failed to create or update stream: %v", err)
		return nil, err
	}

	return stream, nil
}

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
