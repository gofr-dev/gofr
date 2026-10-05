package google

import (
	gcPubSub "cloud.google.com/go/pubsub" //nolint:staticcheck // pubsub v1 is deprecated in favor of v2; the migration is a separate change
)

type googleMessage struct {
	msg *gcPubSub.Message
}

func newGoogleMessage(msg *gcPubSub.Message) *googleMessage {
	return &googleMessage{msg: msg}
}

func (gm *googleMessage) Commit() {
	gm.msg.Ack()
}
