package nats

import (
	"log"

	"github.com/nats-io/nats.go/jetstream"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

// createCommitter returns a Committer for the given NATS message.
func createCommitter(msg jetstream.Msg) pubsub.Committer {
	return &natsCommitter{msg: msg}
}

// natsCommitter implements the pubsub.Committer interface for NATS messages.
type natsCommitter struct {
	msg jetstream.Msg
}

// Commit commits the message.
func (c *natsCommitter) Commit() {
	log.Println("Committing message")
	err := c.msg.Ack()
	if err != nil {
		err := c.msg.Nak()
		if err != nil {
			log.Println("Error committing message:", err)

			return
		}

		log.Println("Error committing message:", err)

		return
	}
}

func (c *natsCommitter) Nak() error {
	return c.msg.Nak()
}

// Rollback rolls back the message.
func (c *natsCommitter) Rollback() error {
	return c.msg.Nak()
}
