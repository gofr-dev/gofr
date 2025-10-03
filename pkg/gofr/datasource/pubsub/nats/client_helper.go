package nats

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

// establishConnection handles the actual connection process to NATS and sets up jStream.
func (c *Client) establishConnection() error {
	connManager := NewConnectionManager(c.Config, c.logger, c.natsConnector, c.jetStreamCreator)
	if err := connManager.Connect(); err != nil {
		return err
	}

	c.connManager = connManager

	js, err := c.connManager.jetStream()
	if err != nil {
		return err
	}

	c.streamManager = newStreamManager(js, c.logger)
	c.subManager = newSubscriptionManager(batchSize)

	c.logger.Logf("connected to NATS server '%s'", c.Config.Server)

	return nil
}

func (c *Client) retryConnect() {
	for {
		c.logger.Debugf("connecting to NATS server at %v", c.Config.Server)

		if err := c.establishConnection(); err != nil {
			c.logger.Errorf("failed to connect to NATS server at %v: %v", c.Config.Server, err)

			time.Sleep(defaultRetryTimeout)

			continue
		}

		return
	}
}

func validateAndPrepare(config *Config, logger pubsub.Logger) error {
	if err := validateConfigs(config); err != nil {
		logger.Errorf("could not initialize NATS jStream: %v", err)

		return err
	}

	return nil
}

func (c *Client) createOrUpdateConsumer(
	ctx context.Context, js jetstream.JetStream, subject, consumerName string) (jetstream.Consumer, error) {
	cons, err := js.CreateOrUpdateConsumer(ctx, c.Config.Stream.Stream, jetstream.ConsumerConfig{
		Durable:       consumerName,
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: subject,
		MaxDeliver:    c.Config.Stream.MaxDeliver,
		DeliverPolicy: jetstream.DeliverNewPolicy,
	})
	if err != nil {
		c.logger.Errorf("failed to create or update consumer: %v", err)
		return nil, err
	}

	return cons, nil
}

func (c *Client) processMessages(ctx context.Context, cons jetstream.Consumer, subject string, handler messageHandler) {
	for ctx.Err() == nil {
		if err := c.fetchAndProcessMessages(ctx, cons, subject, handler); err != nil {
			c.logger.Errorf("Error in message processing loop for subject %s: %v", subject, err)
		}
	}
}

func (c *Client) fetchAndProcessMessages(ctx context.Context, cons jetstream.Consumer, subject string, handler messageHandler) error {
	msgs, err := cons.Fetch(1, jetstream.FetchMaxWait(c.Config.MaxWait))
	if err != nil {
		if !errors.Is(err, context.DeadlineExceeded) {
			c.logger.Errorf("Error fetching messages for subject %s: %v", subject, err)
		}

		return err
	}

	return c.processFetchedMessages(ctx, msgs, handler, subject)
}

func (c *Client) processFetchedMessages(ctx context.Context, msgs jetstream.MessageBatch, handler messageHandler, subject string) error {
	for msg := range msgs.Messages() {
		if err := c.handleMessage(ctx, msg, handler); err != nil {
			c.logger.Errorf("Error processing message: %v", err)
		}
	}

	if err := msgs.Error(); err != nil {
		c.logger.Errorf("Error in message batch for subject %s: %v", subject, err)
		return err
	}

	return nil
}

func (c *Client) handleMessage(ctx context.Context, msg jetstream.Msg, handler messageHandler) error {
	err := handler(ctx, msg)
	if err == nil {
		if ackErr := msg.Ack(); ackErr != nil {
			c.logger.Errorf("Error sending ACK for message: %v", ackErr)
			return ackErr
		}

		return nil
	}

	c.logger.Errorf("Error handling message: %v", err)

	if nakErr := msg.Nak(); nakErr != nil {
		c.logger.Debugf("Error sending NAK for message: %v", nakErr)

		return nakErr
	}

	return err
}

