package migration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"gofr.dev/pkg/gofr/testutil"
)

func TestPubSub_CreateTopic(t *testing.T) {
	ps := newPubSub(&mockPubsub{})

	ctx := context.Background()
	topicName := "testTopic"

	err := ps.CreateTopic(ctx, topicName)

	assert.Nil(t, err)
}

func TestPubSub_DeleteTopic(t *testing.T) {
	ps := newPubSub(&mockPubsub{})

	ctx := context.Background()
	topicName := "testTopic"

	err := ps.DeleteTopic(ctx, topicName)

	assert.Nil(t, err)
}

func TestPubSub_CreateTopicFailed(t *testing.T) {
	ps := newPubSub(&mockPubsub{})

	ctx := context.Background()
	topicName := "failure"

	err := ps.CreateTopic(ctx, topicName)

	assert.NotNil(t, err)
}

func TestPubSub_DeleteTopicFailed(t *testing.T) {
	ps := newPubSub(&mockPubsub{})

	ctx := context.Background()
	topicName := "failure"

	err := ps.DeleteTopic(ctx, topicName)

	assert.NotNil(t, err)
}

type mockPubsub struct {
}

// CreateTopic mocks the CreateTopic method.
func (m *mockPubsub) CreateTopic(_ context.Context, topic string) error {
	if topic == "testTopic" {
		return nil
	}

	return testutil.CustomError{ErrorMessage: "topic creation failed"}
}

// DeleteTopic mocks the DeleteTopic method.
func (m *mockPubsub) DeleteTopic(_ context.Context, topic string) error {
	if topic == "testTopic" {
		return nil
	}

	return testutil.CustomError{ErrorMessage: "topic deletion failed"}
}
