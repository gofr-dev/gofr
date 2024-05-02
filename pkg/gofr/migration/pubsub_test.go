package migration

import (
	"context"
	"testing"
)

func TestPubSub_CreateTopic(t *testing.T) {
	ps := newPubSub(&mockPubsub{})

	ctx := context.Background()
	topicName := "testTopic"

	err := ps.CreateTopic(ctx, topicName)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPubSub_DeleteTopic(t *testing.T) {
	ps := newPubSub(&mockPubsub{})

	ctx := context.Background()
	topicName := "testTopic"

	err := ps.DeleteTopic(ctx, topicName)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

type mockPubsub struct {
}

// CreateTopic mocks the CreateTopic method.
func (m *mockPubsub) CreateTopic(context.Context, string) error {
	return nil
}

// DeleteTopic mocks the DeleteTopic method.
func (m *mockPubsub) DeleteTopic(context.Context, string) error {
	return nil
}
