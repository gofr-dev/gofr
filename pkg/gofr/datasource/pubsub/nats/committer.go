package nats

import (
	"log"

	"github.com/nats-io/nats.go/jetstream"
)

// createTestCommitter is a helper function for tests to create a natsCommitter.
func createTestCommitter(msg jetstream.Msg) *natsCommitter {
	return &natsCommitter{msg: msg}
}

// natsCommitter implements the pubsub.Committer interface for client messages.
type natsCommitter struct {
	msg jetstream.Msg
}

// Commit commits the message.
func (c *natsCommitter) Commit() {
	if err := c.msg.Ack(); err != nil {
		log.Println("Error committing message:", err)

		// nak the message
		if err := c.msg.Nak(); err != nil {
			log.Println("Error naking message:", err)
		}

		return
	}
}

// Nak naks the message.
func (c *natsCommitter) Nak() error {
	return c.msg.Nak()
}

// Rollback rolls back the message.
func (c *natsCommitter) Rollback() error {
	return c.msg.Nak()
}
