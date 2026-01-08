package sqs

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

const (
	defaultRetryDuration = 5 * time.Second
	defaultQueryTimeout  = 30 * time.Second
	defaultMaxMessages   = int32(10)
)

// Client represents an SQS client that implements the pubsub.Client interface.
type Client struct {
	client  *sqs.Client
	cfg     *Config
	logger  Logger
	metrics Metrics
	tracer  trace.Tracer

	// queueURLCache caches queue URLs to avoid repeated GetQueueUrl API calls.
	queueURLCache map[string]string
	cacheMu       sync.RWMutex
}

// New creates a new SQS client with the provided configuration.
// The client is not connected until Connect() is called.
func New(cfg *Config) *Client {
	if cfg == nil {
		cfg = &Config{}
	}

	cfg.setDefaults()

	return &Client{
		cfg:           cfg,
		queueURLCache: make(map[string]string),
	}
}

// UseLogger sets the logger for the SQS client.
func (c *Client) UseLogger(logger any) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

// UseMetrics sets the metrics for the SQS client.
func (c *Client) UseMetrics(metrics any) {
	if m, ok := metrics.(Metrics); ok {
		c.metrics = m
	}
}

// UseTracer sets the tracer for the SQS client.
func (c *Client) UseTracer(tracer any) {
	if t, ok := tracer.(trace.Tracer); ok {
		c.tracer = t
	}
}

// Connect establishes a connection to AWS SQS.
func (c *Client) Connect() {
	if c.logger == nil {
		return
	}

	if c.cfg.Region == "" {
		c.logger.Error("SQS region is required")
		return
	}

	c.logger.Debugf("connecting to AWS SQS in region: %s", c.cfg.Region)

	ctx := context.Background()

	awsCfg, err := c.loadAWSConfig(ctx)
	if err != nil {
		c.logger.Errorf("failed to load AWS config: %v", err)
		return
	}

	// Create SQS client with custom endpoint if provided (for LocalStack)
	opts := func(o *sqs.Options) {
		if c.cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(c.cfg.Endpoint)
		}
	}

	c.client = sqs.NewFromConfig(awsCfg, opts)

	// Verify connection by listing queues
	_, err = c.client.ListQueues(ctx, &sqs.ListQueuesInput{
		MaxResults: aws.Int32(1),
	})
	if err != nil {
		c.logger.Errorf("failed to connect to SQS: %v", err)
		c.client = nil

		go c.retryConnect()

		return
	}

	c.logger.Logf("connected to AWS SQS in region: %s", c.cfg.Region)
}

// loadAWSConfig loads the AWS configuration with credentials.
func (c *Client) loadAWSConfig(ctx context.Context) (aws.Config, error) {
	var opts []func(*config.LoadOptions) error

	opts = append(opts, config.WithRegion(c.cfg.Region))

	// Use static credentials if provided
	if c.cfg.AccessKeyID != "" && c.cfg.SecretAccessKey != "" {
		staticCreds := credentials.NewStaticCredentialsProvider(
			c.cfg.AccessKeyID,
			c.cfg.SecretAccessKey,
			c.cfg.SessionToken,
		)
		opts = append(opts, config.WithCredentialsProvider(staticCreds))
	}

	return config.LoadDefaultConfig(ctx, opts...)
}

// retryConnect attempts to reconnect to SQS on failure.
func (c *Client) retryConnect() {
	retryDuration := c.cfg.RetryDuration
	if retryDuration <= 0 {
		retryDuration = defaultRetryDuration
	}

	for {
		time.Sleep(retryDuration)

		c.logger.Debugf("retrying connection to SQS...")

		c.Connect()

		if c.client != nil {
			c.logger.Log("successfully reconnected to SQS")
			return
		}
	}
}

