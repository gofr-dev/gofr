package container

import (
	"context"
	gosql "database/sql"
	"fmt"
	"log"
	"net/http"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gofr.dev/pkg/gofr/datasource/sql"

	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/file"
	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/logging"
)

type Mocks struct {
	Redis      *MockRedis
	SQL        *mockSQL
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
	queryWithArgs       []queryWithArgs
	expectedDialect     []dialect
	expectedHealthCheck []health
}

type queryWithArgs struct {
	queryText string
	arguments []interface{}
}

// mockSQL wraps go-mock-sql and expectations.
type mockSQL struct {
	sqlmock.Sqlmock
	*expectedQuery
}

// sqlMockDB wraps the go-mock-sql DB connection and expectations.
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

	sql2 := &mockSQL{sqlMock, &e}
	container.SQL = &sqlMockDB{mockDB, &e}

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
		SQL:        sql2,
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

func (m sqlMockDB) Select(_ context.Context, _ interface{}, query string, args ...interface{}) {
	if m.queryWithArgs == nil || len(m.queryWithArgs) == 0 {
		log.Fatalf("Did not expect any calls for Select with query: %s", query)
	}

	lastIndex := len(m.queryWithArgs) - 1
	expectedText := m.queryWithArgs[lastIndex].queryText
	expectedArgs := m.queryWithArgs[lastIndex].arguments

	if expectedText != query {
		log.Fatalf("expected query: %s, actual query: %s", query, expectedText)
	}

	if len(args) != len(expectedArgs) {
		log.Fatalf("expected %d args, actual %d", len(expectedArgs), len(args))
	}

	for i := range args {
		if args[i] != expectedArgs[i] {
			log.Fatalf("expected arg %d, actual arg %d", args[i], expectedArgs[i])
		}
	}

	m.queryWithArgs = m.queryWithArgs[:lastIndex]
}

func (m sqlMockDB) HealthCheck() *datasource.Health {
	if m.expectedHealthCheck == nil || len(m.expectedHealthCheck) == 0 {
		log.Fatal("Did not expect any mock calls for HealthCheck")
	}

	lastIndex := len(m.expectedHealthCheck) - 1
	expectedString := m.expectedHealthCheck[lastIndex]
	d := datasource.Health(expectedString)

	m.expectedHealthCheck = m.expectedHealthCheck[:lastIndex]

	return &d
}

func (m sqlMockDB) Dialect() string {
	if m.expectedDialect == nil || len(m.expectedDialect) == 0 {
		log.Fatal("Did not expect any mock calls for Dialect")
	}

	lastIndex := len(m.expectedDialect) - 1
	expectedString := m.expectedDialect[lastIndex]

	m.expectedDialect = m.expectedDialect[:lastIndex]

	return string(expectedString)
}

// ExpectSelect is no direct method for select in Select we expect the user to already send the
// populated data interface argument that can be used further in the process in the handler of functions.
func (m *mockSQL) ExpectSelect(_ context.Context, _ interface{}, query string, args ...interface{}) {
	emptyQueryWithArgs := make([]queryWithArgs, 0)

	if m.queryWithArgs == nil {
		m.queryWithArgs = emptyQueryWithArgs
	}

	qr := queryWithArgs{query, args}

	sliceQueryWithArgs := append(emptyQueryWithArgs, qr)
	m.queryWithArgs = append(sliceQueryWithArgs, m.queryWithArgs...)
}

func (m *mockSQL) ExpectHealthCheck() *health {
	emptyHealthCheckSlice := make([]health, 0)

	if m.expectedHealthCheck == nil {
		m.expectedHealthCheck = emptyHealthCheckSlice
	}

	hc := health{}

	healthCheckSlice := append(emptyHealthCheckSlice, hc)
	m.expectedHealthCheck = append(healthCheckSlice, m.expectedHealthCheck...)

	return &m.expectedHealthCheck[0]
}

func (d *health) WillReturnHealthCheck(dh datasource.Health) {
	*d = health(dh)
}

func (m *mockSQL) ExpectDialect() *dialect {
	emptyDialectSlice := make([]dialect, 0)

	if m.expectedDialect == nil {
		m.expectedDialect = emptyDialectSlice
	}

	d := dialect("")

	DialectSlice := append(emptyDialectSlice, d)
	m.expectedDialect = append(DialectSlice, m.expectedDialect...)

	return &m.expectedDialect[0]
}

func (m *mockSQL) NewResult(lastInsertID int64, rowsAffected int64) gosql.Result {
	return sqlmock.NewResult(lastInsertID, rowsAffected)
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
