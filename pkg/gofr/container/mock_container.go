package container

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/logging"
)

type Mocks struct {
	Redis      *MockRedis
	SQL        *MockDB
	Clickhouse *MockClickhouse
	Cassandra  *MockCassandra
	Mongo      *MockMongo
}

func NewMockContainer(t *testing.T) (*Container, Mocks) {
	t.Helper()

	container := &Container{}
	container.Logger = logging.NewLogger(logging.DEBUG)

	ctrl := gomock.NewController(t)

	sqlMock := NewMockDB(ctrl)
	container.SQL = sqlMock

	redisMock := NewMockRedis(ctrl)
	container.Redis = redisMock

	cassandraMock := NewMockCassandra(ctrl)
	container.Cassandra = cassandraMock

	clickhouseMock := NewMockClickhouse(ctrl)
	container.Clickhouse = clickhouseMock

	mongoMock := NewMockMongo(ctrl)
	container.Mongo = mongoMock

	mocks := Mocks{
		Redis:      redisMock,
		SQL:        sqlMock,
		Clickhouse: clickhouseMock,
		Cassandra:  cassandraMock,
		Mongo:      mongoMock,
	}

	mockMetrics := NewMockMetrics(ctrl)
	container.metricsManager = mockMetrics

	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_http_service_response", gomock.Any(), "path", gomock.Any(),
		"method", gomock.Any(), "status", fmt.Sprintf("%v", http.StatusInternalServerError)).AnyTimes()
	sqlMock.EXPECT().Close().AnyTimes()

	return container, mocks
}

type MockPubSub struct {
}

func (m *MockPubSub) CreateTopic(_ context.Context, _ string) error {
	return nil
}

func (m *MockPubSub) DeleteTopic(_ context.Context, _ string) error {
	return nil
}

func (m *MockPubSub) Health() datasource.Health {
	return datasource.Health{}
}

func (m *MockPubSub) Publish(_ context.Context, _ string, _ []byte) error {
	return nil
}

func (m *MockPubSub) Subscribe(_ context.Context, _ string) (*pubsub.Message, error) {
	return nil, nil
}

func (m *MockPubSub) Close(_ context.Context) error {
	return nil
}