// Publish sends a message to the specified SQS queue.
// The topic parameter is the queue name (not the full URL).
func (c *Client) Publish(ctx context.Context, topic string, message []byte) error {
	if c.client == nil {
		return ErrClientNotConnected
	}

	if topic == "" {
		return ErrEmptyQueueName
	}

	ctx, span := c.startTrace(ctx, "sqs-publish")
	defer span.End()

	c.metrics.IncrementCounter(ctx, "app_pubsub_publish_total_count", "topic", topic)

	queueURL, err := c.getQueueURL(ctx, topic)
	if err != nil {
		c.logger.Errorf("failed to get queue URL for %s: %v", topic, err)
		return err
	}

	start := time.Now()

	input := &sqs.SendMessageInput{
		QueueUrl:    aws.String(queueURL),
		MessageBody: aws.String(string(message)),
	}

	if c.cfg.DelaySeconds > 0 {
		input.DelaySeconds = c.cfg.DelaySeconds
	}

	result, err := c.client.SendMessage(ctx, input)
	if err != nil {
		c.logger.Errorf("failed to publish message to queue %s: %v", topic, err)
		return err
	}

	duration := time.Since(start)

	c.logger.Debug(&Log{
		Mode:          "PUB",
		CorrelationID: span.SpanContext().TraceID().String(),
		MessageValue:  truncateMessage(string(message)),
		Queue:         topic,
		Host:          c.cfg.Region,
		PubSubBackend: "SQS",
		Time:          duration.Microseconds(),
	})

	c.logger.Debugf("message published to queue %s with ID: %s", topic, *result.MessageId)

	c.metrics.IncrementCounter(ctx, "app_pubsub_publish_success_count", "topic", topic)

	return nil
}

// Subscribe receives a single message from the specified SQS queue.
// The topic parameter is the queue name (not the full URL).
func (c *Client) Subscribe(ctx context.Context, topic string) (*pubsub.Message, error) {
	if c.client == nil {
		time.Sleep(c.cfg.RetryDuration)
		return nil, ErrClientNotConnected
	}

	if topic == "" {
		return nil, ErrEmptyQueueName
	}

	ctx, span := c.startTrace(ctx, "sqs-subscribe")
	defer span.End()

	c.metrics.IncrementCounter(ctx, "app_pubsub_subscribe_total_count", "topic", topic)

	queueURL, err := c.getQueueURL(ctx, topic)
	if err != nil {
		// Don't log error if context was canceled (graceful shutdown)
		if !isContextCanceled(err) {
			c.logger.Errorf("failed to get queue URL for %s: %v", topic, err)
		}

		return nil, err
	}

	start := time.Now()

	input := &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(queueURL),
		MaxNumberOfMessages: 1, // Subscribe returns one message at a time
		WaitTimeSeconds:     c.cfg.WaitTimeSeconds,
		VisibilityTimeout:   c.cfg.VisibilityTimeout,
		AttributeNames: []types.QueueAttributeName{
			types.QueueAttributeNameAll,
		},
		MessageSystemAttributeNames: []types.MessageSystemAttributeName{
			types.MessageSystemAttributeNameAll,
		},
	}

	result, err := c.client.ReceiveMessage(ctx, input)
	if err != nil {
		// Don't log error if context was canceled (graceful shutdown)
		if !isContextCanceled(err) {
			c.logger.Errorf("failed to receive message from queue %s: %v", topic, err)
		}

		return nil, err
	}

	if len(result.Messages) == 0 {
		return nil, nil // No messages available
	}

	sqsMsg := result.Messages[0]
	duration := time.Since(start)

	// Create pubsub message
	msg := pubsub.NewMessage(ctx)
	msg.Topic = topic
	msg.Value = []byte(*sqsMsg.Body)
	msg.MetaData = sqsMsg.MessageAttributes
	msg.Committer = newMessage(
		*sqsMsg.ReceiptHandle,
		queueURL,
		*sqsMsg.MessageId,
		c.client,
		c.logger,
	)

	c.logger.Debug(&Log{
		Mode:          "SUB",
		CorrelationID: span.SpanContext().TraceID().String(),
		MessageValue:  truncateMessage(*sqsMsg.Body),
		Queue:         topic,
		Host:          c.cfg.Region,
		PubSubBackend: "SQS",
		Time:          duration.Microseconds(),
	})

	c.metrics.IncrementCounter(ctx, "app_pubsub_subscribe_success_count", "topic", topic)

	return msg, nil
}

// CreateTopic creates a new SQS queue with the specified name.
// In SQS terminology, this creates a queue, not a topic.
func (c *Client) CreateTopic(ctx context.Context, name string) error {
	if c.client == nil {
		return ErrClientNotConnected
	}

	if name == "" {
		return ErrEmptyQueueName
	}

	c.logger.Debugf("creating SQS queue: %s", name)

	_, err := c.client.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(name),
	})
	if err != nil {
		c.logger.Errorf("failed to create queue %s: %v", name, err)
		return err
	}

	c.logger.Logf("SQS queue created: %s", name)

	return nil
}

