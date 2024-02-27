package google

import (
	gcPubSub "cloud.google.com/go/pubsub"
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
