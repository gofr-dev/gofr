package handlers

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

func TestAVROProducerHandler(t *testing.T) {
	app := gofr.New()
	m := mockPubSub{}
	app.PubSub = &m

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

	for _, tt := range tests {
		context.SetPathParams(map[string]string{
			"id": tt.id,
		})

		m.id = tt.id

		gotResp, gotErr := Producer(context)
		assert.Equal(t, gotErr, tt.expectedErr)
		assert.Equal(t, gotResp, tt.expectedResp)
	}
}

func TestAVROConsumerHandler(t *testing.T) {
	app := gofr.New()

	app.PubSub = &mockPubSub{}

	ctx := gofr.NewContext(nil, nil, app)

	_, err := Consumer(ctx)
	assert.Equal(t, nil, err)
}

func (m *mockPubSub) HealthCheck() types.Health {
	return types.Health{}
}

func (m *mockPubSub) IsSet() bool {
	return false
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
