package redis

import (
	"context"
	"math"
	"time"

	"github.com/redis/go-redis/v9"

	"gofr.dev/pkg/gofr/datasource"
)

// pubSubMessage implements the Committer interface for Redis PubSub messages.
// Redis PubSub is fire-and-forget, so there's nothing to commit.
type pubSubMessage struct {
	msg *redis.Message
}

func newPubSubMessage(msg *redis.Message) *pubSubMessage {
	return &pubSubMessage{
		msg: msg,
	}
}

func (*pubSubMessage) Commit() {
	// Redis PubSub is fire-and-forget, so there's nothing to commit
}

// streamMessage implements the Committer interface for Redis Stream messages.
// It handles message acknowledgment for Redis Streams.
type streamMessage struct {
	client *redis.Client
	stream string
	group  string
	id     string
	logger datasource.Logger
}

func newStreamMessage(client *redis.Client, stream, group, id string, logger datasource.Logger) *streamMessage {
	return &streamMessage{
		client: client,
		stream: stream,
		group:  group,
		id:     id,
		logger: logger,
	}
}

func (m *streamMessage) Commit() {
	const maxRetries = 3

	const baseDelay = 100 * time.Millisecond

	const exponentialBase = 2

	var err error

	for attempt := 0; attempt < maxRetries; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), defaultRetryTimeout)

		err = m.client.XAck(ctx, m.stream, m.group, m.id).Err()

		cancel()

		if err == nil {
			return
		}

		// Exponential backoff: baseDelay * 2^attempt
		if attempt < maxRetries-1 {
			delay := time.Duration(float64(baseDelay) * math.Pow(exponentialBase, float64(attempt)))
			time.Sleep(delay)
		}
	}

	// All retries failed
	m.logger.Errorf("failed to acknowledge message %s in stream %s after %d attempts: %v", m.id, m.stream, maxRetries, err)
}
