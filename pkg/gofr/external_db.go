package gofr

import (
	"database/sql/driver"
	"reflect"

	"go.opentelemetry.io/otel"

	"gofr.dev/pkg/gofr/ai"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/datasource/file"
	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/datasource/sql"
)

// tracerName returns the OpenTelemetry tracer name for a datasource,
// or an empty string if tracing is not applicable for the type.
func tracerName(ds any) string {
	matchers := []struct {
		match func(any) bool
		name  string
	}{
		{func(d any) bool { _, ok := d.(container.Mongo); return ok }, "gofr-mongo"},
		{func(d any) bool { _, ok := d.(container.ArangoDB); return ok }, "gofr-arangodb"},
		{func(d any) bool { _, ok := d.(container.Clickhouse); return ok }, "gofr-clickhouse"},
		{func(d any) bool { _, ok := d.(container.OracleDB); return ok }, "gofr-oracle"},
		{func(d any) bool { _, ok := d.(container.CassandraWithContext); return ok }, "gofr-cassandra"},
		{func(d any) bool { _, ok := d.(container.KVStore); return ok }, "gofr-kvstore"},
		{func(d any) bool { _, ok := d.(container.Solr); return ok }, "gofr-solr"},
		{func(d any) bool { _, ok := d.(container.Dgraph); return ok }, "gofr-dgraph"},
		{func(d any) bool { _, ok := d.(container.OpenTSDB); return ok }, "gofr-opentsdb"},
		{func(d any) bool { _, ok := d.(container.ScyllaDB); return ok }, "gofr-scylladb"},
		{func(d any) bool { _, ok := d.(container.SurrealDB); return ok }, "gofr-surrealdb"},
		{func(d any) bool { _, ok := d.(container.Elasticsearch); return ok }, "gofr-elasticsearch"},
		{func(d any) bool { _, ok := d.(container.Couchbase); return ok }, "gofr-couchbase"},
		{func(d any) bool { _, ok := d.(container.InfluxDB); return ok }, "gofr-influxdb"},
		{func(d any) bool { _, ok := d.(container.DBResolverProvider); return ok }, "gofr-dbresolver"},
	}

	for _, m := range matchers {
		if m.match(ds) {
			return m.name
		}
	}

	return ""
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

	if t, ok := ds.(interface{ UseTracer(any) }); ok {
		if name := tracerName(ds); name != "" {
			t.UseTracer(otel.GetTracerProvider().Tracer(name))
		} else {
			a.Logger().Warnf("datasource %T implements UseTracer but has no tracer name registered in tracerName(); "+
				"tracing will be skipped — add a matcher arm in pkg/gofr/external_db.go", ds)
		}
	}

	if cfg, ok := ds.(interface{ UseConfig(any) }); ok {
		cfg.UseConfig(a.Config)
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

// LLMOption configures how a model is registered. It is reserved for forward-compatible options
// such as naming a model in a multi-model setup.
type LLMOption func(*llmOptions)

type llmOptions struct{}

// AddLLM registers an LLM model on the app. The model becomes reachable in handlers via
// ctx.LLM(), its reachability is reported on the health endpoint, and its request and token
// metrics are registered on first use. A nil or typed-nil model is ignored.
func (a *App) AddLLM(m ai.Model, _ ...LLMOption) {
	if m == nil {
		return
	}

	if v := reflect.ValueOf(m); v.Kind() == reflect.Pointer && v.IsNil() {
		return
	}

	a.instrumentDatasource(m)
	a.llmMetricsOnce.Do(func() { ai.RegisterMetrics(a.Metrics()) })
	a.container.SetLLM(m)
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

// SQLConnector is implemented by pluggable SQL datasource providers (for example
// GCP Cloud SQL with IAM auth, in module gofr.dev/pkg/gofr/datasource/cloudsql).
// It constructs only a driver.Connector from configuration, keeping its cloud SDK
// out of core; AddSQLDB does the SQL wrapping so logging, metrics, health checks
// and transactions behave identically to a normal gofr SQL connection.
type SQLConnector interface {
	// Connect returns the driver.Connector to open, a cleanup to run on Close, and
	// an error. A nil connector with a nil error means the provider defers to the
	// standard environment-configured SQL connection (which container.Create has
	// already opened from the DB_* config), so AddSQLDB leaves that in place.
	Connect() (driver.Connector, func() error, error)
}

// AddSQLDB installs a pluggable SQL datasource built from c, opening it through
// gofr's standard SQL datasource (gofrSQL.NewSQLFromConnector) so ctx.SQL and all
// of gofr's SQL logging, metrics, health checks and transactions behave identically
// to an env-configured connection. Official implementations live in published
// modules such as gofr.dev/pkg/gofr/datasource/cloudsql:
//
//	app.AddSQLDB(cloudsql.New(app.Config))
//
// container.Create eagerly opens an env-configured SQL connection whenever
// DB_DIALECT is set, so the existing one is closed before being replaced —
// otherwise its pool and the background retry/metrics goroutines leak. When the
// provider defers (nil connector), that env-configured connection is kept as-is.
func (a *App) AddSQLDB(c SQLConnector) {
	connector, cleanup, err := c.Connect()
	if err != nil {
		// Connector setup failed. Tear down the env-configured SQL the container
		// opened (on the IAM path it is dialing the instance-connection-name as a
		// literal host and would retry forever) and leave SQL unset, so health
		// reports down rather than using a broken connection.
		a.closeExistingSQL()
		a.container.SQL = nil
		a.Logger().Errorf("failed to initialize SQL connector: %v", err)

		return
	}

	if connector == nil {
		// Provider defers to the standard env-configured SQL connection; keep it.
		return
	}

	a.closeExistingSQL()
	a.container.SQL = sql.NewSQLFromConnector(connector, a.Config, a.Logger(), a.Metrics(), cleanup)
}

// closeExistingSQL closes the container's current SQL datasource if one is set, so
// AddSQLDB never leaks the env-configured pool and its background goroutines when
// it replaces or discards the connection.
func (a *App) closeExistingSQL() {
	if old := a.container.SQL; !isNilDatasource(old) {
		if err := old.Close(); err != nil {
			a.Logger().Errorf("failed to close the existing SQL datasource: %v", err)
		}
	}
}

// isNilDatasource reports whether a container datasource interface is nil or holds
// a typed-nil pointer (as container.Create can leave SQL when the dialect is unset),
// so AddSQLDB never calls Close on a nil value.
func isNilDatasource(i any) bool {
	if i == nil {
		return true
	}

	val := reflect.ValueOf(i)

	return val.Kind() == reflect.Pointer && val.IsNil()
}

// GetSQL returns the SQL datasource from the container.
func (a *App) GetSQL() container.DB {
	return a.container.SQL
}
