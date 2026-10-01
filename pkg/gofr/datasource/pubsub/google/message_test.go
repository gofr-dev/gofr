package google

import (
	"testing"

	gcPubSub "cloud.google.com/go/pubsub" //nolint:staticcheck // the package under test is still on pubsub v1; the v2 migration is a separate change
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	msg := new(gcPubSub.Message)

	out := newGoogleMessage(msg)

	assert.Equal(t, msg, out.msg)
}

func TestGoogleMessage_Commit(_ *testing.T) {
	msg := newGoogleMessage(&gcPubSub.Message{})

	msg.Commit()
}
