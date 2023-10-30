package handlers

import (
	"gofr.dev/pkg/datastore/pubsub"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr/types"
)

type mockPubSub struct {
	id string
}

func (m *mockPubSub) CommitOffset(pubsub.TopicPartition) {
}

func (m *mockPubSub) PublishEventWithOptions(string, interface{}, map[string]string, *pubsub.PublishOptions) error {
	if m.id == "1" {
		return errors.EntityNotFound{ID: "1"}
	}

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
	return &pubsub.Message{}, nil
}

func (m *mockPubSub) Bind([]byte, interface{}) error {
	return nil
}

func (m *mockPubSub) Ping() error {
	return nil
}

func (m *mockPubSub) HealthCheck() types.Health {
	return types.Health{}
}

func (m *mockPubSub) IsSet() bool {
	return true
}

//func TestProducerHandler(t *testing.T) {
//	app := gofr.New()
//	m := mockPubSub{}
//	app.PubSub = &m
//
//	tests := []struct {
//		desc string
//		id   string
//		resp interface{}
//		err  error
//	}{
//		{"error from publisher", "1", nil, errors.EntityNotFound{ID: "1"}},
//		{"success", "123", nil, nil},
//	}
//
//	req := httptest.NewRequest(http.MethodGet, "http://dummy", nil)
//	ctx := gofr.NewContext(nil, request.NewHTTPRequest(req), app)
//
//	for i, tc := range tests {
//		ctx.SetPathParams(map[string]string{
//			"id": tc.id,
//		})
//
//		m.id = tc.id
//		resp, err := Producer(ctx)
//
//		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)
//
//		assert.Equal(t, tc.resp, resp, "TEST[%d], failed.\n%s", i, tc.desc)
//	}
//}
//
//func TestConsumerHandler(t *testing.T) {
//	app := gofr.New()
//	app.PubSub = &mockPubSub{}
//
//	ctx := gofr.NewContext(nil, nil, app)
//	_, err := Consumer(ctx)
//
//	assert.Equal(t, nil, err)
//}
