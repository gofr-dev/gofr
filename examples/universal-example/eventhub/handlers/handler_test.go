package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/datastore/pubsub"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/request"
	"gofr.dev/pkg/gofr/types"
)

type mockPubSub struct {
	id string
}

func TestProducerHandler(t *testing.T) {
	app := gofr.New()

	tests := []struct {
		name         string
		id           string
		expectedResp interface{}
		expectedErr  error
	}{
		{"error from publisher", "1", nil, errors.EntityNotFound{Entity: "", ID: "1"}},
		{"success", "123", nil, nil},
	}

	req := httptest.NewRequest(http.MethodGet, "http://dummy", nil)
	context := gofr.NewContext(nil, request.NewHTTPRequest(req), app)

	for _, tc := range tests {
		context.SetPathParams(map[string]string{
			"id": tc.id,
		})

		gotResp, gotErr := New(&mockPubSub{id: tc.id}).Producer(context)
		assert.Equal(t, tc.expectedErr, gotErr)
		assert.Equal(t, tc.expectedResp, gotResp)
	}
}

func TestConsumerHandler(t *testing.T) {
	app := gofr.New()

	ctx := gofr.NewContext(nil, nil, app)
	tests := []struct {
		id          string
		expectedErr error
	}{
		// Success Case
		{"", nil},
		// Failure Case
		{"1", errors.EntityNotFound{Entity: "", ID: "1"}},
	}

	for _, tc := range tests {
		_, gotErr := New(&mockPubSub{id: tc.id}).Consumer(ctx)
		assert.Equal(t, tc.expectedErr, gotErr)
	}
}

func (m *mockPubSub) PublishEventWithOptions(string, interface{}, map[string]string, *pubsub.PublishOptions) error {
	return nil
}

func (m *mockPubSub) PublishEvent(string, interface{}, map[string]string) error {
	if m.id == "1" {
		return errors.EntityNotFound{ID: "1"}
	}

	return nil
}

func (m *mockPubSub) Subscribe() (*pubsub.Message, error) {
	if m.id == "1" {
		return nil, errors.EntityNotFound{ID: "1"}
	}

	return &pubsub.Message{}, nil
}

func (m *mockPubSub) SubscribeWithCommit(pubsub.CommitFunc) (*pubsub.Message, error) {
	return nil, nil
}

func (m *mockPubSub) Bind([]byte, interface{}) error {
	return nil
}

func (m *mockPubSub) Ping() error {
	return nil
}

//nolint:gosimple //redundant `return` statement
func (m *mockPubSub) CommitOffset(pubsub.TopicPartition) {
	return
}

func (m *mockPubSub) HealthCheck() types.Health {
	return types.Health{}
}

func (m *mockPubSub) IsSet() bool {
	return false
}
