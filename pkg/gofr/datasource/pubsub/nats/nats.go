package nats

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"gofr.dev/pkg/gofr"
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

type subscription struct {
	sub     *nats.Subscription
	handler MessageHandler
	ctx     context.Context
	cancel  context.CancelFunc
}

type natsConnWrapper struct {
	*nats.Conn
}

func (w *natsConnWrapper) NatsConn() *nats.Conn {
	return w.Conn
}

// NATSClient represents a client for NATS JetStream operations.
type NATSClient struct {
	conn          ConnInterface
	js            jetstream.JetStream
	mu            *sync.RWMutex
	logger        pubsub.Logger
	config        *Config
	metrics       Metrics
	subscriptions map[string]*subscription
	subMu         sync.Mutex
}

func NewNATSClient(
	conf *Config,
	logger pubsub.Logger,
	metrics Metrics,
	natsConnect func(string, ...nats.Option) (ConnInterface, error),
	jetstreamNew func(*nats.Conn) (jetstream.JetStream, error),
) (*NATSClient, error) {
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
		conn:          nc,
		js:            js,
		logger:        logger,
		config:        conf,
		metrics:       metrics,
		subscriptions: make(map[string]*subscription),
	}, nil
}

// New initializes a new NATS JetStream client.
func New(
	conf *Config,
	logger pubsub.Logger,
	metrics Metrics,
	natsConnect func(string, ...nats.Option) (*nats.Conn, error),
) (*NATSClient, error) {
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

	// Wrap the nats.Conn with our wrapper
	wrappedConn := &natsConnWrapper{Conn: nc}

	js, err := jetstream.New(nc)
	if err != nil {
		logger.Errorf("failed to create JetStream context: %v", err)
		return nil, err
	}

	logger.Logf("connected to NATS server '%s'", conf.Server)

	return &NATSClient{
		conn:          wrappedConn,
		js:            js,
		mu:            &sync.RWMutex{},
		logger:        logger,
		config:        conf,
		metrics:       metrics,
		subscriptions: make(map[string]*subscription),
	}, nil
}

func (n *NATSClient) Publish(ctx context.Context, subject string, message []byte) error {
	n.metrics.IncrementCounter(ctx, "app_pubsub_publish_total_count", "subject", subject)

	if n.js == nil || subject == "" {
		err := errors.New("JetStream is not configured or subject is empty")
		n.logger.Error(err.Error())
		return err
	}

	_, err := n.js.Publish(ctx, subject, message)
	if err != nil {
		n.logger.Errorf("failed to publish message to NATS JetStream: %v", err)
		return err
	}

	n.metrics.IncrementCounter(ctx, "app_pubsub_publish_success_count", "subject", subject)

	return nil
}

func (n *NATSClient) Subscribe(ctx context.Context, subject string, handler MessageHandler) error {
	if n.config.Consumer == "" {
		n.logger.Error("consumer name not provided")
		return errors.New("consumer name not provided")
	}

	// Create or update the stream
	_, err := n.js.CreateStream(ctx, jetstream.StreamConfig{
		Name:     n.config.Stream.Stream,
		Subjects: []string{n.config.Stream.Subject},
	})
	if err != nil {
		n.logger.Errorf("failed to create or update stream: %v", err)
		return err
	}

	// Create or update the consumer
	cons, err := n.js.CreateOrUpdateConsumer(ctx, n.config.Stream.Stream, jetstream.ConsumerConfig{
		Durable:       n.config.Consumer,
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: subject,
		MaxDeliver:    n.config.Stream.MaxDeliver,
	})
	if err != nil {
		n.logger.Errorf("failed to create or update consumer: %v", err)
		return err
	}

	// Start fetching messages
	go n.startConsuming(ctx, cons, handler)

	return nil
}

func (n *NATSClient) startConsuming(ctx context.Context, cons jetstream.Consumer, handler MessageHandler) {
	for {
		msgs, err := cons.Fetch(n.config.BatchSize, jetstream.FetchMaxWait(n.config.MaxWait))
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			n.logger.Errorf("failed to fetch messages: %v", err)
			time.Sleep(time.Second) // Backoff on error
			continue
		}

		for msg := range msgs.Messages() {
			if err := n.processMessage(ctx, msg, handler); err != nil {
				n.logger.Errorf("failed to process message: %v", err)
			}
			err := msg.Ack()
			if err != nil {
				n.logger.Errorf("failed to acknowledge message: %v", err)
				return
			}
		}

		if msgs.Error() != nil {
			n.logger.Errorf("error fetching messages: %v", msgs.Error())
		}
	}
}

func (n *NATSClient) processMessage(ctx context.Context, msg jetstream.Msg, handler MessageHandler) error {
	msgCtx := &gofr.Context{Context: ctx}

	if err := handler(msgCtx, msg); err != nil {
		n.logger.Errorf("failed to process message: %v", err)
		return n.nakMessage(msg)
	}

	return n.ackMessage(msg)
}

func (n *NATSClient) nakMessage(msg jetstream.Msg) error {
	if err := msg.Nak(); err != nil {
		n.logger.Errorf("failed to nak message: %v", err)
		return err
	}
	return nil
}

func (n *NATSClient) ackMessage(msg jetstream.Msg) error {
	if err := msg.Ack(); err != nil {
		n.logger.Errorf("failed to ack message: %v", err)
		return err
	}
	return nil
}

func (n *NATSClient) Close(ctx context.Context) error {
	n.subMu.Lock()
	for _, sub := range n.subscriptions {
		sub.cancel()
	}
	n.subscriptions = make(map[string]*subscription)
	n.subMu.Unlock()

	if n.conn != nil {
		n.conn.Close()
	}

	return nil
}

func (n *NATSClient) DeleteStream(ctx context.Context, name string) error {
	err := n.js.DeleteStream(ctx, name)
	if err != nil {
		n.logger.Errorf("failed to delete stream: %v", err)
		return err
	}

	return nil
}

func (n *NATSClient) CreateStream(ctx context.Context, cfg StreamConfig) error {
	jsCfg := jetstream.StreamConfig{
		Name:     cfg.Stream,
		Subjects: []string{cfg.Subject},
	}
	_, err := n.js.CreateStream(ctx, jsCfg)
	if err != nil {
		n.logger.Errorf("failed to create stream: %v", err)
		return err
	}

	return nil
}

func (n *NATSClient) CreateOrUpdateStream(ctx context.Context, cfg jetstream.StreamConfig) (jetstream.Stream, error) {
	stream, err := n.js.CreateOrUpdateStream(ctx, cfg)
	if err != nil {
		n.logger.Errorf("failed to create or update stream: %v", err)
		return nil, err
	}

	return stream, nil
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
