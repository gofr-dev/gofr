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
	Couchbase     *MockCouchbase
	File          *file.MockFileSystemProvider
	HTTPService   *service.MockHTTP
	Metrics       *MockMetrics
	ScyllaDB      *MockScyllaDB
}

func newMocks(t *testing.T, ctrl *gomock.Controller) (*Mocks, *sqlMockDB) {
	t.Helper()
	mockDB, sqlMock, _ := sql.NewSQLMocks(t)
	expectation := expectedQuery{}
	sqlMockWrapper := &mockSQL{sqlMock, &expectation}
	sqlDB := &sqlMockDB{mockDB, &expectation, logging.NewLogger(logging.DEBUG)}
	sqlDB.finish(t)

	return &Mocks{
		Redis:         NewMockRedis(ctrl),
		SQL:           sqlMockWrapper,
		Clickhouse:    NewMockClickhouse(ctrl),
		Cassandra:     NewMockCassandraWithContext(ctrl),
		Mongo:         NewMockMongo(ctrl),
		KVStore:       NewMockKVStore(ctrl),
		DGraph:        NewMockDgraph(ctrl),
		ArangoDB:      NewMockArangoDBProvider(ctrl),
		OpenTSDB:      NewMockOpenTSDB(ctrl),
		SurrealDB:     NewMockSurrealDB(ctrl),
		Elasticsearch: NewMockElasticsearch(ctrl),
		PubSub:        NewMockPubSubProvider(ctrl),
		Couchbase:     NewMockCouchbase(ctrl),
		File:          file.NewMockFileSystemProvider(ctrl),
		Metrics:       NewMockMetrics(ctrl),
		ScyllaDB:      NewMockScyllaDB(ctrl),
	}, sqlDB
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

func NewMockContainer(t *testing.T, options ...options) (*Container, *Mocks) {
	t.Helper()

	container := &Container{}
	container.Logger = logging.NewLogger(logging.DEBUG)

	ctrl := gomock.NewController(t)

	mocks, sqlDB := newMocks(t, ctrl)
	container.SQL = sqlDB

	container.Redis = mocks.Redis
	mocks.Redis.EXPECT().Close().AnyTimes()

	container.Cassandra = mocks.Cassandra

	container.Clickhouse = mocks.Clickhouse

	container.Mongo = mocks.Mongo

	container.KVStore = mocks.KVStore

	container.File = mocks.File

	container.DGraph = mocks.DGraph

	container.OpenTSDB = mocks.OpenTSDB

	container.ArangoDB = mocks.ArangoDB

	container.SurrealDB = mocks.SurrealDB

	container.Elasticsearch = mocks.Elasticsearch

	container.PubSub = mocks.PubSub

	container.ScyllaDB = mocks.ScyllaDB

	container.PubSub = mocks.PubSub

	container.Couchbase = mocks.Couchbase

	container.Services = make(map[string]service.HTTP)

	container.metricsManager = mocks.Metrics
	// TODO: Remove this expectation from mock container (previous generalization) to the actual tests where their expectations are being set.
	mocks.Metrics.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	for _, option := range options {
		optionsAdded := option(container, ctrl)

		val, ok := optionsAdded.(*service.MockHTTP)
		if ok {
			mocks.HTTPService = val
		}
	}

	return container, mocks
}

type MockPubSub struct{}

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
