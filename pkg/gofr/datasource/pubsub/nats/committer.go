package nats

import (
	"log"

	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel/trace"
)

// natsCommitter implements the pubsub.Committer interface for Client messages.
type natsCommitter struct {
	msg  jetstream.Msg
	span trace.Span
}

// Commit commits the message and ends the subscribe span.
func (c *natsCommitter) Commit() {
	defer c.span.End()

	if err := c.msg.Ack(); err != nil {
		log.Println("Error committing message:", err)

		// nak the message
		if err := c.msg.Nak(); err != nil {
			log.Println("Error naking message:", err)
		}

		return
	}
}

// Nak naks the message and ends the subscribe span.
func (c *natsCommitter) Nak() error {
	defer c.span.End()

	return c.msg.Nak()
}

// Rollback rolls back the message and ends the subscribe span.
func (c *natsCommitter) Rollback() error {
	defer c.span.End()

	return c.msg.Nak()
}
