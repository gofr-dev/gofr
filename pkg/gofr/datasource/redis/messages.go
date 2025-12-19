package redis

import (
	"context"

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
	ctx, cancel := context.WithTimeout(context.Background(), defaultRetryTimeout)
	defer cancel()

	err := m.client.XAck(ctx, m.stream, m.group, m.id).Err()
	if err != nil {
		m.logger.Errorf("failed to acknowledge message %s in stream %s: %v", m.id, m.stream, err)
		return
	}
}
