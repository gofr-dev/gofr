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
	Redis         *MockRedis
	SQL           *mockSQL
	Clickhouse    *MockClickhouse
	Cassandra     *MockCassandraWithContext
	Mongo         *MockMongo
	KVStore       *MockKVStore
	DGraph        *MockDgraph
	ArangoDB      *MockArangoDBProvider
	OpenTSDB      *MockOpenTSDB
	SurrealDB     *MockSurrealDB
	Elasticsearch *MockElasticsearch
	PubSub        *MockPubSubProvider
	File          *file.MockFileSystemProvider
	HTTPService   *service.MockHTTP
	Metrics       *MockMetrics
	Oracle        *MockOracleDB
	ScyllaDB      *MockScyllaDB
}

type options func(c *Container, ctrl *gomock.Controller) any

func WithMockHTTPService(httpServiceNames ...string) options { //nolint:revive // WithMockHTTPService returns an
	// exported type intentionally; options are internal and subject to change.
	return func(c *Container, ctrl *gomock.Controller) any {
		mockservice := service.NewMockHTTP(ctrl)
		for _, s := range httpServiceNames {
			c.Services[s] = mockservice
		}

		return mockservice
	}
}

// Helper function to initialize all container DB/service mocks.
func setContainerMocks(c *Container, ctrl *gomock.Controller) {
	c.Redis = NewMockRedis(ctrl)

	c.Cassandra = NewMockCassandraWithContext(ctrl)

	c.Clickhouse = NewMockClickhouse(ctrl)

	c.Oracle = NewMockOracleDB(ctrl)

	c.Mongo = NewMockMongo(ctrl)

	c.KVStore = NewMockKVStore(ctrl)

	c.File = file.NewMockFileSystemProvider(ctrl)

	c.DGraph = NewMockDgraph(ctrl)

	c.OpenTSDB = NewMockOpenTSDB(ctrl)

	c.ArangoDB = NewMockArangoDBProvider(ctrl)

	c.SurrealDB = NewMockSurrealDB(ctrl)

	c.Elasticsearch = NewMockElasticsearch(ctrl)

	c.ScyllaDB = NewMockScyllaDB(ctrl)

	c.PubSub = NewMockPubSubProvider(ctrl)
}

func NewMockContainer(t *testing.T, options ...options) (*Container, *Mocks) {
	t.Helper()

	container := &Container{}
	container.Logger = logging.NewLogger(logging.DEBUG)

	ctrl := gomock.NewController(t)

	mockDB, sqlMock, _ := sql.NewSQLMocks(t)
	// initialization of expectations.
	expectation := expectedQuery{}

	sqlMockWrapper := &mockSQL{sqlMock, &expectation}

	sqlDB := &sqlMockDB{mockDB, &expectation, logging.NewLogger(logging.DEBUG)}
	sqlDB.finish(t)

	container.SQL = sqlDB

	// Initialize all other mocks via helpers.
	setContainerMocks(container, ctrl)

	var httpMock *service.MockHTTP

	container.Services = make(map[string]service.HTTP)

	for _, option := range options {
		optionsAdded := option(container, ctrl)

		val, ok := optionsAdded.(*service.MockHTTP)
		if ok {
			httpMock = val
		}
	}

	// Setup expectations/mockmetrics
	container.Redis.(*MockRedis).EXPECT().Close().AnyTimes()

	mockMetrics := NewMockMetrics(ctrl)
	container.metricsManager = mockMetrics

	mocks := Mocks{
		Redis:         container.Redis.(*MockRedis),
		SQL:           sqlMockWrapper,
		Clickhouse:    container.Clickhouse.(*MockClickhouse),
		Cassandra:     container.Cassandra.(*MockCassandraWithContext),
		Mongo:         container.Mongo.(*MockMongo),
		KVStore:       container.KVStore.(*MockKVStore),
		File:          container.File.(*file.MockFileSystemProvider),
		HTTPService:   httpMock,
		DGraph:        container.DGraph.(*MockDgraph),
		OpenTSDB:      container.OpenTSDB.(*MockOpenTSDB),
		ArangoDB:      container.ArangoDB.(*MockArangoDBProvider),
		SurrealDB:     container.SurrealDB.(*MockSurrealDB),
		Elasticsearch: container.Elasticsearch.(*MockElasticsearch),
		PubSub:        container.PubSub.(*MockPubSubProvider),
		Metrics:       mockMetrics,
		Oracle:        container.Oracle.(*MockOracleDB),
		ScyllaDB:      container.ScyllaDB.(*MockScyllaDB),
	}

	// TODO: Remove this expectation from mock container (previous generalization) to the actual tests where their expectations are being set.
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	return container, &mocks
}

type MockPubSub struct {
}

func (*MockPubSub) Query(_ context.Context, _ string, _ ...any) ([]byte, error) {
	return nil, nil
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
