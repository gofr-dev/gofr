package container

import (
	"context"
	"fmt"
	"github.com/DATA-DOG/go-sqlmock"
	"gofr.dev/pkg/gofr/datasource/sql"
	"net/http"
	"testing"

	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/file"
	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/logging"
)

type Mocks struct {
	Redis      *MockRedis
	SQL        mockSql
	Clickhouse *MockClickhouse
	Cassandra  *MockCassandra
	Mongo      *MockMongo
	KVStore    *MockKVStore
	File       *file.MockFileSystemProvider
}

type health datasource.Health

type dialect string

// expectedQuery stores the mock expectations till the method call.
type expectedQuery struct {
	query               string
	args                []interface{}
	expectedDialect     dialect
	expectedHealthCheck health
}

// mockSql wraps go-mock-sql and expectations
type mockSql struct {
	sqlmock.Sqlmock
	*expectedQuery
}

// sqlMockDB wraps the go-mock-sql DB connection and expectations
type sqlMockDB struct {
	*sql.DB
	*expectedQuery
}

type MockPubSub struct {
}

func NewMockContainer(t *testing.T) (*Container, Mocks) {
	t.Helper()

	container := &Container{}
	container.Logger = logging.NewLogger(logging.DEBUG)

	ctrl := gomock.NewController(t)

	mockDB, sqlMock, _ := sql.NewSQLMocks(t)
	// initialisation of expectations
	e := expectedQuery{}

	sql := mockSql{sqlMock, &e}
	container.SQL = sqlMockDB{mockDB, &e}

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

	fileStoreMock := file.NewMockFileSystemProvider(ctrl)
	container.File = fileStoreMock

	mocks := Mocks{
		Redis:      redisMock,
		SQL:        sql,
		Clickhouse: clickhouseMock,
		Cassandra:  cassandraMock,
		Mongo:      mongoMock,
		KVStore:    kvStoreMock,
		File:       fileStoreMock,
	}

	redisMock.EXPECT().Close().AnyTimes()

	mockMetrics := NewMockMetrics(ctrl)
	container.metricsManager = mockMetrics

	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_http_service_response", gomock.Any(), "path", gomock.Any(),
		"method", gomock.Any(), "status", fmt.Sprintf("%v", http.StatusInternalServerError)).AnyTimes()

	return container, mocks
}

func (m sqlMockDB) Select(ctx context.Context, data interface{}, query string, args ...interface{}) {
	if m.query != query {
		fmt.Errorf("expected query: %s, actual query: %s", query, m.query)
	}

	if len(args) != len(m.args) {
		fmt.Errorf("expected %d args, actual %d", len(m.args), len(args))
	}

	for i := range args {
		if args[i] != m.args[i] {
			fmt.Errorf("expected arg %d, actual arg %d", args[i], m.args[i])
		}
	}
}

func (m sqlMockDB) HealthCheck() *datasource.Health {
	d := datasource.Health(m.expectedHealthCheck)
	return &d
}

func (m sqlMockDB) Dialect() string {
	return string(m.expectedDialect)
}

// ExpectSelect is no direct method for select in Select we expect the user to already send the populated data interface
// argument that can be used further in the process in the handler of functions
func (m *mockSql) ExpectSelect(ctx context.Context, data interface{}, query string, args ...interface{}) {
	m.expectedQuery.query = query
	m.expectedQuery.args = args
	return
}

func (m *mockSql) ExpectHealthCheck() *health {
	return &m.expectedHealthCheck
}

func (d *health) WillReturnHealthCheck(dh datasource.Health) {
	*d = health(dh)
}

func (m *mockSql) ExpectDialect() *dialect {
	return &m.expectedDialect
}

func (d *dialect) WillReturnString(s string) {
	*d = dialect(s)
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
