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
	KVStore    *MockKVStore
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

	kvStoreMock := NewMockKVStore(ctrl)
	container.KVStore = kvStoreMock

	mocks := Mocks{
		Redis:      redisMock,
		SQL:        sqlMock,
		Clickhouse: clickhouseMock,
		Cassandra:  cassandraMock,
		Mongo:      mongoMock,
		KVStore:    kvStoreMock,
	}

	sqlMock.EXPECT().Close().AnyTimes()
	redisMock.EXPECT().Close().AnyTimes()

	mockMetrics := NewMockMetrics(ctrl)
	container.metricsManager = mockMetrics

	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_http_service_response", gomock.Any(), "path", gomock.Any(),
		"method", gomock.Any(), "status", fmt.Sprintf("%v", http.StatusInternalServerError)).AnyTimes()

	return container, mocks
}

type MockPubSub struct {
}

func (*MockPubSub) CreateTopic(_ context.Context, _ string) error {
	return nil
}

func (*MockPubSub) DeleteTopic(_ context.Context, _ string) error {
	return nil
}

func (*MockPubSub) Health() datasource.Health {
	return datasource.Health{}
}

func (*MockPubSub) Publish(_ context.Context, _ string, _ []byte) error {
	return nil
}

func (*MockPubSub) Subscribe(_ context.Context, _ string) (*pubsub.Message, error) {
	return nil, nil
}

func (*MockPubSub) Close() error { return nil }
