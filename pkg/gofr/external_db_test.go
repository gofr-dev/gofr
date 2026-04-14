package gofr

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/datasource/file"
	"gofr.dev/pkg/gofr/testutil"
)

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
