package container

import (
	"context"
	"testing"

	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/file"
	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/datasource/sql"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/service"
)

type Mocks struct {
	Redis       *MockRedis
	SQL         *mockSQL
	Clickhouse  *MockClickhouse
	Cassandra   *MockCassandraWithContext
	Mongo       *MockMongo
	KVStore     *MockKVStore
	DGraph      *MockDgraph
	ArangoDB    *MockArangoDBProvider
	OpenTSDB    *MockOpenTSDBProvider
	SurrealDB   *MockSurrealDB
	File        *file.MockFileSystemProvider
	HTTPService *service.MockHTTP
	Metrics     *MockMetrics
}

type options func(c *Container, ctrl *gomock.Controller) any

//nolint:revive // Because user should not access the options, and we might change it to an interface in the future.
func WithMockHTTPService(httpServiceNames ...string) options {
	return func(c *Container, ctrl *gomock.Controller) any {
		mockservice := service.NewMockHTTP(ctrl)
		for _, s := range httpServiceNames {
			c.Services[s] = mockservice
		}

		return mockservice
	}
}

func NewMockContainer(t *testing.T, options ...options) (*Container, *Mocks) {
	t.Helper()

	container := &Container{}
	container.Logger = logging.NewLogger(logging.DEBUG)

	ctrl := gomock.NewController(t)

	mockDB, sqlMock, _ := sql.NewSQLMocks(t)
	// initialisation of expectations
	expectation := expectedQuery{}

	sqlMockWrapper := &mockSQL{sqlMock, &expectation}

	sqlDB := &sqlMockDB{mockDB, &expectation, logging.NewLogger(logging.DEBUG)}
	sqlDB.finish(t)

	container.SQL = sqlDB

	redisMock := NewMockRedis(ctrl)
	container.Redis = redisMock

	cassandraMock := NewMockCassandraWithContext(ctrl)
	container.Cassandra = cassandraMock

	clickhouseMock := NewMockClickhouse(ctrl)
	container.Clickhouse = clickhouseMock

	mongoMock := NewMockMongo(ctrl)
	container.Mongo = mongoMock

	kvStoreMock := NewMockKVStore(ctrl)
	container.KVStore = kvStoreMock

	fileStoreMock := file.NewMockFileSystemProvider(ctrl)
	container.File = fileStoreMock

	dgraphMock := NewMockDgraph(ctrl)
	container.DGraph = dgraphMock

	opentsdbMock := NewMockOpenTSDBProvider(ctrl)
	container.OpenTSDB = opentsdbMock

	arangoMock := NewMockArangoDBProvider(ctrl)
	container.ArangoDB = arangoMock

	surrealMock := NewMockSurrealDB(ctrl)
	container.SurrealDB = surrealMock

	var httpMock *service.MockHTTP

	container.Services = make(map[string]service.HTTP)

	for _, option := range options {
		optionsAdded := option(container, ctrl)

		val, ok := optionsAdded.(*service.MockHTTP)
		if ok {
			httpMock = val
		}
	}

	redisMock.EXPECT().Close().AnyTimes()

	mockMetrics := NewMockMetrics(ctrl)
	container.metricsManager = mockMetrics

	mocks := Mocks{
		Redis:       redisMock,
		SQL:         sqlMockWrapper,
		Clickhouse:  clickhouseMock,
		Cassandra:   cassandraMock,
		Mongo:       mongoMock,
		KVStore:     kvStoreMock,
		File:        fileStoreMock,
		HTTPService: httpMock,
		DGraph:      dgraphMock,
		OpenTSDB:    opentsdbMock,
		ArangoDB:    arangoMock,
		SurrealDB:   surrealMock,
		Metrics:     mockMetrics,
	}

	// TODO: Remove this expectation from mock container (previous generalisation) to the actual tests where their expectations are being set.
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	return container, &mocks
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
