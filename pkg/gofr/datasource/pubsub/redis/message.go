package redis

import (
	"github.com/redis/go-redis/v9"
)

// Message implements the Committer interface for Redis PubSub messages.
type Message struct {
	msg    *redis.Message
	logger Logger
}

// newRedisMessage creates a new Redis message committer.
func newRedisMessage(msg *redis.Message, logger Logger) *Message {
	return &Message{
		msg:    msg,
		logger: logger,
	}
}

// Commit acknowledges the message.
// Note: Redis PubSub doesn't have explicit acknowledgment like Kafka,
// but we implement this for interface compatibility.
func (m *Message) Commit() {
	// Redis PubSub is fire-and-forget, so there's nothing to commit
	// Messages are automatically removed from the channel once received
	if m.logger != nil {
		m.logger.Debugf("Message acknowledged (Redis PubSub doesn't require explicit acknowledgment)")
	}
}
