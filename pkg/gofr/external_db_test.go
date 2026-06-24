package gofr

import (
	"context"
	"database/sql/driver"
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/datasource/file"
	gofrSql "gofr.dev/pkg/gofr/datasource/sql"
	"gofr.dev/pkg/gofr/testutil"
)

var errNoTestConn = errors.New("test: connection not available")

// fakeSQLConnector is a test SQLConnector that returns preset values from Connect.
type fakeSQLConnector struct {
	connector driver.Connector
	cleanup   func() error
	err       error
}

func (f fakeSQLConnector) Connect() (driver.Connector, func() error, error) {
	return f.connector, f.cleanup, f.err
}

// failingConnector is a driver.Connector whose connections always fail, so AddSQLDB
// wraps it through NewSQLFromConnector (logging a failed ping) without needing a
// live database — enough to assert the wiring.
type failingConnector struct{}

func (failingConnector) Connect(context.Context) (driver.Conn, error) { return nil, errNoTestConn }
func (failingConnector) Driver() driver.Driver                        { return failingDriver{} }

type failingDriver struct{}

func (failingDriver) Open(string) (driver.Conn, error) { return nil, errNoTestConn }

func Test_tracerName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name     string
		ds       any
		expected string
	}{
		{"Mongo", container.NewMockMongo(ctrl), "gofr-mongo"},
		{"ArangoDB", container.NewMockArangoDB(ctrl), "gofr-arangodb"},
		{"Clickhouse", container.NewMockClickhouse(ctrl), "gofr-clickhouse"},
		{"Oracle", container.NewMockOracleDB(ctrl), "gofr-oracle"},
		{"Cassandra", container.NewMockCassandraWithContext(ctrl), "gofr-cassandra"},
		{"KVStore", container.NewMockKVStore(ctrl), "gofr-kvstore"},
		{"Solr", container.NewMockSolr(ctrl), "gofr-solr"},
		{"ScyllaDB", container.NewMockScyllaDB(ctrl), "gofr-scylladb"},
		{"SurrealDB", container.NewMockSurrealDB(ctrl), "gofr-surrealdb"},
		{"Elasticsearch", container.NewMockElasticsearch(ctrl), "gofr-elasticsearch"},
		{"Couchbase", container.NewMockCouchbase(ctrl), "gofr-couchbase"},
		{"InfluxDB", container.NewMockInfluxDB(ctrl), "gofr-influxdb"},
		{"DBResolver", &fakeDBResolverProvider{}, "gofr-dbresolver"},
		{"Unknown", "not-a-datasource", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tracerName(tt.ds))
		})
	}
}

func Test_instrumentDatasource(t *testing.T) {
	testutil.NewServerConfigs(t)

	app := New()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// MockMongoProvider implements UseLogger/UseMetrics/UseTracer/Connect via provider interface
	mock := container.NewMockMongoProvider(ctrl)
	mock.EXPECT().UseLogger(app.Logger())
	mock.EXPECT().UseMetrics(app.Metrics())
	mock.EXPECT().UseTracer(otel.GetTracerProvider().Tracer("gofr-mongo"))
	mock.EXPECT().Connect()

	app.instrumentDatasource(mock)
}

func Test_instrumentDatasource_PartialImplementation(t *testing.T) {
	testutil.NewServerConfigs(t)

	app := New()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// FileSystemProvider has UseLogger/UseMetrics/Connect but no UseTracer
	mock := file.NewMockFileSystemProvider(ctrl)
	mock.EXPECT().UseLogger(app.Logger())
	mock.EXPECT().UseMetrics(app.Metrics())
	mock.EXPECT().Connect()

	// Should not panic — UseTracer is simply skipped
	app.instrumentDatasource(mock)
}

func TestApp_AddSQLDB(t *testing.T) {
	testutil.NewServerConfigs(t)

	app := New()

	// A pluggable SQL provider (here a fake connector, as Cloud SQL supplies) must be
	// opened through the standard SQL wrapper and installed as the container's SQL.
	app.AddSQLDB(fakeSQLConnector{connector: failingConnector{}})
	t.Cleanup(func() { _ = app.container.SQL.Close() })

	require.NotNil(t, app.container.SQL)

	_, ok := app.container.SQL.(*gofrSql.DB)
	assert.True(t, ok, "AddSQLDB must wrap the connector in the standard SQL datasource")
}

// TestApp_AddSQLDB_ClosesExisting verifies AddSQLDB closes the env-configured SQL
// connection that container.Create opens eagerly, before swapping in the connector —
// otherwise its pool and background retry/metrics goroutines leak.
func TestApp_AddSQLDB_ClosesExisting(t *testing.T) {
	testutil.NewServerConfigs(t)

	app := New()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	existing := container.NewMockDB(ctrl)
	existing.EXPECT().Close().Return(nil)
	app.container.SQL = existing

	app.AddSQLDB(fakeSQLConnector{connector: failingConnector{}})
	t.Cleanup(func() { _ = app.container.SQL.Close() })

	require.NotNil(t, app.container.SQL)
	assert.NotEqual(t, existing, app.container.SQL)
}

