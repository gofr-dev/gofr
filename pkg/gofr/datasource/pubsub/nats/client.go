package nats

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel/trace"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

// Client represents a Client for NATS JetStream operations.
type Client struct {
	connManager      ConnectionManagerInterface
	subManager       SubscriptionManagerInterface
	streamManager    StreamManagerInterface
	Config           *Config
	logger           pubsub.Logger
	metrics          Metrics
	tracer           trace.Tracer
	natsConnector    NATSConnector
	jetStreamCreator JetStreamCreator
}

type messageHandler func(context.Context, jetstream.Msg) error

// NewClient creates a new NATS JetStream client.
func NewClient(cfg *Config, logger pubsub.Logger, metrics Metrics, tracer trace.Tracer) *Client {
	return &Client{
		Config:           cfg,
		logger:           logger,
		metrics:          metrics,
		tracer:           tracer,
		natsConnector:    &DefaultNATSConnector{},
		jetStreamCreator: &DefaultJetStreamCreator{},
	}
}

// Connect establishes a connection to NATS and sets up JetStream.
func (c *Client) Connect() error {
	if err := c.validateAndPrepare(); err != nil {
		return err
	}

	c.connManager = NewConnectionManager(c.Config, c.logger, c.natsConnector, c.jetStreamCreator)
	if err := c.connManager.Connect(); err != nil {
		return err
	}

	c.streamManager = NewStreamManager(c.connManager.JetStream(), c.logger)
	c.subManager = NewSubscriptionManager(c.Config.BatchSize)
	c.logSuccessfulConnection()
	return nil
}

func (c *Client) validateAndPrepare() error {
	if err := ValidateConfigs(c.Config); err != nil {
		c.logger.Errorf("could not initialize NATS JetStream: %v", err)
		return err
	}
	return nil
}

func (c *Client) logSuccessfulConnection() {
	if c.logger != nil {
		c.logger.Logf("connected to NATS server '%s'", c.Config.Server)
	}
}

// UseLogger sets the logger for the NATS client.
func (c *Client) UseLogger(logger any) {
	if l, ok := logger.(pubsub.Logger); ok {
		c.logger = l
	}
}

// UseTracer sets the tracer for the NATS client.
func (c *Client) UseTracer(tracer any) {
	if t, ok := tracer.(trace.Tracer); ok {
		c.tracer = t
	}
}

// UseMetrics sets the metrics for the NATS client.
func (c *Client) UseMetrics(metrics any) {
	if m, ok := metrics.(Metrics); ok {
		c.metrics = m
	}
}

// Publish publishes a message to a topic.
func (c *Client) Publish(ctx context.Context, subject string, message []byte) error {
	return c.connManager.Publish(ctx, subject, message, c.metrics)
}

// Subscribe subscribes to a topic and returns a single message.
func (c *Client) Subscribe(ctx context.Context, topic string) (*pubsub.Message, error) {
	return c.subManager.Subscribe(ctx, topic, c.connManager.JetStream(), c.Config, c.logger, c.metrics)
}

// SubscribeWithHandler subscribes to a topic with a message handler.
func (c *Client) SubscribeWithHandler(ctx context.Context, subject string, handler messageHandler) error {
	js := c.connManager.JetStream()

	// Create a unique consumer name
	consumerName := fmt.Sprintf("%s_%s", c.Config.Consumer, strings.ReplaceAll(subject, ".", "_"))

	// Create or update the consumer
	cons, err := js.CreateOrUpdateConsumer(ctx, c.Config.Stream.Stream, jetstream.ConsumerConfig{
		Durable:       consumerName,
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: subject,
		MaxDeliver:    c.Config.Stream.MaxDeliver,
		DeliverPolicy: jetstream.DeliverNewPolicy,
	})
	if err != nil {
		c.logger.Errorf("failed to create or update consumer: %v", err)
		return err
	}

	// Start a goroutine to process messages
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				msgs, err := cons.Fetch(1, jetstream.FetchMaxWait(c.Config.MaxWait))
				if err != nil {
					if !errors.Is(err, context.DeadlineExceeded) {
						c.logger.Errorf("Error fetching messages for subject %s: %v", subject, err)
					}
					continue
				}

				for msg := range msgs.Messages() {
					err := handler(ctx, msg)
					if err != nil {
						c.logger.Errorf("Error handling message: %v", err)
						err := msg.Nak()
						if err != nil {
							c.logger.Errorf("Error sending NAK for message: %v", err)
							return
						}
					} else {
						err := msg.Ack()
						if err != nil {
							c.logger.Errorf("Error sending ACK for message: %v", err)
							return
						}
					}
				}

				if err := msgs.Error(); err != nil {
					c.logger.Errorf("Error in message batch for subject %s: %v", subject, err)
				}
			}
		}
	}()

	return nil
}

// Close closes the Client.
func (c *Client) Close(ctx context.Context) error {
	c.subManager.Close()
	if c.connManager != nil {
		c.connManager.Close(ctx)
	}
	return nil
}

// CreateTopic creates a new topic (stream) in NATS JetStream.
func (c *Client) CreateTopic(ctx context.Context, name string) error {
	return c.streamManager.CreateStream(ctx, StreamConfig{
		Stream:   name,
		Subjects: []string{name},
	})
}

// DeleteTopic deletes a topic (stream) in NATS JetStream.
func (c *Client) DeleteTopic(ctx context.Context, name string) error {
	return c.streamManager.DeleteStream(ctx, name)
}

// CreateStream creates a new stream in NATS JetStream.
func (c *Client) CreateStream(ctx context.Context, cfg StreamConfig) error {
	return c.streamManager.CreateStream(ctx, cfg)
}

// DeleteStream deletes a stream in NATS JetStream.
func (c *Client) DeleteStream(ctx context.Context, name string) error {
	return c.streamManager.DeleteStream(ctx, name)
}

// CreateOrUpdateStream creates or updates a stream in NATS JetStream.
func (c *Client) CreateOrUpdateStream(ctx context.Context, cfg jetstream.StreamConfig) (jetstream.Stream, error) {
	return c.streamManager.CreateOrUpdateStream(ctx, &cfg)
}
