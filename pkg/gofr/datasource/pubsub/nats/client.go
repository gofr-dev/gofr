package nats

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel/trace"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

//go:generate mockgen -destination=mock_tracer.go -package=nats go.opentelemetry.io/otel/trace Tracer

const defaultRetryTimeout = 2 * time.Second

var (
	errClientNotConnected = errors.New("nats client not connected")
	errEmptySubject       = errors.New("subject name cannot be empty")
)

const (
	goFrNatsStreamName   = "gofr_migrations"
	defaultDeleteTimeout = 5 * time.Second
	defaultQueryTimeout  = 30 * time.Second
	defaultMaxBytes      = 100 * 1024 * 1024
	defaultAckWait       = 30 * time.Second
)

// Client represents a Client for NATS jStream operations.
type Client struct {
	connManager      ConnectionManagerInterface
	subManager       SubscriptionManagerInterface
	subscriptions    map[string]context.CancelFunc
	subMutex         sync.Mutex
	streamManager    StreamManagerInterface
	Config           *Config
	logger           pubsub.Logger
	metrics          Metrics
	tracer           trace.Tracer
	natsConnector    Connector
	jetStreamCreator JetStreamCreator
}

type messageHandler func(context.Context, jetstream.Msg) error

// Connect establishes a connection to NATS and sets up jStream.
func (c *Client) Connect() error {
	c.logger.Debugf("connecting to NATS server at %v", c.Config.Server)

	if err := validateAndPrepare(c.Config, c.logger); err != nil {
		return err
	}

	if err := c.establishConnection(); err != nil {
		c.logger.Errorf("failed to connect to NATS server at %v: %v", c.Config.Server, err)

		go c.retryConnect()

		return err
	}

	return nil
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
	if err := checkClient(c); err != nil {
		return err
	}

	return c.connManager.Publish(ctx, subject, message, c.metrics)
}

// Subscribe subscribes to a topic and returns a single message.
func (c *Client) Subscribe(ctx context.Context, topic string) (*pubsub.Message, error) {
	for {
		if err := checkClient(c); err != nil {
			time.Sleep(defaultRetryTimeout)

			return nil, errClientNotConnected
		}

		js, err := c.connManager.jetStream()
		if err == nil {
			return c.subManager.Subscribe(ctx, topic, js, c.Config, c.logger, c.metrics)
		}

		c.logger.Debugf("Waiting for NATS connection to be established for topic %s", topic)

		time.Sleep(defaultRetryTimeout)
	}
}

func (c *Client) SubscribeWithHandler(ctx context.Context, subject string, handler messageHandler) error {
	c.subMutex.Lock()
	defer c.subMutex.Unlock()

	// Cancel any existing subscription for this subject
	c.cancelExistingSubscription(subject)

	js, err := c.connManager.jetStream()
	if err != nil {
		return err
	}

	consumerName := fmt.Sprintf("%s_%s", c.Config.Consumer, strings.ReplaceAll(subject, ".", "_"))

	cons, err := c.createOrUpdateConsumer(ctx, js, subject, consumerName)
	if err != nil {
		return err
	}

	// Create a new context for this subscription
	subCtx, cancel := context.WithCancel(ctx)
	c.subscriptions[subject] = cancel

	go func() {
		defer cancel() // Ensure the cancellation is handled properly

		c.processMessages(subCtx, cons, subject, handler)
	}()

	return nil
}

func (c *Client) cancelExistingSubscription(subject string) {
	if cancel, exists := c.subscriptions[subject]; exists {
		cancel()
		delete(c.subscriptions, subject)
	}
}

// Close closes the Client.
func (c *Client) Close(ctx context.Context) error {
	c.subManager.Close()

	if c.connManager != nil {
		c.connManager.Close(ctx)
	}

	return nil
}

// Query retrieves messages from a NATS stream/subject.
func (c *Client) Query(ctx context.Context, query string, args ...any) ([]byte, error) {
	if err := checkClient(c); err != nil {
		return nil, err
	}

	if query == "" {
		return nil, errEmptySubject
	}

	// Parse optional arguments
	timeout, limit := parseQueryArgs(args...)

	// Create a query context with timeout
	queryCtx, cancel := createQueryContext(ctx, timeout)
	defer cancel()

	js, err := c.connManager.jetStream()
	if err != nil {
		return nil, err
	}

	streamName := c.getStreamName(query)
	consumerName := fmt.Sprintf("query_%s_%d", query, time.Now().UnixNano())

	// Create a consumer
	cons, err := c.createConsumer(queryCtx, js, streamName, query, consumerName)
	if err != nil {
		return nil, err
	}
	defer c.cleanupConsumer(js, streamName, cons)

	// Collect messages
	return collectMessages(queryCtx, cons, limit, c.Config, c.logger)
}

// CreateTopic creates a new topic (stream) in NATS jStream.
func (c *Client) CreateTopic(ctx context.Context, name string) error {
	if err := checkClient(c); err != nil {
		return err
	}

	// For migrations stream, use special configuration with max bytes
	if name == goFrNatsStreamName {
		return c.streamManager.CreateStream(ctx, &StreamConfig{
			Stream:    name,
			Subjects:  []string{name},
			MaxBytes:  defaultMaxBytes,
			Storage:   "file",
			Retention: "limits",
			MaxAge:    365 * 24 * time.Hour,
		})
	}

	return c.streamManager.CreateStream(ctx, &StreamConfig{
		Stream:   name,
		Subjects: []string{name},
	})
}

// DeleteTopic deletes a topic (stream) in NATS jStream.
func (c *Client) DeleteTopic(ctx context.Context, name string) error {
	if err := checkClient(c); err != nil {
		return err
	}

	return c.streamManager.DeleteStream(ctx, name)
}

// CreateStream creates a new stream in NATS jStream.
func (c *Client) CreateStream(ctx context.Context, cfg *StreamConfig) error {
	if err := checkClient(c); err != nil {
		return err
	}

	return c.streamManager.CreateStream(ctx, cfg)
}

// DeleteStream deletes a stream in NATS jStream.
func (c *Client) DeleteStream(ctx context.Context, name string) error {
	if err := checkClient(c); err != nil {
		return err
	}

	return c.streamManager.DeleteStream(ctx, name)
}

// CreateOrUpdateStream creates or updates a stream in NATS jStream.
func (c *Client) CreateOrUpdateStream(ctx context.Context, cfg *jetstream.StreamConfig) (jetstream.Stream, error) {
	if err := checkClient(c); err != nil {
		return nil, err
	}

	return c.streamManager.CreateOrUpdateStream(ctx, cfg)
}

// GetJetStreamStatus returns the status of the jStream connection.
func GetJetStreamStatus(ctx context.Context, js jetstream.JetStream) (string, error) {
	_, err := js.AccountInfo(ctx)
	if err != nil {
		return jetStreamStatusError, err
	}

	return jetStreamStatusOK, nil
}
