package gofr

import (
	"go.opentelemetry.io/otel"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/datasource/file"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

// tracerName returns the OpenTelemetry tracer name for a datasource.
// Returns empty string if tracing is not applicable for the type.
//
//nolint:cyclop // complexity from datasource count, not logic
func tracerName(ds any) string {
	switch ds.(type) {
	case container.Mongo:
		return "gofr-mongo"
	case container.ArangoDB:
		return "gofr-arangodb"
	case container.Clickhouse:
		return "gofr-clickhouse"
	case container.OracleDB:
		return "gofr-oracle"
	case container.CassandraWithContext:
		return "gofr-cassandra"
	case container.KVStore:
		return "gofr-kvstore"
	case container.Solr:
		return "gofr-solr"
	case container.Dgraph:
		return "gofr-dgraph"
	case container.OpenTSDB:
		return "gofr-opentsdb"
	case container.ScyllaDB:
		return "gofr-scylladb"
	case container.SurrealDB:
		return "gofr-surrealdb"
	case container.Elasticsearch:
		return "gofr-elasticsearch"
	case container.Couchbase:
		return "gofr-couchbase"
	case container.InfluxDB:
		return "gofr-influxdb"
	default:
		return ""
	}
}

// instrumentDatasource sets up logging, metrics, tracing, and connection for a datasource
// using duck typing. Each datasource only needs to implement the methods it supports.
func (a *App) instrumentDatasource(ds any) {
	if l, ok := ds.(interface{ UseLogger(any) }); ok {
		l.UseLogger(a.Logger())
	}

	if m, ok := ds.(interface{ UseMetrics(any) }); ok {
		m.UseMetrics(a.Metrics())
	}

	if name := tracerName(ds); name != "" {
		if t, ok := ds.(interface{ UseTracer(any) }); ok {
			t.UseTracer(otel.GetTracerProvider().Tracer(name))
		}
	}

	if c, ok := ds.(interface{ Connect() }); ok {
		c.Connect()
	}
}

// AddMongo sets the Mongo datasource in the app's container.
func (a *App) AddMongo(db container.Mongo) {
	a.instrumentDatasource(db)
	a.container.Mongo = db
}

// AddFTP sets the FTP datasource in the app's container.
// Deprecated: Use the AddFileStore method instead.
func (a *App) AddFTP(fs file.FileSystemProvider) {
	a.instrumentDatasource(fs)
	a.container.File = fs
}

// AddPubSub sets the PubSub client in the app's container.
func (a *App) AddPubSub(ps pubsub.Client) {
	a.instrumentDatasource(ps)
	a.container.PubSub = ps
}

// AddFileStore sets the FTP, SFTP, S3, GCS, or Azure File Storage datasource in the app's container.
func (a *App) AddFileStore(fs file.FileSystemProvider) {
	a.instrumentDatasource(fs)
	a.container.File = fs
}

// AddClickhouse initializes the clickhouse client.
// Official implementation is available in the package: gofr.dev/pkg/gofr/datasource/clickhouse.
func (a *App) AddClickhouse(db container.Clickhouse) {
	a.instrumentDatasource(db)
	a.container.Clickhouse = db
}

// AddOracle initializes the OracleDB client.
// Official implementation is available in the package: gofr.dev/pkg/gofr/datasource/oracle.
func (a *App) AddOracle(db container.OracleDB) {
	a.instrumentDatasource(db)
	a.container.Oracle = db
}

// UseMongo sets the Mongo datasource in the app's container.
// Deprecated: Use the AddMongo method instead.
func (a *App) UseMongo(db container.Mongo) {
	a.container.Mongo = db
}

// AddCassandra sets the Cassandra datasource in the app's container.
func (a *App) AddCassandra(db container.CassandraWithContext) {
	a.instrumentDatasource(db)
	a.container.Cassandra = db
}

// AddKVStore sets the KV-Store datasource in the app's container.
func (a *App) AddKVStore(db container.KVStore) {
	a.instrumentDatasource(db)
	a.container.KVStore = db
}

// AddSolr sets the Solr datasource in the app's container.
func (a *App) AddSolr(db container.Solr) {
	a.instrumentDatasource(db)
	a.container.Solr = db
}

// AddDgraph sets the Dgraph datasource in the app's container.
func (a *App) AddDgraph(db container.Dgraph) {
	a.instrumentDatasource(db)
	a.container.DGraph = db
}

// AddOpenTSDB sets the OpenTSDB datasource in the app's container.
func (a *App) AddOpenTSDB(db container.OpenTSDB) {
	a.instrumentDatasource(db)
	a.container.OpenTSDB = db
}

// AddScyllaDB sets the ScyllaDB datasource in the app's container.
func (a *App) AddScyllaDB(db container.ScyllaDB) {
	a.instrumentDatasource(db)
	a.container.ScyllaDB = db
}

// AddArangoDB sets the ArangoDB datasource in the app's container.
func (a *App) AddArangoDB(db container.ArangoDB) {
	a.instrumentDatasource(db)
	a.container.ArangoDB = db
}

// AddSurrealDB sets the SurrealDB datasource in the app's container.
func (a *App) AddSurrealDB(db container.SurrealDB) {
	a.instrumentDatasource(db)
	a.container.SurrealDB = db
}

// AddElasticsearch sets the Elasticsearch datasource in the app's container.
func (a *App) AddElasticsearch(db container.Elasticsearch) {
	a.instrumentDatasource(db)
	a.container.Elasticsearch = db
}

// AddCouchbase sets the Couchbase datasource in the app's container.
func (a *App) AddCouchbase(db container.Couchbase) {
	a.instrumentDatasource(db)
	a.container.Couchbase = db
}

// AddDBResolver sets up database resolver with read/write splitting.
func (a *App) AddDBResolver(resolver container.DBResolverProvider) {
	if a.container.SQL == nil {
		a.Logger().Fatal("Primary SQL connection must be configured before adding DBResolver")
		return
	}

	a.instrumentDatasource(resolver)
	a.container.SQL = resolver.GetResolver()

	a.Logger().Logf("DB Resolver initialized successfully")
}

// AddInfluxDB sets the InfluxDB datasource in the app's container.
func (a *App) AddInfluxDB(db container.InfluxDB) {
	a.instrumentDatasource(db)
	a.container.InfluxDB = db
}

// GetSQL returns the SQL datasource from the container.
func (a *App) GetSQL() container.DB {
	return a.container.SQL
}
