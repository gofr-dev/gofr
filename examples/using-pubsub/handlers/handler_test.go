package handlers

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/datastore/pubsub"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/metrics"
	"gofr.dev/pkg/gofr/request"
	"gofr.dev/pkg/gofr/types"
)

type mockPubSub struct {
	id string
}

func (m *mockPubSub) SubscribeWithCommit(commitFunc pubsub.CommitFunc) (*pubsub.Message, error) {
	pubsubMessage := &pubsub.Message{
		Offset: 1,
		Topic:  "test-topic",
	}

	count := 0

	for {
		pubsubMessage.Offset = int64(count + 1)

		_, isContinue := commitFunc(pubsubMessage)
		if !isContinue {
			break
		}
	}

	if m.id == "error" {
		return nil, &errors.Response{Reason: "test-error"}
	}

	return &pubsub.Message{}, nil
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

func (m *mockPubSub) Bind([]byte, interface{}) error {
	return nil
}

func (m *mockPubSub) Ping() error {
	return nil
}

func (m *mockPubSub) IsSet() bool {
	return true
}

func (m *mockPubSub) HealthCheck() types.Health {
	return types.Health{}
}

func (m *mockPubSub) CommitOffset(pubsub.TopicPartition) {
}

func TestProducerHandler(t *testing.T) {
	app := gofr.New()
	mockMetric := metrics.NewMockMetric(gomock.NewController(t))
	app.Metric = mockMetric

	mockMetric.EXPECT().ObserveHistogram(PublishEventHistogram, gomock.Any()).Return(nil)

	m := mockPubSub{}
	app.PubSub = &m

	tests := []struct {
		name    string
		id      string
		want    interface{}
		wantErr bool
	}{
		{"error from publisher", "1", nil, true},
		{"success", "123", nil, false},
	}

	req := httptest.NewRequest(http.MethodGet, "http://dummy", nil)
	context := gofr.NewContext(nil, request.NewHTTPRequest(req), app)

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			context.SetPathParams(map[string]string{
				"id": tt.id,
			})

			m.id = tt.id

			got, err := Producer(context)
			if (err != nil) != tt.wantErr {
				t.Errorf("Producer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Producer() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConsumerHandler(t *testing.T) {
	app := gofr.New()
	mockMetric := metrics.NewMockMetric(gomock.NewController(t))
	app.Metric = mockMetric

	mockMetric.EXPECT().ObserveSummary(ConsumeEventSummary, gomock.Any()).Return(nil)

	app.PubSub = &mockPubSub{}

	ctx := gofr.NewContext(nil, nil, app)

	_, err := Consumer(ctx)
	if err != nil {
		t.Errorf("Consumer() error = %v, wantErr %v", err, nil)
		return
	}
}

func TestConsumerWithCommitHandler(t *testing.T) {
	app := gofr.New()

	tests := []struct {
		desc   string
		pubSub pubsub.PublisherSubscriber
		err    error
	}{
		{"success consuming messages", &mockPubSub{}, nil},
		{"error from pubsub subscribe", &mockPubSub{"error"}, &errors.Response{Reason: "test-error"}},
	}

	for i, tc := range tests {
		app.PubSub = tc.pubSub
		ctx := gofr.NewContext(nil, nil, app)

		_, err := ConsumerWithCommit(ctx)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}
