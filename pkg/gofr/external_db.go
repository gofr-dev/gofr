package gofr

import (
	"go.opentelemetry.io/otel"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/datasource/file"
)

// AddMongo sets the Mongo datasource in the app's container.
func (a *App) AddMongo(db container.MongoProvider) {
	db.UseLogger(a.Logger())
	db.UseMetrics(a.Metrics())

	db.Connect()

	a.container.Mongo = db
}

// AddFTP sets the FTP datasource in the app's container.
// Deprecated: Use the AddFile method instead.
func (a *App) AddFTP(fs file.FileSystemProvider) {
	fs.UseLogger(a.Logger())
	fs.UseMetrics(a.Metrics())

	fs.Connect()

	a.container.File = fs
}

// AddPubSub sets the PubSub client in the app's container.
func (a *App) AddPubSub(pubsub container.PubSubProvider) {
	pubsub.UseLogger(a.Logger())
	pubsub.UseMetrics(a.Metrics())

	pubsub.Connect()

	a.container.PubSub = pubsub
}

// AddFile sets the FTP,SFTP,S3 datasource in the app's container.
func (a *App) AddFileStore(fs file.FileSystemProvider) {
	fs.UseLogger(a.Logger())
	fs.UseMetrics(a.Metrics())

	fs.Connect()

	a.container.File = fs
}

// AddClickhouse initializes the clickhouse client.
// Official implementation is available in the package : gofr.dev/pkg/gofr/datasource/clickhouse .
func (a *App) AddClickhouse(db container.ClickhouseProvider) {
	db.UseLogger(a.Logger())
	db.UseMetrics(a.Metrics())

	tracer := otel.GetTracerProvider().Tracer("gofr-clickhouse")

	db.UseTracer(tracer)

	db.Connect()

	a.container.Clickhouse = db
}

// UseMongo sets the Mongo datasource in the app's container.
// Deprecated: Use the AddMongo method instead.
func (a *App) UseMongo(db container.Mongo) {
	a.container.Mongo = db
}

// AddCassandra sets the Cassandra datasource in the app's container.
func (a *App) AddCassandra(db container.CassandraProvider) {
	db.UseLogger(a.Logger())
	db.UseMetrics(a.Metrics())

	tracer := otel.GetTracerProvider().Tracer("gofr-cassandra")

	db.UseTracer(tracer)

	db.Connect()

	a.container.Cassandra = db
}

// AddKVStore sets the KV-Store datasource in the app's container.
func (a *App) AddKVStore(db container.KVStoreProvider) {
	db.UseLogger(a.Logger())
	db.UseMetrics(a.Metrics())

	db.Connect()

	a.container.KVStore = db
}

// AddSolr sets the Solr datasource in the app's container.
func (a *App) AddSolr(db container.SolrProvider) {
	db.UseLogger(a.Logger())
	db.UseMetrics(a.Metrics())

	db.Connect()

	a.container.Solr = db
}

// AddDgraph sets the Dgraph datasource in the app's container.
func (a *App) AddDgraph(db container.DgraphProvider) {
	// Create the Dgraph client with the provided configuration
	db.UseLogger(a.Logger())
	db.UseMetrics(a.Metrics())

	db.Connect()

	a.container.DGraph = db
}