// DeleteTopic deletes an SQS queue with the specified name.
func (c *Client) DeleteTopic(ctx context.Context, name string) error {
	if c.client == nil {
		return ErrClientNotConnected
	}

	if name == "" {
		return ErrEmptyQueueName
	}

	queueURL, err := c.getQueueURL(ctx, name)
	if err != nil {
		return err
	}

	c.logger.Debugf("deleting SQS queue: %s", name)

	_, err = c.client.DeleteQueue(ctx, &sqs.DeleteQueueInput{
		QueueUrl: aws.String(queueURL),
	})
	if err != nil {
		c.logger.Errorf("failed to delete queue %s: %v", name, err)
		return err
	}

	// Remove from cache
	c.cacheMu.Lock()
	delete(c.queueURLCache, name)
	c.cacheMu.Unlock()

	c.logger.Logf("SQS queue deleted: %s", name)

	return nil
}

// Query retrieves multiple messages from an SQS queue.
// Args: [0] = limit (int, max 10).
func (c *Client) Query(ctx context.Context, query string, args ...any) ([]byte, error) {
	if c.client == nil {
		return nil, ErrClientNotConnected
	}

	if query == "" {
		return nil, ErrEmptyQueueName
	}

	queueURL, err := c.getQueueURL(ctx, query)
	if err != nil {
		return nil, err
	}

	limit := c.parseQueryArgs(args...)

	// Use provided context or add default timeout
	readCtx := ctx
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc

		readCtx, cancel = context.WithTimeout(ctx, defaultQueryTimeout)
		defer cancel()
	}

	result, err := c.client.ReceiveMessage(readCtx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(queueURL),
		MaxNumberOfMessages: limit,
		WaitTimeSeconds:     c.cfg.WaitTimeSeconds,
	})
	if err != nil {
		return nil, err
	}

	if len(result.Messages) == 0 {
		return nil, nil
	}

	// Combine all message bodies
	messages := make([]string, 0, len(result.Messages))
	for _, msg := range result.Messages {
		messages = append(messages, *msg.Body)
	}

	return []byte("[" + strings.Join(messages, ",") + "]"), nil
}

// Close closes the SQS client connection.
// SQS client doesn't require explicit closing, but we implement this for interface compliance.
func (c *Client) Close() error {
	c.logger.Debug("closing SQS client")
	c.client = nil

	return nil
}

// getQueueURL retrieves the queue URL for a given queue name.
// It caches the result to avoid repeated API calls.
func (c *Client) getQueueURL(ctx context.Context, queueName string) (string, error) {
	// Check cache first
	c.cacheMu.RLock()

	if url, ok := c.queueURLCache[queueName]; ok {
		c.cacheMu.RUnlock()

		return url, nil
	}

	c.cacheMu.RUnlock()

	// Get queue URL from SQS
	result, err := c.client.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{
		QueueName: aws.String(queueName),
	})
	if err != nil {
		return "", ErrQueueNotFound
	}

	// Cache the result
	c.cacheMu.Lock()
	c.queueURLCache[queueName] = *result.QueueUrl
	c.cacheMu.Unlock()

	return *result.QueueUrl, nil
}

// startTrace starts a new trace span using the global OpenTelemetry tracer provider.
// This ensures proper trace propagation from the incoming context.
func (*Client) startTrace(ctx context.Context, name string) (context.Context, trace.Span) {
	return otel.GetTracerProvider().Tracer("gofr").Start(ctx, name)
}

// parseQueryArgs parses the query arguments for Query method.
func (*Client) parseQueryArgs(args ...any) int32 {
	if len(args) > 0 {
		if l, ok := args[0].(int32); ok && l > 0 && l <= 10 {
			return l
		}
	}

	return defaultMaxMessages
}

// truncateMessage truncates the message for logging purposes.
func truncateMessage(msg string) string {
	const maxLen = 100

	if len(msg) > maxLen {
		return msg[:maxLen] + "..."
	}

	return msg
}

// isContextCanceled checks if the error is due to context cancellation.
// This is used to suppress error logs during graceful shutdown.
func isContextCanceled(err error) bool {
	if err == nil {
		return false
	}

	// Check for standard context errors
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	// Check error message for "canceled" (AWS SDK wraps context errors)
	errMsg := err.Error()

	return strings.Contains(errMsg, "context canceled") ||
		strings.Contains(errMsg, "context deadline exceeded") ||
		strings.Contains(errMsg, "canceled")
}