// TestApp_AddSQLDB_NilExisting verifies AddSQLDB is safe when the container holds a
// typed-nil SQL datasource (container.Create leaves one when DB_DIALECT is unset) —
// it must not attempt to Close it.
func TestApp_AddSQLDB_NilExisting(t *testing.T) {
	testutil.NewServerConfigs(t)

	app := New()

	var nilDB *gofrSql.DB

	app.container.SQL = nilDB // typed-nil interface value

	assert.NotPanics(t, func() { app.AddSQLDB(fakeSQLConnector{connector: failingConnector{}}) })
	t.Cleanup(func() { _ = app.container.SQL.Close() })

	require.NotNil(t, app.container.SQL)
}

// TestApp_AddSQLDB_DefersWhenNoConnector verifies that when the provider returns a
// nil connector (e.g. IAM auth not requested), AddSQLDB leaves the standard
// env-configured SQL connection in place rather than tearing it down.
func TestApp_AddSQLDB_DefersWhenNoConnector(t *testing.T) {
	testutil.NewServerConfigs(t)

	app := New()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// No Close expectation: deferring must not close the existing connection.
	existing := container.NewMockDB(ctrl)
	app.container.SQL = existing

	app.AddSQLDB(fakeSQLConnector{}) // nil connector, nil error

	assert.Equal(t, existing, app.container.SQL, "a nil connector must keep the env SQL connection")
}

// TestApp_AddSQLDB_ClosesExistingOnError verifies that a connector setup error tears
// down the env-configured SQL (which on the IAM path is dialing the wrong host and
// would retry forever) and leaves SQL unset, so health reports down instead of using
// a broken connection.
func TestApp_AddSQLDB_ClosesExistingOnError(t *testing.T) {
	testutil.NewServerConfigs(t)

	app := New()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	existing := container.NewMockDB(ctrl)
	existing.EXPECT().Close().Return(nil)
	app.container.SQL = existing

	app.AddSQLDB(fakeSQLConnector{err: errNoTestConn})

	assert.Nil(t, app.container.SQL)
}

func TestApp_AddMongo(t *testing.T) {
	testutil.NewServerConfigs(t)

	app := New()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := container.NewMockMongoProvider(ctrl)
	mock.EXPECT().UseLogger(app.Logger())
	mock.EXPECT().UseMetrics(app.Metrics())
	mock.EXPECT().UseTracer(gomock.Any())
	mock.EXPECT().Connect()

	app.AddMongo(mock)

	assert.Equal(t, mock, app.container.Mongo)
}

func TestApp_AddArangoDB(t *testing.T) {
	port := testutil.GetFreePort(t)
	t.Setenv("METRICS_PORT", strconv.Itoa(port))

	app := New()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := container.NewMockArangoDBProvider(ctrl)
	mock.EXPECT().UseLogger(app.Logger())
	mock.EXPECT().UseMetrics(app.Metrics())
	mock.EXPECT().UseTracer(otel.GetTracerProvider().Tracer("gofr-arangodb"))
	mock.EXPECT().Connect()

	app.AddArangoDB(mock)

	assert.Equal(t, mock, app.container.ArangoDB)
}

func TestApp_AddClickhouse(t *testing.T) {
	testutil.NewServerConfigs(t)

	app := New()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := container.NewMockClickhouseProvider(ctrl)
	mock.EXPECT().UseLogger(app.Logger())
	mock.EXPECT().UseMetrics(app.Metrics())
	mock.EXPECT().UseTracer(otel.GetTracerProvider().Tracer("gofr-clickhouse"))
	mock.EXPECT().Connect()

	app.AddClickhouse(mock)

	assert.Equal(t, mock, app.container.Clickhouse)
}

func TestApp_AddCassandra(t *testing.T) {
	testutil.NewServerConfigs(t)

	app := New()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := container.NewMockCassandraProvider(ctrl)
	mock.EXPECT().UseLogger(app.Logger())
	mock.EXPECT().UseMetrics(app.Metrics())
	mock.EXPECT().UseTracer(otel.GetTracerProvider().Tracer("gofr-cassandra"))
	mock.EXPECT().Connect()

	app.AddCassandra(mock)

	assert.Equal(t, mock, app.container.Cassandra)
}