// parseQueryArgs parses the query arguments.
func parseQueryArgs(args ...any) (timeout time.Duration, limit int) {
	// Default values
	timeout = defaultQueryTimeout
	limit = 100

	if len(args) > 0 {
		// First argument can be a custom timeout
		if val, ok := args[0].(time.Duration); ok && val > 0 {
			timeout = val
		}
	}

	if len(args) > 1 {
		// Second argument is the message limit
		if val, ok := args[1].(int); ok && val > 0 {
			limit = val
		}
	}

	return timeout, limit
}

// collectMessages fetches messages from the consumer and combines them.
func collectMessages(ctx context.Context, cons jetstream.Consumer, limit int,
	config *Config, logger pubsub.Logger) ([]byte, error) {
	var result []byte

	messagesCollected := 0

	for messagesCollected < limit {
		// Fetch messages with a batch size based on remaining needed messages
		fetchSize := minInt(batchSize, limit-messagesCollected)
		if fetchSize <= 0 {
			break
		}

		msgs, err := fetchBatch(cons, fetchSize, config.MaxWait, logger)
		if err != nil {
			return result, err
		}

		collected := processBatch(ctx, msgs, &result, &messagesCollected, limit, logger)
		if !collected {
			break
		}
	}

	if strings.Contains(cons.CachedInfo().Config.FilterSubject, goFrNatsStreamName) && len(result) == 0 {
		logger.Debugf("No migration records found in stream %s", config.Stream.Stream)
	}

	return result, nil
}

func fetchBatch(cons jetstream.Consumer, fetchSize int,
	maxWait time.Duration, logger pubsub.Logger) (jetstream.MessageBatch, error) {
	msgs, err := cons.Fetch(fetchSize, jetstream.FetchMaxWait(maxWait))
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, nil
		}

		logger.Errorf("Error fetching messages: %v", err)

		return nil, err
	}

	return msgs, nil
}

func processBatch(ctx context.Context, msgs jetstream.MessageBatch,
	result *[]byte, messagesCollected *int, limit int, logger pubsub.Logger) bool {
	receivedAny := false

	for msg := range msgs.Messages() {
		receivedAny = true

		// Add newline separator between messages
		if len(*result) > 0 {
			*result = append(*result, '\n')
		}

		// Append message data
		*result = append(*result, msg.Data()...)

		// Acknowledge the message
		if err := msg.Ack(); err != nil {
			logger.Debugf("Error acknowledging message: %v", err)
		}

		*messagesCollected++
		if *messagesCollected >= limit {
			break
		}
	}

	if !receivedAny || errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(ctx.Err(), context.Canceled) {
		return false
	}

	return true
}

// Helper function for Go versions < 1.21.
func minInt(a, b int) int {
	if a < b {
		return a
	}

	return b
}

func checkClient(c *Client) error {
	if c == nil {
		return errClientNotConnected
	}

	if c.connManager == nil {
		return errClientNotConnected
	}

	if !c.connManager.isConnected() {
		return errClientNotConnected
	}

	return nil
}

func createQueryContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return ctx, func() {}
	}

	return context.WithTimeout(ctx, timeout)
}

func (c *Client) getStreamName(query string) string {
	if query == goFrNatsStreamName {
		return goFrNatsStreamName
	}

	return c.Config.Stream.Stream
}

func (c *Client) createConsumer(ctx context.Context, js jetstream.JetStream, streamName,
	query, consumerName string) (jetstream.Consumer, error) {
	cons, err := js.CreateOrUpdateConsumer(ctx, streamName, jetstream.ConsumerConfig{
		Durable:       consumerName,
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: query,
		DeliverPolicy: jetstream.DeliverAllPolicy,
		AckWait:       c.Config.MaxWait,
	})
	if err != nil {
		c.logger.Errorf("failed to create consumer for query: %v", err)

		return nil, err
	}

	return cons, nil
}

func (c *Client) cleanupConsumer(js jetstream.JetStream, streamName string, cons jetstream.Consumer) {
	deleteCtx, cancel := context.WithTimeout(context.Background(), defaultDeleteTimeout)
	defer cancel()

	info, err := cons.Info(deleteCtx)
	if err == nil {
		if err := js.DeleteConsumer(deleteCtx, streamName, info.Name); err != nil {
			c.logger.Debugf("failed to delete temporary consumer: %v", err)
		}
	}
}