func TestApp_AddOracle(t *testing.T) {
	testutil.NewServerConfigs(t)

	app := New()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := container.NewMockOracleProvider(ctrl)
	mock.EXPECT().UseLogger(app.Logger())
	mock.EXPECT().UseMetrics(app.Metrics())
	mock.EXPECT().UseTracer(otel.GetTracerProvider().Tracer("gofr-oracle"))
	mock.EXPECT().Connect()

	app.AddOracle(mock)

	assert.Equal(t, mock, app.container.Oracle)
}

func TestApp_AddKVStore(t *testing.T) {
	testutil.NewServerConfigs(t)

	app := New()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := container.NewMockKVStoreProvider(ctrl)
	mock.EXPECT().UseLogger(app.Logger())
	mock.EXPECT().UseMetrics(app.Metrics())
	mock.EXPECT().UseTracer(otel.GetTracerProvider().Tracer("gofr-kvstore"))
	mock.EXPECT().Connect()

	app.AddKVStore(mock)

	assert.Equal(t, mock, app.container.KVStore)
}

func TestApp_AddSolr(t *testing.T) {
	testutil.NewServerConfigs(t)

	app := New()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := container.NewMockSolrProvider(ctrl)
	mock.EXPECT().UseLogger(app.Logger())
	mock.EXPECT().UseMetrics(app.Metrics())
	mock.EXPECT().UseTracer(gomock.Any())
	mock.EXPECT().Connect()

	app.AddSolr(mock)

	assert.Equal(t, mock, app.container.Solr)
}

func TestApp_AddFTP(t *testing.T) {
	testutil.NewServerConfigs(t)

	app := New()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := file.NewMockFileSystemProvider(ctrl)
	mock.EXPECT().UseLogger(app.Logger())
	mock.EXPECT().UseMetrics(app.Metrics())
	mock.EXPECT().Connect()

	app.AddFTP(mock)

	assert.Equal(t, mock, app.container.File)
}

func TestApp_AddFileStore(t *testing.T) {
	testutil.NewServerConfigs(t)

	app := New()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := file.NewMockFileSystemProvider(ctrl)
	mock.EXPECT().UseLogger(app.Logger())
	mock.EXPECT().UseMetrics(app.Metrics())
	mock.EXPECT().Connect()

	app.AddFileStore(mock)

	assert.Equal(t, mock, app.container.File)
}

// fakeDBResolverProvider satisfies container.DBResolverProvider without importing
// the dbresolver package (which would create an import cycle through pkg/gofr).
type fakeDBResolverProvider struct {
	logger     any
	metrics    any
	tracer     any
	connected  bool
	resolverDB container.DB
}

func (f *fakeDBResolverProvider) UseLogger(l any)           { f.logger = l }
func (f *fakeDBResolverProvider) UseMetrics(m any)          { f.metrics = m }
func (f *fakeDBResolverProvider) UseTracer(t any)           { f.tracer = t }
func (f *fakeDBResolverProvider) Connect()                  { f.connected = true }
func (f *fakeDBResolverProvider) GetResolver() container.DB { return f.resolverDB }

func TestApp_AddDBResolver_WiresTracing(t *testing.T) {
	testutil.NewServerConfigs(t)

	app := New()

	db, _, _ := gofrSql.NewSQLMocksWithConfig(t, &gofrSql.DBConfig{Dialect: "mysql"})
	defer db.Close()

	app.container.SQL = db

	resolved := &fakeDBResolverProvider{resolverDB: db}

	app.AddDBResolver(resolved)

	assert.Equal(t, app.Logger(), resolved.logger)
	assert.Equal(t, app.Metrics(), resolved.metrics)
	assert.Equal(t, otel.GetTracerProvider().Tracer("gofr-dbresolver"), resolved.tracer)
	assert.True(t, resolved.connected)
	assert.Equal(t, container.DB(db), app.container.SQL)
}

func TestApp_AddOpenTSDB(t *testing.T) {
	testutil.NewServerConfigs(t)

	app := New()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := container.NewMockOpenTSDBProvider(ctrl)
	mock.EXPECT().UseLogger(app.Logger())
	mock.EXPECT().UseMetrics(app.Metrics())
	mock.EXPECT().UseTracer(gomock.Any())
	mock.EXPECT().Connect()

	app.AddOpenTSDB(mock)

	assert.Equal(t, mock, app.container.OpenTSDB)
}

func TestApp_AddScyllaDB(t *testing.T) {
	testutil.NewServerConfigs(t)

	app := New()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := container.NewMockScyllaDBProvider(ctrl)
	mock.EXPECT().UseLogger(app.Logger())
	mock.EXPECT().UseMetrics(app.Metrics())
	mock.EXPECT().UseTracer(gomock.Any())
	mock.EXPECT().Connect()

	app.AddScyllaDB(mock)

	assert.Equal(t, mock, app.container.ScyllaDB)
}
